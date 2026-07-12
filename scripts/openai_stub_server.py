#!/usr/bin/env python3
"""Small OpenAI-compatible server used only by release smoke tests."""

import json
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class OpenAIStubHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.send_json(200, {"status": "healthy"})
            return
        self.send_json(404, {"error": {"message": "not found"}})

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
            "reason": "Release smoke stub returns a review-level moderation risk.",
        }
        self.send_json(200, {
            "id": "chatcmpl-release-smoke",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": request.get("model", "release-smoke-model"),
            "choices": [{
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": json.dumps(suggestion, separators=(",", ":")),
                },
                "finish_reason": "stop",
            }],
        })

    def read_json_body(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
            return json.loads(self.rfile.read(length).decode("utf-8"))
        except (ValueError, json.JSONDecodeError) as error:
            self.send_json(400, {"error": {"message": "invalid json: " + str(error)}})
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


def main():
    port = int(os.getenv("PORT", "8000"))
    ThreadingHTTPServer(("0.0.0.0", port), OpenAIStubHandler).serve_forever()


if __name__ == "__main__":
    main()
