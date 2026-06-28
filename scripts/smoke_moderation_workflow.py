#!/usr/bin/env python3
"""Smoke-test the external-client text moderation workflow over HTTP."""

import json
import os
import sys
import urllib.error
import urllib.request
import uuid


BASE_URL = "http://localhost:8080"
TIMEOUT_SECONDS = 20.0


def main():
    global BASE_URL, TIMEOUT_SECONDS

    load_env_file()
    BASE_URL = os.getenv("HATESENTRY_BASE_URL", BASE_URL).rstrip("/")
    TIMEOUT_SECONDS = smoke_timeout_seconds()
    admin_token = admin_token_from_env_or_login()
    client = None
    cleanup_ok = True

    try:
        client = create_client(admin_token)

        external_id = os.getenv("HATESENTRY_EXTERNAL_ID")
        if not external_id:
            external_id = "smoke-" + uuid.uuid4().hex[:12]

        check = moderation_check(client["api_key"], external_id)
        validate_expected_decision(check.get("decision"))
        repeated_check = moderation_check(client["api_key"], external_id)
        if repeated_check["request_id"] != check["request_id"]:
            fail("repeated external_id did not return the original request_id")
        result = get_result(client["api_key"], check["request_id"])
        review = None

        if check.get("decision") == "review":
            review = finalize_review(admin_token, check["request_id"])
            result = get_result(client["api_key"], check["request_id"])

        summary = {
            "base_url": BASE_URL,
            "client_id": client["id"],
            "request_id": check["request_id"],
            "decision": check.get("decision"),
            "idempotency_reused": repeated_check["request_id"] == check["request_id"],
            "review_status": result.get("review_status"),
            "final_decision": result.get("final_decision"),
            "review_id": review.get("id") if review else None,
            "external_id": external_id,
        }
        print(json.dumps(summary, indent=2, sort_keys=True))

        if result["request_id"] != check["request_id"]:
            fail("result request_id did not match check request_id")
        if check.get("decision") == "review" and result.get("final_decision") not in ("allow", "block"):
            fail("review decision was finalized but result did not include final_decision")
    finally:
        if client and os.getenv("HATESENTRY_KEEP_SMOKE_CLIENT", "").strip() != "1":
            cleanup_ok = cleanup_client(admin_token, client["id"])
    if not cleanup_ok:
        fail("failed to deactivate temporary smoke client")


def smoke_timeout_seconds():
    raw_timeout = os.getenv("HATESENTRY_SMOKE_TIMEOUT", "20").strip()
    try:
        timeout = float(raw_timeout)
    except ValueError:
        fail("HATESENTRY_SMOKE_TIMEOUT must be a number of seconds")
    if timeout <= 0:
        fail("HATESENTRY_SMOKE_TIMEOUT must be greater than zero")
    return timeout


def load_env_file():
    env_path = os.getenv("HATESENTRY_ENV_FILE", default_env_file()).strip()
    if not env_path or not os.path.exists(env_path):
        return

    try:
        with open(env_path, "r", encoding="utf-8") as env_file:
            for raw_line in env_file:
                load_env_line(raw_line)
    except OSError as err:
        fail("failed to read env file {}: {}".format(env_path, err))


def default_env_file():
    return os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), ".env")


def load_env_line(raw_line):
    line = raw_line.strip()
    if not line or line.startswith("#") or "=" not in line:
        return

    key, value = line.split("=", 1)
    key = key.strip()
    if not key or key in os.environ:
        return

    os.environ[key] = strip_optional_quotes(value.strip())


def strip_optional_quotes(value):
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value


def validate_expected_decision(decision):
    expected = os.getenv("HATESENTRY_EXPECT_DECISION", "").strip().lower()
    if not expected:
        return
    if expected not in ("allow", "review", "block"):
        fail("HATESENTRY_EXPECT_DECISION must be allow, review, or block")
    if decision != expected:
        fail("moderation decision was {}, expected {}".format(decision, expected))


def admin_token_from_env_or_login():
    token = os.getenv("HATESENTRY_ADMIN_TOKEN", "").strip()
    if token:
        return token

    email = os.getenv("HATESENTRY_ADMIN_EMAIL", "").strip()
    password = os.getenv("HATESENTRY_ADMIN_PASSWORD", "").strip()
    if not email or not password:
        fail(
            "set HATESENTRY_ADMIN_TOKEN, or set HATESENTRY_ADMIN_EMAIL and "
            "HATESENTRY_ADMIN_PASSWORD"
        )

    try:
        return login_admin(email, password)
    except SmokeHTTPError as err:
        bootstrap_token = admin_bootstrap_token()
        if not bootstrap_token:
            fail(
                "admin login failed: {}; set HATESENTRY_ADMIN_TOKEN, or use "
                "valid admin credentials. For a fresh database, set "
                "HATESENTRY_ADMIN_BOOTSTRAP_TOKEN to the same value as the "
                "running service ADMIN_BOOTSTRAP_TOKEN.".format(err)
            )

        try:
            return register_initial_admin(email, password, bootstrap_token)
        except SmokeHTTPError as register_err:
            fail(
                "admin login failed and bootstrap registration failed: login {}; "
                "register {}. Use an existing admin token or reset the supplied "
                "admin credentials.".format(err, register_err)
            )


def login_admin(email, password):
    response = request_json(
        "POST",
        "/api/v1/auth/login",
        payload={
            "email": email,
            "password": password,
        },
        expected_statuses=(200,),
    )
    token = response.get("token", "")
    if not token:
        fail("login response did not include token")
    if response.get("user", {}).get("role") != "admin":
        fail("logged-in user is not an admin")

    return token


