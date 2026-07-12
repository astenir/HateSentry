#!/usr/bin/env python3
"""Run a local end-to-end smoke test for the text moderation MVP."""

import json
import os
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


ROOT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MYSQL_ROOT_PASSWORD = os.getenv(
    "HATESENTRY_SMOKE_MYSQL_PASSWORD",
    os.getenv("MYSQL_ROOT_PASSWORD", "password"),
)
REDIS_PASSWORD = os.getenv(
    "HATESENTRY_SMOKE_REDIS_PASSWORD",
    os.getenv("REDIS_PASSWORD", ""),
)
RABBITMQ_USERNAME = os.getenv(
    "HATESENTRY_SMOKE_RABBITMQ_USERNAME",
    os.getenv("RABBITMQ_USERNAME", "guest"),
)
RABBITMQ_PASSWORD = os.getenv(
    "HATESENTRY_SMOKE_RABBITMQ_PASSWORD",
    os.getenv("RABBITMQ_PASSWORD", "guest"),
)
APP_VERSION = os.getenv("HATESENTRY_SMOKE_APP_VERSION", "0.2.0")
ADMIN_EMAIL = "smoke-admin@example.test"
ADMIN_PASSWORD = "password123"
ADMIN_BOOTSTRAP_TOKEN = "smoke-bootstrap-token"
DEPENDENCY_WAIT_SECONDS = 60
DEPENDENCY_PROBE_TIMEOUT_SECONDS = 5
CLEANUP_TIMEOUT_SECONDS = 10


def main():
    api_process = None
    api_log_path = None
    openai_server = None
    failed = False
    database_name = "hatesentry_smoke_" + uuid.uuid4().hex[:12]
    restored_database_name = database_name + "_restore"

    try:
        run(
            ["docker", "compose", "up", "-d", "mysql", "redis", "rabbitmq"],
            env=smoke_compose_environment(),
        )
        wait_for_mysql()
        wait_for_redis()
        wait_for_rabbitmq()
        create_database(database_name)

        openai_server = start_openai_stub()
        api_port = free_port()
        api_process, api_log_path = start_api(
            database_name, api_port, openai_server.server_port
        )
        wait_for_health(api_port)

        run_smoke_workflow(api_port)
        if console_smoke_enabled():
            run_console_workflow(api_port)
        verify_database_backup_restore(
            database_name,
            restored_database_name,
        )
    except BaseException:
        failed = True
        raise
    finally:
        if api_process is not None:
            stop_process(api_process)
        if failed and api_log_path is not None:
            print_api_log(api_log_path)
        if openai_server is not None:
            openai_server.shutdown()
            openai_server.server_close()
        restored_cleaned = drop_database(restored_database_name)
        source_cleaned = drop_database(database_name)
        if api_log_path is not None:
            try:
                os.remove(api_log_path)
            except FileNotFoundError:
                pass
        if not (restored_cleaned and source_cleaned) and not failed:
            raise SystemExit("smoke database cleanup failed")


