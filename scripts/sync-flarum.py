#!/usr/bin/env python3
"""Mirror a legacy database into a READ-ONLY Go forum on the same MySQL server.

Uses the mysql:5.7 container's clients; no Python packages or credentials in
arguments/files. MYSQL_ROOT_PASSWORD is read inside the container. Only the
source is dumped; all import commands select the distinct target database.
"""
import argparse
import collections
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time

LOCAL_TABLES = {"flarum_access_tokens"}  # Go browser sessions survive refreshes.
STATE_TABLE = "_bbs_sync_state"
IDENTIFIER = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")
INSERT_TABLE = re.compile(rb"^INSERT INTO `([A-Za-z_][A-Za-z0-9_]*)` ")


def identifier(value):
    if not IDENTIFIER.fullmatch(value):
        raise ValueError("invalid database/table identifier")
    return "`" + value + "`"


def client_command(container, program, args):
    # No interpolation into shell code: database/table names are positional args.
    return ["docker", "exec", "-i", container, "sh", "-c",
            'export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"; exec "$@"', "sync", program,
            "--user=root", "--default-character-set=utf8mb4", *args]


class Mirror:
    def __init__(self, container, source, target):
        identifier(source); identifier(target)
        if source.lower() == target.lower():
            raise ValueError("source and target databases must differ")
        if source.lower() in {"mysql", "sys", "information_schema", "performance_schema"} or target.lower() in {"mysql", "sys", "information_schema", "performance_schema"}:
            raise ValueError("system databases cannot be mirrored")
        self.container, self.source, self.target = container, source, target

    def mysql(self, query, database=None):
        args = ["--batch", "--raw", "--skip-column-names"]
        if database:
            args.append(database)
        result = subprocess.run(client_command(self.container, "mysql", args),
                                input=query.encode(), capture_output=True, timeout=240)
        if result.returncode:
            # mysql diagnostics can include private row text; don't put it in logs.
            raise RuntimeError("MySQL command failed (exit %d); no SQL/row values logged" % result.returncode)
        return result.stdout.decode()

    def tables(self, database):
        rows = self.mysql("SELECT TABLE_NAME,ENGINE,TABLE_TYPE FROM information_schema.TABLES "
                          "WHERE TABLE_SCHEMA='%s' ORDER BY TABLE_NAME;" % database)
        tables = []
        for row in rows.splitlines():
            name, engine, kind = row.split("\t")
            identifier(name)
            if kind != "BASE TABLE" or engine != "InnoDB":
                raise RuntimeError("consistent mirror requires InnoDB base tables: " + name)
            tables.append(name)
        return tables

    def schema(self, database):
        # Excludes changing AUTO_INCREMENT counters, includes column/default/index shape.
        return self.mysql("SELECT TABLE_NAME,ORDINAL_POSITION,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,"
                          "COALESCE(COLUMN_DEFAULT,'<NULL>'),EXTRA,COALESCE(COLLATION_NAME,''),"
                          "COALESCE(GENERATION_EXPRESSION,'') FROM information_schema.COLUMNS "
                          "WHERE TABLE_SCHEMA='%s' AND TABLE_NAME<>'%s' ORDER BY TABLE_NAME,ORDINAL_POSITION;"
                          "SELECT TABLE_NAME,INDEX_NAME,NON_UNIQUE,SEQ_IN_INDEX,COLUMN_NAME,COALESCE(SUB_PART,0) "
                          "FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='%s' AND TABLE_NAME<>'%s' "
                          "ORDER BY TABLE_NAME,INDEX_NAME,SEQ_IN_INDEX;" % (database, STATE_TABLE, database, STATE_TABLE))

    def dump(self, tables, path, schema=False):
        args = ["--single-transaction", "--quick", "--hex-blob", "--set-gtid-purged=OFF",
                "--skip-triggers", "--skip-add-locks", "--skip-disable-keys",
                "--skip-add-drop-table", "--skip-comments", "--complete-insert",
                "--skip-extended-insert", "--no-data" if schema else "--no-create-info",
                self.source, *tables]
        with path.open("wb") as out:
            result = subprocess.run(client_command(self.container, "mysqldump", args),
                                    stdout=out, stderr=subprocess.PIPE, timeout=240)
        if result.returncode:
            raise RuntimeError("source snapshot failed; target data unchanged")

    def initialize(self, tables, temporary):
        existing = self.mysql("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='%s';" % self.target).strip()
        if existing != "0":
            raise RuntimeError("initialization requires an empty target; refusing existing data")
        self.mysql("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" % identifier(self.target))
        schema_file = temporary / "schema.sql"
        self.dump(tables, schema_file, schema=True)
        # mysqldump is not invoked with --databases: no CREATE DATABASE or USE.
        self.mysql(schema_file.read_text(), self.target)
        self.mysql("CREATE TABLE %s (id INT PRIMARY KEY,source_db VARCHAR(64) NOT NULL,"
                   "schema_hash CHAR(64) NOT NULL,completed_at DATETIME NULL,row_counts LONGTEXT NULL) ENGINE=InnoDB;"
                   "INSERT INTO %s (id,source_db,schema_hash) VALUES (1,'%s','%s');" %
                   (identifier(STATE_TABLE), identifier(STATE_TABLE), self.source,
                    hashlib.sha256(self.schema(self.source).encode()).hexdigest()), self.target)

    def run(self, initialize, temporary):
        tables = self.tables(self.source)
        if not tables or "flarum_posts" not in tables or STATE_TABLE in tables:
            raise RuntimeError("source is not a legacy Flarum database")
        if initialize:
            self.initialize(tables, temporary)
        target_tables = self.tables(self.target)
        if set(target_tables) != set(tables) | {STATE_TABLE}:
            raise RuntimeError("table set changed; review schema before resuming synchronization")
        source_schema = self.schema(self.source)
        if source_schema != self.schema(self.target):
            raise RuntimeError("source/target schema differs; review schema before synchronization")
        schema_hash = hashlib.sha256(source_schema.encode()).hexdigest()
        marker = self.mysql("SELECT source_db,schema_hash FROM %s WHERE id=1;" % identifier(STATE_TABLE), self.target).strip()
        if marker != self.source + "\t" + schema_hash:
            raise RuntimeError("source/schema does not match initialized target")
        copied = [table for table in tables if table not in LOCAL_TABLES]
        snapshot = temporary / "snapshot.sql"
        self.dump(copied, snapshot)
        if self.schema(self.source) != source_schema:
            raise RuntimeError("source schema changed during snapshot; target unchanged")
        counts = collections.Counter({table: 0 for table in copied})
        insert_file = temporary / "inserts.sql"
        with snapshot.open("rb") as data, insert_file.open("wb") as inserts:
            for line in data:
                match = INSERT_TABLE.match(line)
                if match:
                    table = match[1].decode()
                    if table not in counts:
                        raise RuntimeError("unexpected table in source snapshot")
                    counts[table] += 1
                    inserts.write(line)
                elif line.strip() and not line.startswith((b"/*!", b"--")):
                    raise RuntimeError("unexpected statement in data snapshot; target unchanged")
        # Single target transaction: no TRUNCATE, LOCK TABLES, DDL or --force.
        # mysql batch mode aborts on errors; closing the connection rolls back.
        with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
            process = subprocess.Popen(client_command(self.container, "mysql", ["--binary-mode", self.target]),
                                       stdin=subprocess.PIPE, stdout=output, stderr=errors)
            try:
                process.stdin.write(b"SET NAMES utf8mb4; SET time_zone='+00:00'; SET SESSION sql_mode='NO_AUTO_VALUE_ON_ZERO'; SET FOREIGN_KEY_CHECKS=0; START TRANSACTION;\n")
                for table in copied:
                    process.stdin.write(("DELETE FROM %s;\n" % identifier(table)).encode())
                with insert_file.open("rb") as inserts:
                    while True:
                        block = inserts.read(1 << 20)
                        if not block:
                            break
                        process.stdin.write(block)
                summary = json.dumps(dict(counts), sort_keys=True, separators=(",", ":"))
                process.stdin.write(("UPDATE %s SET completed_at=UTC_TIMESTAMP(),row_counts='%s' WHERE id=1; COMMIT;\n" %
                                     (identifier(STATE_TABLE), summary)).encode())
                process.stdin.close()
                if process.wait(timeout=240):
                    raise RuntimeError("target import failed; transaction rolled back")
            except BaseException:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                raise
        # App is read-only and sessions are excluded, so these counts are stable.
        rows = self.mysql(" UNION ALL ".join("SELECT '%s',COUNT(*) FROM %s" % (table, identifier(table)) for table in copied) + ";", self.target)
        actual = {table: int(count) for table, count in (line.split("\t") for line in rows.splitlines())}
        if actual != dict(counts):
            raise RuntimeError("target counts differ from snapshot; investigate unexpected target writes")
        return {"tables": len(copied), "rows": sum(counts.values()), "source": self.source, "target": self.target}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--container", default=os.environ.get("MYSQL_CONTAINER", "bbs-mysql"))
    parser.add_argument("--source", default=os.environ.get("SOURCE_DB", "flarum"))
    parser.add_argument("--target", default=os.environ.get("TARGET_DB", "bbs"))
    parser.add_argument("--state-dir", default="/var/lib/jnsec-bbs-sync")
    parser.add_argument("--initialize", action="store_true", help="create schema in an EMPTY target before initial copy")
    parser.add_argument("--read-only-target", action="store_true", help="required: acknowledge target forum writes are disabled")
    args = parser.parse_args()
    if not args.read_only_target:
        parser.error("requires --read-only-target; mirroring overwrites target forum data")
    mirror = Mirror(args.container, args.source, args.target)
    state = Path(args.state_dir)
    state.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.umask(0o077)
    with (state / "sync.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("sync already running; skipped")
            return
        start = time.monotonic()
        with tempfile.TemporaryDirectory(prefix="snapshot-", dir=state) as temporary:
            result = mirror.run(args.initialize, Path(temporary))
        result["elapsed_seconds"] = round(time.monotonic() - start, 2)
        print(json.dumps(result, sort_keys=True), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print("sync failed: " + str(exc), file=sys.stderr)
        sys.exit(1)
