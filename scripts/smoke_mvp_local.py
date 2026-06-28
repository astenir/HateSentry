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
MYSQL_ROOT_PASSWORD = os.getenv("HATESENTRY_SMOKE_MYSQL_PASSWORD", "password")
ADMIN_EMAIL = "smoke-admin@example.test"
ADMIN_PASSWORD = "password123"
ADMIN_BOOTSTRAP_TOKEN = "smoke-bootstrap-token"


def main():
    api_process = None
    openai_server = None
    database_name = "hatesentry_smoke_" + uuid.uuid4().hex[:12]

    try:
        run(["docker", "compose", "up", "-d", "mysql", "redis", "rabbitmq"])
        create_database(database_name)

        openai_server = start_openai_stub()
        api_port = free_port()
        api_process = start_api(database_name, api_port, openai_server.server_port)
        wait_for_health(api_port)

        run_smoke_workflow(api_port)
    finally:
        if api_process is not None:
            stop_process(api_process)
        if openai_server is not None:
            openai_server.shutdown()
            openai_server.server_close()
        drop_database(database_name)


def start_openai_stub():
    server = ThreadingHTTPServer(("127.0.0.1", 0), OpenAIStubHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


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
            "RABBITMQ_HOST": "127.0.0.1",
            "RABBITMQ_PORT": "5672",
            "RABBITMQ_USERNAME": "guest",
            "RABBITMQ_PASSWORD": "guest",
            "ADMIN_BOOTSTRAP_TOKEN": ADMIN_BOOTSTRAP_TOKEN,
            "JWT_SECRET": "local-smoke-jwt-secret",
            "AI_PROVIDER": "openai",
            "OPENAI_API_KEY": "local-smoke-key",
            "OPENAI_BASE_URL": "http://127.0.0.1:{}/v1".format(openai_port),
            "OPENAI_MODEL": "local-smoke-model",
            "LOG_LEVEL": "error",
            "LOG_FORMAT": "console",
            "MODERATION_WEBHOOK_RETRY_ENABLED": "false",
        }
    )

    log_file = tempfile.NamedTemporaryFile(
        mode="w",
        prefix="hatesentry-smoke-api-",
        suffix=".log",
        delete=False,
    )
    print("local API log: {}".format(log_file.name))

    return subprocess.Popen(
        ["go", "run", "."],
        cwd=ROOT_DIR,
        env=env,
        stdout=log_file,
        stderr=subprocess.STDOUT,
        text=True,
        start_new_session=True,
    )


def wait_for_health(api_port):
    url = "http://127.0.0.1:{}/api/v1/health".format(api_port)
    deadline = time.monotonic() + 60
    last_error = ""

    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2) as response:
                if response.getcode() == 200:
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


def create_database(database_name):
    run(
        [
            "docker",
            "compose",
            "exec",
            "-T",
            "mysql",
            "mysql",
            "-uroot",
            "-p" + MYSQL_ROOT_PASSWORD,
            "-e",
            "CREATE DATABASE `{}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci".format(
                database_name
            ),
        ]
    )


def drop_database(database_name):
    run(
        [
            "docker",
            "compose",
            "exec",
            "-T",
            "mysql",
            "mysql",
            "-uroot",
            "-p" + MYSQL_ROOT_PASSWORD,
            "-e",
            "DROP DATABASE IF EXISTS `{}`".format(database_name),
        ],
        check=False,
    )


def stop_process(process):
    if process.poll() is not None:
        return

    os.killpg(process.pid, signal.SIGTERM)
    try:
        process.wait(timeout=15)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=5)


def free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def run(args, env=None, check=True):
    print("+ " + " ".join(args))
    completed = subprocess.run(
        args,
        cwd=ROOT_DIR,
        env=env,
        text=True,
    )
    if check and completed.returncode != 0:
        raise SystemExit(
            "command failed with exit {}: {}".format(completed.returncode, args)
        )
    return completed


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("local smoke interrupted", file=sys.stderr)
        sys.exit(130)