def start_openai_stub():
    server = ThreadingHTTPServer(("127.0.0.1", 0), OpenAIStubHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


def smoke_compose_environment():
    env = os.environ.copy()
    env.update(
        {
            "MYSQL_ROOT_PASSWORD": MYSQL_ROOT_PASSWORD,
            "DB_USERNAME": "hatesentry_smoke",
            "DB_PASSWORD": "smoke-app-password",
            "REDIS_PASSWORD": REDIS_PASSWORD,
            "RABBITMQ_USERNAME": RABBITMQ_USERNAME,
            "RABBITMQ_PASSWORD": RABBITMQ_PASSWORD,
        }
    )
    return env


class OpenAIStubHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if not self.path.endswith("/chat/completions"):
            self.send_json(404, {"error": {"message": "not found"}})
            return

        request = self.read_json_body()
        if request is None:
            return

        suggestion = {
            "risk_score": 0.6,
            "labels": ["harassment"],
            "reason": "Local smoke stub returns a review-level moderation risk.",
        }
        response = {
            "id": "chatcmpl-smoke",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": request.get("model", "smoke-model"),
            "choices": [
                {
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": json.dumps(suggestion, separators=(",", ":")),
                    },
                    "finish_reason": "stop",
                }
            ],
        }
        self.send_json(200, response)

    def read_json_body(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self.send_json(400, {"error": {"message": "invalid content length"}})
            return None

        try:
            return json.loads(self.rfile.read(length).decode("utf-8"))
        except json.JSONDecodeError as err:
            self.send_json(400, {"error": {"message": "invalid json: " + str(err)}})
            return None

    def send_json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        return


def start_api(database_name, api_port, openai_port):
    env = os.environ.copy()
    env.update(
        {
            "SERVER_HOST": "127.0.0.1",
            "SERVER_PORT": str(api_port),
            "DB_HOST": "127.0.0.1",
            "DB_PORT": "3306",
            "DB_USERNAME": "root",
            "DB_PASSWORD": MYSQL_ROOT_PASSWORD,
            "DB_DATABASE": database_name,
            "REDIS_HOST": "127.0.0.1",
            "REDIS_PORT": "6379",
            "REDIS_PASSWORD": REDIS_PASSWORD,
            "RABBITMQ_HOST": "127.0.0.1",
            "RABBITMQ_PORT": "5672",
            "RABBITMQ_USERNAME": RABBITMQ_USERNAME,
            "RABBITMQ_PASSWORD": RABBITMQ_PASSWORD,
            "ADMIN_BOOTSTRAP_TOKEN": ADMIN_BOOTSTRAP_TOKEN,
            "JWT_SECRET": "local-smoke-jwt-secret",
            "AI_PROVIDER": "openai",
            "OPENAI_API_KEY": "local-smoke-key",
            "OPENAI_BASE_URL": "http://127.0.0.1:{}/v1".format(openai_port),
            "OPENAI_MODEL": "local-smoke-model",
            "LOG_LEVEL": "error",
            "LOG_FORMAT": "console",
            "MODERATION_WEBHOOK_RETRY_ENABLED": "false",
            "APP_VERSION": APP_VERSION,
        }
    )

    log_file = tempfile.NamedTemporaryFile(
        mode="w",
        prefix="hatesentry-smoke-api-",
        suffix=".log",
        delete=False,
    )
    print("local API log: {}".format(log_file.name))

    process = subprocess.Popen(
        ["go", "run", "."],
        cwd=ROOT_DIR,
        env=env,
        stdout=log_file,
        stderr=subprocess.STDOUT,
        text=True,
        start_new_session=True,
    )
    return process, log_file.name


def wait_for_health(api_port):
    url = "http://127.0.0.1:{}/api/v1/health".format(api_port)
    deadline = time.monotonic() + 60
    last_error = ""

    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2) as response:
                if response.getcode() == 200:
                    payload = json.load(response)
                    if payload.get("version") != APP_VERSION:
                        raise SystemExit(
                            "local API version = {!r}, want {!r}".format(
                                payload.get("version"),
                                APP_VERSION,
                            )
                        )
                    return
        except (TimeoutError, urllib.error.URLError) as err:
            last_error = str(err)
        time.sleep(1)

    raise SystemExit("local API health check did not pass: " + last_error)


def run_smoke_workflow(api_port):
    env = os.environ.copy()
    env.update(
        {
            "HATESENTRY_ENV_FILE": "",
            "HATESENTRY_BASE_URL": "http://127.0.0.1:{}".format(api_port),
            "HATESENTRY_ADMIN_EMAIL": ADMIN_EMAIL,
            "HATESENTRY_ADMIN_PASSWORD": ADMIN_PASSWORD,
            "HATESENTRY_ADMIN_BOOTSTRAP_TOKEN": ADMIN_BOOTSTRAP_TOKEN,
            "HATESENTRY_ADMIN_USERNAME": "smoke-admin",
            "HATESENTRY_EXPECT_DECISION": "review",
            "HATESENTRY_SMOKE_CONTENT": "Local smoke test content requiring review.",
            "HATESENTRY_SMOKE_TIMEOUT": "30",
        }
    )
    run(["python3", "scripts/smoke_moderation_workflow.py"], env=env)


