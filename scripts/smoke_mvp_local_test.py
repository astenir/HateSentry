import contextlib
import io
import subprocess
import tempfile
import unittest
from unittest import mock

from scripts import smoke_mvp_local


class SmokeBackupErrorHandlingTest(unittest.TestCase):
    def test_backup_timeout_does_not_expose_mysql_password(self):
        secret = "root-password-must-not-leak"
        timeout = subprocess.TimeoutExpired(["mysqldump", "-p" + secret], 60)

        with tempfile.TemporaryDirectory() as directory:
            backup_path = directory + "/backup.sql"
            with mock.patch.object(smoke_mvp_local, "MYSQL_ROOT_PASSWORD", secret):
                with mock.patch.object(subprocess, "run", side_effect=timeout):
                    with self.assertRaises(SystemExit) as raised:
                        smoke_mvp_local.verify_database_backup_restore_with_file(
                            "source",
                            "restored",
                            backup_path,
                        )

        self.assertEqual(str(raised.exception), "database backup timed out after 60 seconds")
        self.assertNotIn(secret, str(raised.exception))

    def test_restore_timeout_does_not_expose_mysql_password(self):
        secret = "root-password-must-not-leak"
        timeout = subprocess.TimeoutExpired(["mysql", "-p" + secret], 60)
        completed = subprocess.CompletedProcess([], 0)

        with tempfile.TemporaryDirectory() as directory:
            backup_path = directory + "/backup.sql"
            with open(backup_path, "wb"):
                pass
            with mock.patch.object(smoke_mvp_local, "MYSQL_ROOT_PASSWORD", secret):
                with mock.patch.object(smoke_mvp_local, "create_database"):
                    with mock.patch.object(subprocess, "run", side_effect=[completed, timeout]):
                        with self.assertRaises(SystemExit) as raised:
                            smoke_mvp_local.verify_database_backup_restore_with_file(
                                "source",
                                "restored",
                                backup_path,
                            )

        self.assertEqual(str(raised.exception), "database restore timed out after 60 seconds")
        self.assertNotIn(secret, str(raised.exception))

    def test_count_timeout_does_not_expose_mysql_password(self):
        secret = "root-password-must-not-leak"
        timeout = subprocess.TimeoutExpired(["mysql", "-p" + secret], 30)

        with mock.patch.object(smoke_mvp_local, "MYSQL_ROOT_PASSWORD", secret):
            with mock.patch.object(subprocess, "run", side_effect=timeout):
                with self.assertRaises(SystemExit) as raised:
                    smoke_mvp_local.database_table_counts("source")

        self.assertEqual(str(raised.exception), "database count query timed out after 30 seconds")
        self.assertNotIn(secret, str(raised.exception))

    def test_drop_database_reports_nonzero_exit(self):
        stderr = io.StringIO()
        completed = subprocess.CompletedProcess([], 1)

        with mock.patch.object(smoke_mvp_local, "run", return_value=completed):
            with contextlib.redirect_stderr(stderr):
                cleaned = smoke_mvp_local.drop_database("hatesentry_smoke_failed")

        self.assertFalse(cleaned)
        self.assertIn("hatesentry_smoke_failed", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
