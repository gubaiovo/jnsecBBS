import importlib.util
import os
from pathlib import Path
import subprocess
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
                             "INSERT INTO flarum_posts VALUES (1,'中文 & <tag>'),(2,'deleted');", source)
                subprocess.run(args + ['--initialize'], check=True)
                mirror.mysql("INSERT INTO flarum_access_tokens VALUES(1,'session');", target)
                mirror.mysql("UPDATE flarum_posts SET content='edited' WHERE id=1; DELETE FROM flarum_posts WHERE id=2; INSERT INTO flarum_posts VALUES(3,'new');", source)
                subprocess.run(args, check=True)
                query = 'SELECT id,content FROM flarum_posts ORDER BY id;'
                self.assertEqual(mirror.mysql(query, source), mirror.mysql(query, target))
                self.assertEqual(mirror.mysql('SELECT token FROM flarum_access_tokens;', target).strip(), 'session')
                before = mirror.mysql(query, target)
                mirror.mysql("UPDATE flarum_posts SET content='must rollback' WHERE id=1;", source)
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
