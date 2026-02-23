#!/usr/bin/env python3
"""Simple HTTP server that logs full request details for debugging RadosGW notifications."""

from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import sys
from datetime import datetime


class NotificationHandler(BaseHTTPRequestHandler):
    request_count = 0

    def _handle(self):
        NotificationHandler.request_count += 1
        count = NotificationHandler.request_count
        timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length).decode("utf-8") if content_length > 0 else ""

        print(f"\n{'='*72}")
        print(f"[#{count}] {timestamp} — {self.command} {self.path}")
        print(f"{'-'*72}")
        print(f"Headers:")
        for key, val in self.headers.items():
            print(f"  {key}: {val}")

        if body:
            print(f"Body:")
            try:
                parsed = json.loads(body)
                print(json.dumps(parsed, indent=2))
            except json.JSONDecodeError:
                print(body)

        print(f"{'='*72}")
        sys.stdout.flush()

        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(b"OK")

    def do_POST(self):
        self._handle()

    def do_GET(self):
        self._handle()

    def do_PUT(self):
        self._handle()

    def log_message(self, format, *args):
        pass  # suppress default logging


if __name__ == "__main__":
    port = 10900
    server = HTTPServer(("0.0.0.0", port), NotificationHandler)
    print(f"Notification listener started on port {port}")
    print(f"Waiting for RadosGW notifications...")
    sys.stdout.flush()
    server.serve_forever()
