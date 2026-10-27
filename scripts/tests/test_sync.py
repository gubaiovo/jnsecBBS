import importlib.util
import os
from pathlib import Path
import subprocess
import shlex
import tempfile
import time
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'sync-flarum.py'
spec = importlib.util.spec_from_file_location('sync_flarum', SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SafetyTests(unittest.TestCase):
    def test_rejects_same_or_system_database(self):
        for source, target in [('flarum', 'flarum'), ('flarum', 'FLARUM'), ('mysql', 'bbs'), ('flarum', 'mysql')]:
            with self.assertRaises(ValueError):
                module.Mirror('unused', source, target)

    def test_rejects_sql_identifiers(self):
        for value in ['foo;DROP DATABASE bar', '`flarum`', 'a.b', '']:
            with self.assertRaises(ValueError):
                module.identifier(value)


@unittest.skipUnless(os.environ.get('BBS_SYNC_TEST_MYSQL'), 'set BBS_SYNC_TEST_MYSQL to an expendable Docker MySQL container')
class MySQLIntegration(unittest.TestCase):
    def test_snapshot_and_failure_rollback(self):
        suffix = str(time.time_ns())
        source, target = 'bbs_sync_test_src_' + suffix, 'bbs_sync_test_dst_' + suffix
        mirror = module.Mirror(os.environ['BBS_SYNC_TEST_MYSQL'], source, target)
        with tempfile.TemporaryDirectory() as directory:
            args = ['python3', str(SCRIPT), '--container', mirror.container, '--source', source,
                    '--target', target, '--state-dir', directory, '--read-only-target']
            try:
                mirror.mysql('CREATE DATABASE `%s` CHARACTER SET utf8mb4;' % source)
                mirror.mysql("CREATE TABLE flarum_posts (id INT PRIMARY KEY,content TEXT) ENGINE=InnoDB;"
                             "CREATE TABLE flarum_access_tokens (id INT PRIMARY KEY,token VARCHAR(100)) ENGINE=InnoDB;"
                             "CREATE TABLE bbs_notifications (id INT PRIMARY KEY,content TEXT) ENGINE=InnoDB;"
                             "INSERT INTO bbs_notifications VALUES (1,'legacy notification');"
                             "INSERT INTO flarum_posts VALUES (1,'中文 & <tag>'),(2,'deleted');", source)
                subprocess.run(args + ['--initialize'], check=True)
                # Optional Go tables must survive refreshes without changing the
                # source schema fingerprint stored at initialization.
                mirror.mysql("CREATE TABLE bbs_user_profiles (user_id INT PRIMARY KEY,real_name VARCHAR(100)) ENGINE=InnoDB;"
                             "INSERT INTO bbs_user_profiles VALUES (1,'姓名');"
                             "CREATE TABLE bbs_schema_migrations (version VARCHAR(100) PRIMARY KEY) ENGINE=InnoDB;"
                             "INSERT INTO bbs_schema_migrations VALUES ('initial');"
                             "CREATE TABLE bbs_invite_codes (id INT PRIMARY KEY,code VARCHAR(100)) ENGINE=InnoDB;"
                             "INSERT INTO bbs_invite_codes VALUES (1,'local invite');", target)
                # Docker's interactive stdin must not consume a caller's SSH
                # shell script and silently discard commands after the sync.
                shell = shlex.join(args) + "\nprintf 'SYNC_STDIN_PRESERVED\\n'\n"
                result = subprocess.run(['bash'], input=shell.encode(), capture_output=True, check=True)
                self.assertIn(b'SYNC_STDIN_PRESERVED', result.stdout)
                mirror.mysql("INSERT INTO flarum_access_tokens VALUES(1,'session');", target)
                mirror.mysql("UPDATE flarum_posts SET content='edited' WHERE id=1; DELETE FROM flarum_posts WHERE id=2; INSERT INTO flarum_posts VALUES(3,'new');", source)
                mirror.mysql("UPDATE bbs_notifications SET content='updated legacy notification' WHERE id=1;", source)
                subprocess.run(args, check=True)
                query = 'SELECT id,content FROM flarum_posts ORDER BY id;'
                self.assertEqual(mirror.mysql(query, source), mirror.mysql(query, target))
                self.assertEqual(mirror.mysql('SELECT token FROM flarum_access_tokens;', target).strip(), 'session')
                self.assertEqual(mirror.mysql('SELECT real_name FROM bbs_user_profiles;', target).strip(), '姓名')
                self.assertEqual(mirror.mysql('SELECT version FROM bbs_schema_migrations;', target).strip(), 'initial')
                self.assertEqual(mirror.mysql('SELECT code FROM bbs_invite_codes;', target).strip(), 'local invite')
                self.assertEqual(mirror.mysql('SELECT content FROM bbs_notifications;', source), mirror.mysql('SELECT content FROM bbs_notifications;', target))
                before = mirror.mysql(query, target)
                mirror.mysql("UPDATE flarum_posts SET content='must rollback' WHERE id=1;", source)
                # Unknown tables and drift in mirrored columns must stop the
                # job before any target row is replaced.
                for statement, undo in [
                    ('CREATE TABLE unknown_table (id INT) ENGINE=InnoDB;', 'DROP TABLE unknown_table;'),
                    ('ALTER TABLE flarum_posts ADD extra INT;', 'ALTER TABLE flarum_posts DROP extra;'),
                ]:
                    mirror.mysql(statement, target)
                    self.assertNotEqual(subprocess.run(args, capture_output=True).returncode, 0)
                    self.assertEqual(before, mirror.mysql(query, target))
                    mirror.mysql(undo, target)
                mirror.mysql('ALTER TABLE flarum_posts ADD extra INT;', source)
                self.assertNotEqual(subprocess.run(args, capture_output=True).returncode, 0)
                self.assertEqual(before, mirror.mysql(query, target))
                mirror.mysql('ALTER TABLE flarum_posts DROP extra;', source)
                mirror.mysql("CREATE TRIGGER reject_test BEFORE INSERT ON flarum_posts FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='intentional test failure';", target)
                self.assertNotEqual(subprocess.run(args, capture_output=True).returncode, 0)
                self.assertEqual(before, mirror.mysql(query, target))
                mirror.mysql('DROP TRIGGER reject_test;', target)
                subprocess.run(args, check=True)
                self.assertEqual(mirror.mysql(query, source), mirror.mysql(query, target))
                self.assertNotEqual(subprocess.run(args + ['--initialize'], capture_output=True).returncode, 0)
            finally:
                mirror.mysql('DROP DATABASE IF EXISTS `%s`; DROP DATABASE IF EXISTS `%s`;' % (source, target))


if __name__ == '__main__':
    unittest.main()
