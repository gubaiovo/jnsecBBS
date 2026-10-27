package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

type Config struct {
	SourceDSN, TargetDSN       string
	Initialize, ReadOnlyTarget bool
}

type Result struct {
	SchemaVersion int              `json:"schema_version"`
	SourceRows    map[string]int64 `json:"source_rows"`
	NativeRows    map[string]int64 `json:"native_rows"`
	CompletedAt   time.Time        `json:"completed_at"`
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type table struct{ name, create string }

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var autoIncrementPattern = regexp.MustCompile(` AUTO_INCREMENT=\d+`)

func quoted(name string) (string, error) {
	if !namePattern.MatchString(name) {
		return "", errors.New("unsupported database/table/column identifier")
	}
	return "`" + name + "`", nil
}

// Only operation names reach callers. Database diagnostics can contain private
// row values, SQL, and credentials and must never be returned to CLI logs.
func failed(operation string) error {
	return errors.New("native import: " + operation + " failed; target snapshot was not committed")
}

func open(dsn string) (*sql.DB, error) {
	c, err := mysql.ParseDSN(dsn)
	if err != nil || c.DBName == "" {
		return nil, errors.New("native import requires valid MySQL DSNs with explicit databases")
	}
	if _, err := quoted(c.DBName); err != nil {
		return nil, err
	}
	switch strings.ToLower(c.DBName) {
	case "mysql", "sys", "information_schema", "performance_schema":
		return nil, errors.New("native import refuses system databases")
	}
	c.ParseTime, c.MultiStatements, c.AllowAllFiles = false, false, false
	c.Collation, c.Loc, c.Timeout, c.ReadTimeout, c.WriteTimeout = "utf8mb4_unicode_ci", time.UTC, 10*time.Second, 4*time.Minute, 30*time.Second
	if c.Params == nil {
		c.Params = make(map[string]string)
	}
	c.Params["charset"], c.Params["time_zone"] = "utf8mb4", "'+00:00'"
	// Refuse lossy truncation/coercion while preserving legacy zero dates and IDs.
	c.Params["sql_mode"] = "'STRICT_ALL_TABLES,NO_AUTO_VALUE_ON_ZERO'"
	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return nil, failed("database configuration")
	}
	db.SetMaxOpenConns(2)
	return db, nil
}

func identity(ctx context.Context, q queryer) (server, database string, err error) {
	err = q.QueryRowContext(ctx, "SELECT @@server_uuid,DATABASE()").Scan(&server, &database)
	if err != nil {
		err = failed("database identity check")
	}
	return
}

func catalog(ctx context.Context, q queryer, lock bool) ([]table, error) {
	rows, err := q.QueryContext(ctx, "SELECT TABLE_NAME,ENGINE,TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME")
	if err != nil {
		return nil, failed("table catalog read")
	}
	var names []string
	for rows.Next() {
		var name, engine, kind string
		if err := rows.Scan(&name, &engine, &kind); err != nil {
			rows.Close()
			return nil, failed("table catalog read")
		}
		if _, err := quoted(name); err != nil {
			rows.Close()
			return nil, err
		}
		if kind != "BASE TABLE" || engine != "InnoDB" {
			rows.Close()
			return nil, errors.New("native import requires InnoDB base tables only")
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, failed("table catalog read")
	}
	rows.Close()
	var triggers int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE()").Scan(&triggers); err != nil {
		return nil, failed("trigger check")
	}
	if triggers != 0 {
		return nil, errors.New("native import refuses database triggers; review them before import")
	}
	var result []table
	for _, name := range names {
		identifier, _ := quoted(name)
		if lock {
			// Metadata locks last until the read transaction ends. No source DDL
			// or writes are issued; a real read establishes the repeatable view.
			r, err := q.QueryContext(ctx, "SELECT * FROM "+identifier+" LIMIT 1")
			if err != nil {
				return nil, failed("source snapshot read")
			}
			for r.Next() {
			} // discard without exposing source values
			err = r.Err()
			r.Close()
			if err != nil {
				return nil, failed("source snapshot read")
			}
		}
		var ignored, definition string
		if err := q.QueryRowContext(ctx, "SHOW CREATE TABLE "+identifier).Scan(&ignored, &definition); err != nil {
			return nil, failed("table definition read")
		}
		result = append(result, table{name, definition})
	}
	return result, nil
}

