package migration

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func fixture(t *testing.T) (Config, *sql.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("BBS_NATIVE_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_NATIVE_TEST_DSN to an expendable MySQL server; test creates and drops unique databases")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid test MySQL DSN")
	}
	cfg.ParseTime = false
	cfg.MultiStatements = true // Only this trusted SQL fixture; importer forcibly disables it.
	cfg.Params = map[string]string{"charset": "utf8mb4", "sql_mode": "'STRICT_ALL_TABLES,NO_AUTO_VALUE_ON_ZERO'", "time_zone": "'+00:00'"}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("cannot open test MySQL")
	}
	t.Cleanup(func() { admin.Close() })
	prefix := fmt.Sprintf("bbs_native_test_%d", time.Now().UnixNano())
	sourceName, targetName := prefix+"_src", prefix+"_dst"
	for _, name := range []string{sourceName, targetName} {
		if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
			t.Fatal("cannot create test database")
		}
		name := name
		t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS `" + name + "`") })
	}
	cfg.DBName = sourceName
	sourceDSN := cfg.FormatDSN()
	source, err := sql.Open("mysql", sourceDSN)
	if err != nil {
		t.Fatal("source connection failed")
	}
	t.Cleanup(func() { source.Close() })
	cfg.DBName = targetName
	targetDSN := cfg.FormatDSN()
	target, err := sql.Open("mysql", targetDSN)
	if err != nil {
		t.Fatal("target connection failed")
	}
	t.Cleanup(func() { target.Close() })
	data, err := os.ReadFile("testdata/flarum.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(string(data)); err != nil {
		t.Fatal("test fixture failed: ", err)
	}
	return Config{SourceDSN: sourceDSN, TargetDSN: targetDSN, ReadOnlyTarget: true, Initialize: true}, source, target
}

