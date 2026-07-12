import pathlib
import subprocess
import sys
import tempfile
import unittest

from scripts import release_preflight


SAFE_EXISTING = {
    "APP_VERSION": "0.2.0",
    "SERVER_MODE": "release",
    "JWT_SECRET": "jwt-" + "a" * 40,
    "DB_USERNAME": "hatesentry_app",
    "DB_PASSWORD": "db-B4e8L2q6U0y3I7o1P5a9D3f7",
    "MYSQL_ROOT_PASSWORD": "root-" + "c" * 24,
    "REDIS_PASSWORD": "redis-" + "c" * 24,
    "RABBITMQ_USERNAME": "hatesentry_queue",
    "RABBITMQ_PASSWORD": "queue-" + "d" * 24,
    "ADMIN_BOOTSTRAP_TOKEN": "",
    "AI_PROVIDER": "ollama",
}


class ReleasePreflightTest(unittest.TestCase):
    def test_accepts_existing_deployment_with_non_default_secrets(self):
        self.assertEqual(release_preflight.validate(SAFE_EXISTING), [])

    def test_accepts_initial_openai_bootstrap(self):
        values = {
            **SAFE_EXISTING,
            "AI_PROVIDER": "openai",
            "OPENAI_API_KEY": "sk-live-A3d9K7m2Q8x4R6t1V5z0P2n8",
            "ADMIN_BOOTSTRAP_TOKEN": "bootstrap-A3d9K7m2Q8x4R6t1V5z0",
        }
        self.assertEqual(release_preflight.validate(values, bootstrap=True), [])

    def test_rejects_local_defaults_without_echoing_values(self):
        values = {
            **SAFE_EXISTING,
            "SERVER_MODE": "debug",
            "JWT_SECRET": "dev-jwt-secret-change-me",
            "DB_USERNAME": "root",
            "DB_PASSWORD": "password",
            "MYSQL_ROOT_PASSWORD": "password",
            "REDIS_PASSWORD": "",
            "RABBITMQ_USERNAME": "guest",
            "RABBITMQ_PASSWORD": "guest",
            "AI_PROVIDER": "openai",
            "OPENAI_API_KEY": "your-openai-api-key",
            "ADMIN_BOOTSTRAP_TOKEN": "still-configured",
        }

        errors = release_preflight.validate(values)

        self.assertGreaterEqual(len(errors), 10)
        joined = "\n".join(errors)
        self.assertNotIn("dev-jwt-secret-change-me", joined)
        self.assertNotIn("your-openai-api-key", joined)

    def test_rejects_long_replace_placeholders(self):
        values = {**SAFE_EXISTING, "JWT_SECRET": "replace-with-at-least-32-random-characters"}

        errors = release_preflight.validate(values)

        self.assertTrue(any(error.startswith("JWT_SECRET") for error in errors))

    def test_rejects_obvious_test_and_low_diversity_secrets(self):
        self.assertFalse(release_preflight.safe_secret("sk-test-" + "a" * 40, 20))
        self.assertFalse(release_preflight.safe_secret("a" * 40, 32))

    def test_rejects_reused_secrets_without_printing_the_value(self):
        reused = "shared-A3d9K7m2Q8x4R6t1V5z0"
        values = {**SAFE_EXISTING, "JWT_SECRET": reused, "DB_PASSWORD": reused}

        errors = release_preflight.validate(values)

        self.assertTrue(any("JWT_SECRET, DB_PASSWORD" in error for error in errors))
        self.assertNotIn(reused, "\n".join(errors))

    def test_load_env_supports_comments_export_and_quotes(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "release.env"
            path.write_text("# comment\nexport SERVER_MODE=release\nJWT_SECRET='quoted-value'\n", encoding="utf-8")

            values = release_preflight.load_env(path)

        self.assertEqual(values, {"SERVER_MODE": "release", "JWT_SECRET": "quoted-value"})

    def test_load_env_rejects_non_assignment_lines(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "release.env"
            path.write_text("SERVER_MODE release\n", encoding="utf-8")

            with self.assertRaisesRegex(ValueError, "line 1"):
                release_preflight.load_env(path)

    def test_cli_passes_without_printing_configuration_values(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "release.env"
            path.write_text(
                "\n".join("{}={}".format(key, value) for key, value in SAFE_EXISTING.items()),
                encoding="utf-8",
            )
            path.chmod(0o600)

            completed = subprocess.run(
                [sys.executable, str(pathlib.Path(release_preflight.__file__)), str(path)],
                text=True,
                capture_output=True,
                check=False,
            )

        self.assertEqual(completed.returncode, 0)
        self.assertIn("existing deployment", completed.stdout)
        self.assertNotIn(SAFE_EXISTING["JWT_SECRET"], completed.stdout + completed.stderr)

    def test_unquoted_inline_comment_does_not_pad_a_short_secret(self):
        self.assertEqual(
            release_preflight.parse_env_value("short # this comment is intentionally very long", 1),
            "short",
        )

    def test_quoted_hash_is_preserved_and_trailing_comment_is_ignored(self):
        self.assertEqual(
            release_preflight.parse_env_value("'value # inside' # outside", 1),
            "value # inside",
        )

    def test_rejects_environment_file_readable_by_other_users(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "release.env"
            path.write_text("SERVER_MODE=release\n", encoding="utf-8")
            path.chmod(0o644)

            errors = release_preflight.validate_file_permissions(path)

        self.assertEqual(errors, ["release environment file must not be accessible by group or other users"])

    def test_rejects_interpolation_and_escape_syntax(self):
        for value in ("${SHARED_SECRET}", "quoted\\nvalue"):
            with self.subTest(value=value):
                with self.assertRaisesRegex(ValueError, "unsupported interpolation or escape"):
                    release_preflight.parse_env_value(value, 1)


if __name__ == "__main__":
    unittest.main()