def run_console_workflow(api_port):
    env = os.environ.copy()
    env.update(
        {
            "HATESENTRY_BASE_URL": "http://127.0.0.1:{}".format(api_port),
            "HATESENTRY_ADMIN_EMAIL": ADMIN_EMAIL,
            "HATESENTRY_ADMIN_PASSWORD": ADMIN_PASSWORD,
            "HATESENTRY_CONSOLE_SMOKE_CONTENT": "Browser smoke content requiring review.",
        }
    )
    run(["node", "web/scripts/smoke_review_console.mjs"], env=env)


def console_smoke_enabled():
    return os.getenv("HATESENTRY_SMOKE_CONSOLE", "").strip().lower() in {
        "1",
        "true",
        "yes",
    }


def create_database(database_name):
    run(
        mysql_root_command(
            "mysql",
            "-uroot",
            "-e",
            "CREATE DATABASE `{}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci".format(
                database_name
            ),
        )
    )


def mysql_root_command(*args):
    return [
        "docker",
        "compose",
        "exec",
        "-T",
        "mysql",
        "sh",
        "-c",
        'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec "$@"',
        "hatesentry-mysql-root",
        *args,
    ]


def verify_database_backup_restore(source_database, restored_database):
    with tempfile.TemporaryDirectory(prefix="hatesentry-release-backup-") as directory:
        backup_path = os.path.join(directory, "backup.sql")
        verify_database_backup_restore_with_file(
            source_database,
            restored_database,
            backup_path,
        )


def verify_database_backup_restore_with_file(source_database, restored_database, backup_path):
    dump_args = mysql_root_command(
        "mysqldump",
        "-uroot",
        "--single-transaction",
        "--skip-lock-tables",
        "--no-tablespaces",
        source_database,
    )
    print("+ {} > <private-temporary-backup>".format(" ".join(redact_command_args(dump_args))))
    try:
        with open(backup_path, "wb") as output:
            completed = subprocess.run(
                dump_args,
                cwd=ROOT_DIR,
                stdout=output,
                stderr=subprocess.PIPE,
                timeout=60,
            )
    except subprocess.TimeoutExpired:
        raise SystemExit("database backup timed out after 60 seconds") from None
    if completed.returncode != 0:
        raise SystemExit("database backup failed with exit {}".format(completed.returncode))

    create_database(restored_database)
    restore_args = mysql_root_command(
        "mysql",
        "-uroot",
        restored_database,
    )
    print("+ {} < <private-temporary-backup>".format(" ".join(redact_command_args(restore_args))))
    try:
        with open(backup_path, "rb") as source:
            completed = subprocess.run(
                restore_args,
                cwd=ROOT_DIR,
                stdin=source,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.PIPE,
                timeout=60,
            )
    except subprocess.TimeoutExpired:
        raise SystemExit("database restore timed out after 60 seconds") from None
    if completed.returncode != 0:
        raise SystemExit("database restore failed with exit {}".format(completed.returncode))

    source_counts = database_table_counts(source_database)
    restored_counts = database_table_counts(restored_database)
    if source_counts != restored_counts:
        raise SystemExit("restored database table counts do not match the source database")

    for table in (
        "users",
        "client_applications",
        "moderation_requests",
        "moderation_results",
        "review_cases",
    ):
        if source_counts.get(table, 0) < 1:
            raise SystemExit("backup verification source table {} is unexpectedly empty".format(table))

    print(json.dumps({
        "backup_restore_verified": True,
        "table_counts": source_counts,
    }, indent=2, sort_keys=True))


def database_table_counts(database_name):
    tables = (
        "users",
        "client_applications",
        "moderation_requests",
        "moderation_results",
        "review_cases",
        "webhook_deliveries",
    )
    query = " UNION ALL ".join(
        "SELECT '{}', COUNT(*) FROM `{}`".format(table, table)
        for table in tables
    )
    args = mysql_root_command(
        "mysql",
        "-N",
        "-B",
        "-uroot",
        database_name,
        "-e",
        query,
    )
    print("+ {} # compare release backup counts".format(" ".join(redact_command_args(args))))
    try:
        completed = subprocess.run(
            args,
            cwd=ROOT_DIR,
            text=True,
            capture_output=True,
            timeout=30,
        )
    except subprocess.TimeoutExpired:
        raise SystemExit("database count query timed out after 30 seconds") from None
    if completed.returncode != 0:
        raise SystemExit("database count query failed with exit {}".format(completed.returncode))

    counts = {}
    for line in completed.stdout.splitlines():
        table, count = line.split("\t", 1)
        counts[table] = int(count)
    return counts


