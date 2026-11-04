#!/usr/bin/env python3
"""告警通知接收器（AC-OPS-07-x）：记录 Alertmanager 发来的每一条通知，给测试断言。

POST /notify        记录请求体（Alertmanager 的 webhook JSON）
GET  /notifications 返回全部记录：[{receivedAtMs, payload}]
GET  /healthz       就绪检查
配了 NOTIFY_LOG 时同时追加到文件，重启不丢（AC-OPS-07-4 要跑两个多小时）。
"""
import json
import os
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_BODY = 1 << 20


class Store:
    def __init__(self, path=None):
        self.path = path
        self.lock = threading.Lock()
        self.rows = []
        if path and os.path.exists(path):
            with open(path, encoding="utf-8") as f:
                self.rows = [json.loads(line) for line in f if line.strip()]

    def add(self, payload):
        row = {"receivedAtMs": int(time.time() * 1000), "payload": payload}
        with self.lock:
            self.rows.append(row)
            if self.path:
                with open(self.path, "a", encoding="utf-8") as f:
                    f.write(json.dumps(row, ensure_ascii=False) + "\n")

    def all(self):
        with self.lock:
            return list(self.rows)


def handler(store):
    class Handler(BaseHTTPRequestHandler):
        def reply(self, code, body=b"", kind="application/json"):
            self.send_response(code)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_POST(self):
            if self.path != "/notify":
                return self.reply(404)
            size = int(self.headers.get("Content-Length") or 0)
            if size > MAX_BODY:
                return self.reply(413)
            try:
                payload = json.loads(self.rfile.read(size))
            except ValueError:
                return self.reply(400)
            store.add(payload)
            self.reply(200, b"{}")

        def do_GET(self):
            if self.path == "/notifications":
                return self.reply(200, json.dumps(store.all(), ensure_ascii=False).encode())
            if self.path == "/healthz":
                return self.reply(200, b"ok", "text/plain")
            self.reply(404)

        def log_message(self, *_):
            pass

    return Handler


def serve(port, path=None):
    return ThreadingHTTPServer(("0.0.0.0", port), handler(Store(path)))


if __name__ == "__main__":
    serve(int(os.environ.get("PORT", "9095")), os.environ.get("NOTIFY_LOG")).serve_forever()