func fingerprint(tables []table) string {
	h := sha256.New()
	for _, t := range tables {
		fmt.Fprintln(h, t.name)
		fmt.Fprintln(h, autoIncrementPattern.ReplaceAllString(t.create, ""))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sameTableSet(tables []table, expected []string) bool {
	if len(tables) != len(expected) {
		return false
	}
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.name)
	}
	sort.Strings(names)
	wanted := append([]string(nil), expected...)
	sort.Strings(wanted)
	for i := range names {
		if names[i] != wanted[i] {
			return false
		}
	}
	return true
}

// Run imports every source table into a reversible archive and converts the
// supported core tables. Every refresh replaces only importer-managed rows in
// one transaction, preserving local sessions/profiles and the previous snapshot
// on any validation, conversion, connection, or commit failure.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if !cfg.ReadOnlyTarget {
		return nil, errors.New("native import requires -read-only-target; target forum writers must be disabled")
	}
	source, err := open(cfg.SourceDSN)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	target, err := open(cfg.TargetDSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	sourceUUID, sourceName, err := identity(ctx, source)
	if err != nil {
		return nil, err
	}
	targetUUID, targetName, err := identity(ctx, target)
	if err != nil {
		return nil, err
	}
	if sourceUUID == targetUUID && strings.EqualFold(sourceName, targetName) {
		return nil, errors.New("native import source and target identify the same database")
	}
	// Advisory locks are held by one pinned target connection, including across
	// the import transaction. Distinct credentials still share the same lock.
	connection, err := target.Conn(ctx)
	if err != nil {
		return nil, failed("target connection")
	}
	defer connection.Close()
	lockHash := sha256.Sum256([]byte(strings.ToLower(targetName)))
	lockName := "bbs.native." + hex.EncodeToString(lockHash[:20])
	var locked int
	if err := connection.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", lockName).Scan(&locked); err != nil || locked != 1 {
		return nil, errors.New("native import target is locked by another importer")
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = connection.ExecContext(unlockCtx, "SELECT RELEASE_LOCK(?)", lockName)
	}()
	if _, err := connection.ExecContext(ctx, "SET SESSION sql_mode='STRICT_ALL_TABLES,NO_AUTO_VALUE_ON_ZERO'"); err != nil {
		return nil, failed("target strict SQL mode")
	}
	read, err := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, failed("source read-only transaction")
	}
	defer read.Rollback()
	sourceTables, err := catalog(ctx, read, true)
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(sourceTables))
	for _, t := range sourceTables {
		present[t.name] = true
	}
	for _, required := range []string{"flarum_users", "flarum_groups", "flarum_group_user", "flarum_group_permission", "flarum_tags", "flarum_discussion_tag", "flarum_discussions", "flarum_posts", "flarum_post_likes"} {
		if !present[required] {
			return nil, errors.New("native import source is missing a required Flarum core table")
		}
	}
	sourceHash := fingerprint(sourceTables)
	targetTables, err := catalog(ctx, connection, false)
	if err != nil {
		return nil, err
	}
	if cfg.Initialize {
		if len(targetTables) != 0 {
			return nil, errors.New("native initialization requires an empty target")
		}
		for _, statement := range schemaStatements {
			if _, err := connection.ExecContext(ctx, statement+" ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"); err != nil {
				return nil, failed("native schema initialization; initialization may require empty-target cleanup")
			}
		}
		targetTables, err = catalog(ctx, connection, false)
		if err != nil {
			return nil, err
		}
		_, err = connection.ExecContext(ctx, "INSERT INTO migration_state (id,schema_version,source_uuid,source_database,source_schema_hash,target_schema_hash) VALUES (1,?,?,?,?,?)", schemaVersion, sourceUUID, sourceName, sourceHash, fingerprint(targetTables))
		if err != nil {
			return nil, failed("native ownership marker initialization")
		}
	}
	if !sameTableSet(targetTables, append(append([]string(nil), managedTables...), localTables...)) {
		return nil, errors.New("native target table set differs; only initialized native tables are allowed")
	}
	var version int
	var ownerUUID, ownerName, oldSourceHash, targetHash string
	err = connection.QueryRowContext(ctx, "SELECT schema_version,source_uuid,source_database,source_schema_hash,target_schema_hash FROM migration_state WHERE id=1").Scan(&version, &ownerUUID, &ownerName, &oldSourceHash, &targetHash)
	if err != nil || version != schemaVersion || ownerUUID != sourceUUID || ownerName != sourceName || oldSourceHash != sourceHash || targetHash != fingerprint(targetTables) {
		return nil, errors.New("native target ownership or source/target schema changed; review before refreshing")
	}
	write, err := connection.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, failed("target transaction")
	}
	defer write.Rollback()
	// Do not disable foreign-key checks or truncate: future constraints and
	// import errors must abort the whole refresh.
	for _, name := range managedTables {
		identifier, _ := quoted(name)
		if _, err := write.ExecContext(ctx, "DELETE FROM "+identifier); err != nil {
			return nil, failed("target snapshot replacement")
		}
	}
	result := &Result{SchemaVersion: schemaVersion, SourceRows: make(map[string]int64), NativeRows: make(map[string]int64)}
	for _, t := range sourceTables {
		count, err := copyTable(ctx, read, write, t)
		if err != nil {
			return nil, err
		}
		result.SourceRows[t.name] = count
	}
	latest, err := catalog(ctx, read, false)
	if err != nil {
		return nil, err
	}
	if fingerprint(latest) != sourceHash {
		return nil, errors.New("native source schema changed during snapshot; target unchanged")
	}
	if err := validate(ctx, write); err != nil {
		return nil, err
	}
	currentTarget, err := catalog(ctx, write, false)
	if err != nil {
		return nil, err
	}
	if fingerprint(currentTarget) != targetHash {
		return nil, errors.New("native target schema changed during refresh; target unchanged")
	}
	for _, name := range managedTables {
		identifier, _ := quoted(name)
		var count int64
		if err := write.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+identifier).Scan(&count); err != nil {
			return nil, failed("native row count")
		}
		result.NativeRows[name] = count
	}
	// Verify the archive and conversion counts before commit, against this same
	// source snapshot rather than a later source COUNT that can race old-site writes.
	if err := validateCounts(result); err != nil {
		return nil, err
	}
	result.CompletedAt = time.Now().UTC()
	counts, _ := json.Marshal(result)
	if _, err := write.ExecContext(ctx, "UPDATE migration_state SET completed_at=?,counts_json=? WHERE id=1", result.CompletedAt, counts); err != nil {
		return nil, failed("success marker update")
	}
	if err := read.Commit(); err != nil {
		return nil, failed("source snapshot completion")
	}
	if err := write.Commit(); err != nil {
		return nil, errors.New("native target commit result is unknown; inspect the success marker before retrying")
	}
	return result, nil
}

