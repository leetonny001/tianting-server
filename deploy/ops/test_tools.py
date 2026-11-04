#!/usr/bin/env python3
"""deploy/ops 工具的测试：python3 deploy/ops/test_tools.py"""
import importlib.machinery
import importlib.util
import json
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))


def load(name):
    loader = importlib.machinery.SourceFileLoader(name.replace("-", "_"), os.path.join(HERE, name))
    spec = importlib.util.spec_from_loader(loader.name, loader)
    mod = importlib.util.module_from_spec(spec)
    loader.exec_module(mod)
    return mod


reconcile = load("reconcile")
scan_logs = load("scan-logs")

TEXT = 'time=2026-11-04T10:00:00Z level=INFO msg="ops request" log=ops_access endpoint=connections kid=k1 operator=ops:a1 requestId={rid} count={n} durationMs=1 peer=10.0.0.1 result={res}\n'
JSON_LINE = '{{"time":"x","level":"INFO","msg":"ops request","endpoint":"disconnects","requestId":"{rid}","count":{n},"result":"ok"}}\n'


class ReconcileTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)

    def write(self, name, text):
        path = os.path.join(self.dir.name, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def run_main(self, audit, *logs):
        return reconcile.main(["--audit", self.write("audit.jsonl", "".join(json.dumps(a) + "\n" for a in audit))] + list(logs))

    def test_clean(self):
        # 一次排查查了两个节点：条数相加；节点探测和失败请求不算用户级请求。
        n1 = self.write("n1.log", TEXT.format(rid="r1", n=1, res="ok") + JSON_LINE.format(rid="r1", n=2))
        n2 = self.write("n2.log", TEXT.format(rid="r1", n=1, res="ok") + TEXT.format(rid="bad", n=0, res="unauthorized")
                        + 'msg="ops request" endpoint=node requestId=probe count=1 result=ok\nnot a log line\n')
        self.assertEqual(self.run_main([{"action": "ops.troubleshoot.query", "requestId": "r1", "count": 4}, {"action": "ops.config.view", "requestId": "c1"}], n1, n2), 0)

    def test_audit_without_log(self):
        self.assertEqual(self.run_main([{"action": "ops.troubleshoot.query", "requestId": "r9", "count": 1}], self.write("n.log", "")), 1)

    def test_log_without_audit(self):
        self.assertEqual(self.run_main([], self.write("n.log", TEXT.format(rid="r2", n=1, res="ok"))), 1)

    def test_count_mismatch(self):
        self.assertEqual(self.run_main([{"action": "ops.troubleshoot.query", "requestId": "r3", "count": 5}], self.write("n.log", TEXT.format(rid="r3", n=1, res="ok"))), 1)


class ScanTest(unittest.TestCase):
    def test_hits(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "a.log")
            with open(path, "w", encoding="utf-8") as f:
                f.write("clean\nhas u-canary-7f3a here\n")
            self.assertEqual(scan_logs.main(["--marker", "u-canary-7f3a", path]), 1)
            self.assertEqual(scan_logs.main(["--marker", "nothing", path]), 0)

    def test_hash(self):
        import hashlib
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "a.log")
            with open(path, "w", encoding="utf-8") as f:
                f.write("digest " + hashlib.sha256(b"SECRET").hexdigest()[:16] + "\n")
            self.assertEqual(scan_logs.main(["--marker", "SECRET", path]), 0)
            self.assertEqual(scan_logs.main(["--marker", "SECRET", "--with-hash", path]), 1)


if __name__ == "__main__":
    unittest.main()