def wait_for_mysql():
    wait_for_dependency(
        "MySQL",
        mysql_root_command(
            "mysqladmin",
            "ping",
            "-h",
            "127.0.0.1",
            "-uroot",
            "--silent",
        ),
    )


def wait_for_redis():
    wait_for_dependency(
        "Redis",
        [
            "docker",
            "compose",
            "exec",
            "-T",
            "redis",
            "sh",
            "-c",
            'if [ -n "$REDIS_PASSWORD" ]; then REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli ping; else redis-cli ping; fi',
        ],
    )


def wait_for_rabbitmq():
    wait_for_dependency(
        "RabbitMQ",
        [
            "docker",
            "compose",
            "exec",
            "-T",
            "rabbitmq",
            "rabbitmq-diagnostics",
            "-q",
            "check_port_connectivity",
        ],
    )


def wait_for_dependency(name, args):
    deadline = time.monotonic() + DEPENDENCY_WAIT_SECONDS

    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            break
        try:
            completed = subprocess.run(
                args,
                cwd=ROOT_DIR,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                text=True,
                timeout=min(DEPENDENCY_PROBE_TIMEOUT_SECONDS, remaining),
            )
        except subprocess.TimeoutExpired:
            continue
        if completed.returncode == 0:
            print("+ {} # {} ready".format(" ".join(redact_command_args(args)), name))
            return
        time.sleep(min(1, max(0, deadline - time.monotonic())))

    raise SystemExit(
        "{} did not become ready within {} seconds".format(
            name, DEPENDENCY_WAIT_SECONDS
        )
    )


def drop_database(database_name):
    try:
        completed = run(
            mysql_root_command(
                "mysql",
                "-uroot",
                "-e",
                "DROP DATABASE IF EXISTS `{}`".format(database_name),
            ),
            check=False,
            timeout=CLEANUP_TIMEOUT_SECONDS,
        )
    except subprocess.TimeoutExpired:
        print(
            "warning: smoke database cleanup timed out after {} seconds".format(
                CLEANUP_TIMEOUT_SECONDS
            ),
            file=sys.stderr,
        )
        return False
    if completed.returncode != 0:
        print(
            "warning: failed to drop smoke database {}".format(database_name),
            file=sys.stderr,
        )
        return False
    return True


def stop_process(process):
    if process.poll() is not None:
        return

    os.killpg(process.pid, signal.SIGTERM)
    try:
        process.wait(timeout=15)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=5)


def print_api_log(path):
    print("--- local API log (last 200 lines) ---", file=sys.stderr)
    try:
        with open(path, encoding="utf-8", errors="replace") as log_file:
            lines = log_file.readlines()
    except OSError as err:
        print("unable to read local API log: {}".format(err), file=sys.stderr)
        return

    for line in lines[-200:]:
        print(line, end="", file=sys.stderr)
    print("--- end local API log ---", file=sys.stderr)


def free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def run(args, env=None, check=True, timeout=None):
    display_args = redact_command_args(args)
    print("+ " + " ".join(display_args))
    completed = subprocess.run(
        args,
        cwd=ROOT_DIR,
        env=env,
        text=True,
        timeout=timeout,
    )
    if check and completed.returncode != 0:
        raise SystemExit(
            "command failed with exit {}: {}".format(
                completed.returncode, display_args
            )
        )
    return completed


def redact_command_args(args):
    redacted = []
    for arg in args:
        if arg.startswith("-p") and len(arg) > 2:
            redacted.append("-p***")
        elif arg.startswith("--password="):
            redacted.append("--password=***")
        else:
            redacted.append(arg)
    return redacted


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("local smoke interrupted", file=sys.stderr)
        sys.exit(130)