def admin_bootstrap_token():
    token = os.getenv("HATESENTRY_ADMIN_BOOTSTRAP_TOKEN", "").strip()
    if token:
        return token
    return os.getenv("ADMIN_BOOTSTRAP_TOKEN", "").strip()


def register_initial_admin(email, password, bootstrap_token):
    response = request_json(
        "POST",
        "/api/v1/auth/register",
        payload={
            "username": os.getenv("HATESENTRY_ADMIN_USERNAME", "smoke-admin"),
            "email": email,
            "password": password,
            "admin_bootstrap_token": bootstrap_token,
        },
        expected_statuses=(201,),
    )
    token = response.get("token", "")
    if not token:
        fail("registration response did not include token")
    if response.get("user", {}).get("role") != "admin":
        fail("registered user is not an admin")

    return token


def create_client(admin_token):
    response = request_json(
        "POST",
        "/api/v1/admin/clients",
        headers=admin_headers(admin_token),
        payload={
            "name": os.getenv("HATESENTRY_SMOKE_CLIENT_NAME", "smoke-client"),
            "policy_version": os.getenv("HATESENTRY_POLICY_VERSION", "default-v1"),
        },
        expected_statuses=(201,),
    )
    if not response.get("api_key"):
        fail("client creation response did not include api_key")
    return response


def cleanup_client(admin_token, client_id):
    try:
        request_json(
            "POST",
            "/api/v1/admin/clients/{}/deactivate".format(client_id),
            headers=admin_headers(admin_token),
            expected_statuses=(200,),
        )
        return True
    except SmokeHTTPError as err:
        print(
            "smoke workflow warning: failed to deactivate client {}: {}".format(
                client_id,
                err,
            ),
            file=sys.stderr,
        )
    except SystemExit as err:
        print(
            "smoke workflow warning: failed to deactivate client {}: exit {}".format(
                client_id,
                err.code,
            ),
            file=sys.stderr,
        )
    return False


def moderation_check(api_key, external_id):
    content = os.getenv(
        "HATESENTRY_SMOKE_CONTENT",
        "Please review this moderation smoke-test comment.",
    )
    response = request_json(
        "POST",
        "/api/v1/moderation/check",
        headers={
            "X-API-Key": api_key,
            "Content-Type": "application/json",
        },
        payload={
            "content": content,
            "source": os.getenv("HATESENTRY_SMOKE_SOURCE", "comment"),
            "external_id": external_id,
            "actor_id": os.getenv("HATESENTRY_SMOKE_ACTOR_ID", "smoke-user"),
        },
        expected_statuses=(200,),
    )
    decision = response.get("decision")
    if decision not in ("allow", "review", "block"):
        fail("moderation response decision must be allow, review, or block")
    if not response.get("request_id"):
        fail("moderation response did not include request_id")
    return response


def finalize_review(admin_token, request_id):
    pending = request_json(
        "GET",
        "/api/v1/reviews?status=pending",
        headers=admin_headers(admin_token),
        expected_statuses=(200,),
    )
    review = next(
        (item for item in pending.get("items", []) if item.get("request_id") == request_id),
        None,
    )
    if not review:
        fail("moderation check returned review but pending review case was not found")

    action = os.getenv("HATESENTRY_REVIEW_ACTION", "approve").strip().lower()
    if action not in ("approve", "reject"):
        fail("HATESENTRY_REVIEW_ACTION must be approve or reject")

    return request_json(
        "POST",
        "/api/v1/reviews/{}/{}".format(review["id"], action),
        headers=admin_headers(admin_token),
        payload={
            "notes": "finalized by moderation smoke workflow",
        },
        expected_statuses=(200,),
    )


def get_result(api_key, request_id):
    response = request_json(
        "GET",
        "/api/v1/moderation/results/{}".format(request_id),
        headers={
            "X-API-Key": api_key,
        },
        expected_statuses=(200,),
    )
    if "raw_output" in response or "reviewer_id" in response or "review_notes" in response:
        fail("public moderation result exposed internal provider or review fields")
    return response


def admin_headers(token):
    return {
        "Authorization": "Bearer " + token,
        "Content-Type": "application/json",
    }


def request_json(method, path, headers=None, payload=None, expected_statuses=(200,)):
    headers = dict(headers or {})
    data = None
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers.setdefault("Content-Type", "application/json")

    request = urllib.request.Request(
        BASE_URL + path,
        data=data,
        headers=headers,
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=TIMEOUT_SECONDS) as response:
            body = response.read()
            status = response.getcode()
    except urllib.error.HTTPError as err:
        body = err.read()
        message = decode_body(body)
        raise SmokeHTTPError(err.code, message) from err
    except urllib.error.URLError as err:
        fail("request failed: {}".format(err))

    if status not in expected_statuses:
        fail(
            "{} {} returned HTTP {}, expected {}; body={}".format(
                method,
                path,
                status,
                expected_statuses,
                decode_body(body),
            )
        )
    if not body:
        return {}
    try:
        return json.loads(body.decode("utf-8"))
    except json.JSONDecodeError as err:
        fail("response was not JSON: {}; body={}".format(err, decode_body(body)))


def decode_body(body):
    return body.decode("utf-8", errors="replace") if body else ""


def fail(message):
    print("smoke workflow failed: {}".format(message), file=sys.stderr)
    sys.exit(1)


class SmokeHTTPError(Exception):
    def __init__(self, status, body):
        super().__init__("HTTP {}: {}".format(status, body))
        self.status = status
        self.body = body


if __name__ == "__main__":
    try:
        main()
    except SmokeHTTPError as err:
        fail(str(err))
    except KeyboardInterrupt:
        print("smoke workflow interrupted", file=sys.stderr)
        sys.exit(130)