func scalar(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	var result string
	if err := db.QueryRow(query).Scan(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func execTest(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	if _, err := db.Exec(statement); err != nil {
		t.Fatal(err)
	}
}

func TestNativeSnapshotRefreshAndRollback(t *testing.T) {
	cfg, source, target := fixture(t)
	ctx := context.Background()
	result, err := Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.NativeRows["posts"] != 3 || result.NativeRows["discussion_subscriptions"] != 1 {
		t.Fatalf("unexpected converted counts: %+v", result.NativeRows)
	}
	if got := scalar(t, target, "SELECT body_markdown FROM posts WHERE id=1"); got != "**中文 & Markdown**" {
		t.Fatalf("unexpected Markdown %q", got)
	}
	if got := scalar(t, target, "SELECT password_hash FROM users WHERE id=1"); got != "$2y$10$unchanged" {
		t.Fatal("password hash changed")
	}
	if got := scalar(t, target, "SELECT DATE_FORMAT(joined_at,'%Y-%m-%d %H:%i:%s.%f') FROM users WHERE id=1"); got != "2026-01-02 03:04:05.123456" {
		t.Fatal("date precision changed")
	}
	if got := scalar(t, target, "SELECT CONCAT(scope,':',scope_id,':',action) FROM permissions WHERE scope='tag'"); got != "tag:10:discussion.reply" {
		t.Fatal("tag scope changed")
	}
	var columns []string
	var values []*string
	if err := json.Unmarshal([]byte(scalar(t, target, "SELECT columns_json FROM legacy_tables WHERE name='unknown_extension'")), &columns); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(scalar(t, target, "SELECT values_json FROM legacy_rows WHERE source_table='unknown_extension'")), &values); err != nil {
		t.Fatal(err)
	}
	archived := make(map[string]*string)
	for i, column := range columns {
		archived[column] = values[i]
	}
	for column, want := range map[string]string{"payload": "\x00\xff\\'\n", "empty": "", "exact_number": "12345678901234567890.1234567890", "at": "2026-01-02 03:04:05.123456", "zero_at": "0000-00-00 00:00:00"} {
		if archived[column] == nil {
			t.Fatalf("archive %s unexpectedly NULL", column)
		}
		got, err := base64.StdEncoding.DecodeString(*archived[column])
		if err != nil || string(got) != want {
			t.Fatalf("archive %s changed", column)
		}
	}
	if archived["nullable"] != nil {
		t.Fatal("archive lost SQL NULL")
	}
	execTest(t, target, "INSERT INTO sessions VALUES(UNHEX(SHA2('local-session',256)),1,'2030-01-01');")
	execTest(t, target, "INSERT INTO user_profiles VALUES(1,'姓名','https://example.test','2026-01-01');")
	cfg.Initialize = false
	execTest(t, source, "UPDATE flarum_posts SET content='<t>edited</t>' WHERE id=1")
	execTest(t, source, "DELETE FROM flarum_posts WHERE id=2")
	execTest(t, source, "INSERT INTO flarum_posts SELECT 4,discussion_id,4,user_id,type,'<t>new</t>',created_at,edited_at,edited_user_id,hidden_at,hidden_user_id,is_private,is_approved FROM flarum_posts WHERE id=1")
	if _, err := Run(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, target, "SELECT GROUP_CONCAT(id ORDER BY id) FROM posts"); got != "1,3,4" {
		t.Fatal("add/delete snapshot mismatch")
	}
	if got := scalar(t, target, "SELECT body_markdown FROM posts WHERE id=1"); got != "edited" {
		t.Fatal("edit not imported")
	}
	if scalar(t, target, "SELECT COUNT(*) FROM sessions") != "1" || scalar(t, target, "SELECT real_name FROM user_profiles") != "姓名" {
		t.Fatal("local data lost")
	}
	before := scalar(t, target, "SELECT counts_json FROM migration_state")
	execTest(t, source, "UPDATE flarum_posts SET discussion_id=999,content='<t>must rollback</t>' WHERE id=1")
	if _, err := Run(ctx, cfg); err == nil {
		t.Fatal("dangling relationship accepted")
	}
	if scalar(t, target, "SELECT body_markdown FROM posts WHERE id=1") != "edited" || scalar(t, target, "SELECT counts_json FROM migration_state") != before {
		t.Fatal("failed refresh changed previous snapshot")
	}
	execTest(t, source, "UPDATE flarum_posts SET discussion_id=1 WHERE id=1")
	// A target constraint error during insertion must also roll back all tables.
	execTest(t, source, "INSERT INTO flarum_users (id,username,email) VALUES (2,'Alice','duplicate@example.test')")
	if _, err := Run(ctx, cfg); err == nil {
		t.Fatal("duplicate target username accepted")
	}
	if scalar(t, target, "SELECT body_markdown FROM posts WHERE id=1") != "edited" || scalar(t, target, "SELECT counts_json FROM migration_state") != before {
		t.Fatal("insert failure changed previous snapshot")
	}
	execTest(t, source, "DELETE FROM flarum_users WHERE id=2")
	if _, err := Run(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if scalar(t, target, "SELECT body_markdown FROM posts WHERE id=1") != "must rollback" {
		t.Fatal("retry did not replace snapshot")
	}
	cfg.Initialize = true
	if _, err := Run(ctx, cfg); err == nil {
		t.Fatal("reinitialization accepted nonempty target")
	}
}

func TestNativeTargetOwnershipAndDriftGuards(t *testing.T) {
	cfg, source, target := fixture(t)
	ctx := context.Background()
	same := cfg
	same.TargetDSN = cfg.SourceDSN + "&parseTime=true"
	if _, err := Run(ctx, same); err == nil || !strings.Contains(err.Error(), "same database") {
		t.Fatal("same database accepted despite changed DSN options")
	}
	if _, err := Run(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Initialize = false
	for _, test := range []struct {
		db           *sql.DB
		change, undo string
	}{
		{target, "CREATE TABLE unknown_table(id INT) ENGINE=InnoDB", "DROP TABLE unknown_table"},
		{target, "ALTER TABLE posts ADD unexpected INT", "ALTER TABLE posts DROP unexpected"},
		{source, "ALTER TABLE flarum_posts ADD unexpected INT", "ALTER TABLE flarum_posts DROP unexpected"},
	} {
		execTest(t, test.db, test.change)
		if _, err := Run(ctx, cfg); err == nil {
			t.Fatal("table/schema drift accepted")
		}
		execTest(t, test.db, test.undo)
	}
	conn, err := target.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT('bbs.native.',LEFT(SHA2(LOWER(DATABASE()),256),40)),0)").Scan(&locked); err != nil || locked != 1 {
		t.Fatal("test advisory lock failed")
	}
	if _, err := Run(ctx, cfg); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatal("simultaneous import was not blocked")
	}
	_, _ = conn.ExecContext(ctx, "SELECT RELEASE_ALL_LOCKS()")
	conn.Close()
	if _, err := Run(ctx, cfg); err != nil {
		t.Fatal(err)
	}
}
