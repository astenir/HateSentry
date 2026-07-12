#!/usr/bin/env python3
"""Validate a HateSentry release environment file without printing secrets."""

import argparse
import pathlib
import stat
import sys


KNOWN_UNSAFE_VALUES = {
    "password",
    "hatesentry",
    "guest",
    "dev-jwt-secret-change-me",
    "your-openai-api-key",
}
RELEASE_VERSION = "0.2.0"


def load_env(path):
    values = {}
    for line_number, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[7:].strip()
        if "=" not in line:
            raise ValueError("line {} is not a KEY=VALUE assignment".format(line_number))
        key, value = line.split("=", 1)
        key = key.strip()
        values[key] = parse_env_value(value, line_number)
    return values


def parse_env_value(raw_value, line_number):
    value = raw_value.strip()
    if not value:
        return ""
    if "$" in value or "\\" in value:
        raise ValueError(
            "line {} uses unsupported interpolation or escape syntax".format(line_number)
        )
    if value[0] not in {'"', "'"}:
        for index, character in enumerate(value):
            if character == "#" and index > 0 and value[index - 1].isspace():
                value = value[:index]
                break
        return value.rstrip()

    quote = value[0]
    closing = 1
    while closing < len(value):
        if value[closing] == quote and value[closing - 1] != "\\":
            break
        closing += 1
    if closing >= len(value):
        raise ValueError("line {} has an unterminated quoted value".format(line_number))
    remainder = value[closing + 1:].strip()
    if remainder and not remainder.startswith("#"):
        raise ValueError("line {} has content after a quoted value".format(line_number))
    return value[1:closing]


def validate_file_permissions(path):
    mode = stat.S_IMODE(path.stat().st_mode)
    if mode & 0o077:
        return ["release environment file must not be accessible by group or other users"]
    return []


def validate(values, bootstrap=False):
    errors = []

    require_exact(values, "APP_VERSION", RELEASE_VERSION, errors)
    require_exact(values, "SERVER_MODE", "release", errors)
    require_secret(values, "JWT_SECRET", 32, errors)
    require_non_root_database_user(values, errors)
    require_secret(values, "DB_PASSWORD", 16, errors)
    require_secret(values, "MYSQL_ROOT_PASSWORD", 16, errors)
    require_secret(values, "REDIS_PASSWORD", 16, errors)
    require_non_guest_rabbit_user(values, errors)
    require_secret(values, "RABBITMQ_PASSWORD", 16, errors)

    provider = values.get("AI_PROVIDER", "").strip().lower()
    if provider not in {"openai", "ollama"}:
        errors.append("AI_PROVIDER must be openai or ollama")
    elif provider == "openai":
        require_secret(values, "OPENAI_API_KEY", 20, errors)

    bootstrap_token = values.get("ADMIN_BOOTSTRAP_TOKEN", "").strip()
    if bootstrap:
        if not safe_secret(bootstrap_token, 24):
            errors.append("ADMIN_BOOTSTRAP_TOKEN must be a non-placeholder value of at least 24 characters for initial bootstrap")
    elif bootstrap_token:
        errors.append("ADMIN_BOOTSTRAP_TOKEN must be empty after initial administrator bootstrap")

    secret_keys = [
        "JWT_SECRET",
        "DB_PASSWORD",
        "MYSQL_ROOT_PASSWORD",
        "REDIS_PASSWORD",
        "RABBITMQ_PASSWORD",
    ]
    if bootstrap:
        secret_keys.append("ADMIN_BOOTSTRAP_TOKEN")
    if provider == "openai":
        secret_keys.append("OPENAI_API_KEY")
    require_distinct_secrets(values, secret_keys, errors)

    return errors


def require_exact(values, key, expected, errors):
    if values.get(key, "").strip() != expected:
        errors.append("{} must be {}".format(key, expected))


def require_secret(values, key, minimum_length, errors):
    if not safe_secret(values.get(key, ""), minimum_length):
        errors.append("{} must be a non-placeholder value of at least {} characters".format(key, minimum_length))


def safe_secret(value, minimum_length):
    normalized = value.strip()
    lowered = normalized.lower()
    return (
        len(normalized) >= minimum_length
        and lowered not in KNOWN_UNSAFE_VALUES
        and not lowered.startswith(("replace-", "your-"))
        and not any(marker in lowered for marker in ("changeme", "example", "sk-test"))
        and len(set(normalized)) >= 4
    )


def require_distinct_secrets(values, keys, errors):
    owners = {}
    for key in keys:
        value = values.get(key, "").strip()
        if not value:
            continue
        owners.setdefault(value, []).append(key)
    for duplicate_keys in owners.values():
        if len(duplicate_keys) > 1:
            errors.append("{} must use distinct values".format(", ".join(duplicate_keys)))


def require_non_root_database_user(values, errors):
    username = values.get("DB_USERNAME", "").strip().lower()
    if not username or username == "root":
        errors.append("DB_USERNAME must be a non-root application user")


def require_non_guest_rabbit_user(values, errors):
    username = values.get("RABBITMQ_USERNAME", "").strip().lower()
    if not username or username == "guest":
        errors.append("RABBITMQ_USERNAME must be a non-guest application user")


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("env_file", type=pathlib.Path)
    parser.add_argument(
        "--bootstrap",
        action="store_true",
        help="require a one-time ADMIN_BOOTSTRAP_TOKEN for a fresh database",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        permission_errors = validate_file_permissions(args.env_file)
        values = load_env(args.env_file)
    except (OSError, ValueError) as error:
        print("release preflight could not read the environment file: {}".format(error), file=sys.stderr)
        return 2

    errors = permission_errors + validate(values, bootstrap=args.bootstrap)
    if errors:
        print("release configuration preflight failed:", file=sys.stderr)
        for error in errors:
            print("- " + error, file=sys.stderr)
        return 1

    mode = "initial bootstrap" if args.bootstrap else "existing deployment"
    print("release configuration preflight passed for {}".format(mode))
    return 0


if __name__ == "__main__":
    sys.exit(main())