func copyTable(ctx context.Context, read, write *sql.Tx, t table) (int64, error) {
	identifier, _ := quoted(t.name)
	rows, err := read.QueryContext(ctx, "SELECT * FROM "+identifier)
	if err != nil {
		return 0, failed("source table read")
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, failed("source column read")
	}
	if err := requiredColumns(t.name, columns); err != nil {
		return 0, err
	}
	columnsJSON, _ := json.Marshal(columns)
	var count int64
	for rows.Next() {
		values := make([]sql.RawBytes, len(columns))
		scan := make([]any, len(columns))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return 0, failed("source row read")
		}
		// Array order is defined by columns_json. Each non-null SQL value is
		// base64 of its exact wire bytes: binary/text/NULL/empty/zero dates are
		// unambiguous and recoverable using the archived CREATE TABLE.
		archive := make([]*string, len(columns))
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			if values[i] == nil {
				row[column] = nil
				continue
			}
			encoded := base64.StdEncoding.EncodeToString(values[i])
			archive[i] = &encoded
			row[column] = string(values[i])
		}
		encoded, _ := json.Marshal(archive)
		count++
		if _, err := write.ExecContext(ctx, "INSERT INTO legacy_rows (source_table,row_ordinal,values_json) VALUES (?,?,?)", t.name, count, encoded); err != nil {
			return 0, failed("source archive write")
		}
		if err := convert(ctx, write, t.name, row); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, failed("source row stream")
	}
	if _, err := write.ExecContext(ctx, "INSERT INTO legacy_tables (name,create_sql,columns_json,row_count) VALUES (?,?,?,?)", t.name, t.create, columnsJSON, count); err != nil {
		return 0, failed("source archive metadata write")
	}
	return count, nil
}
