#!/usr/bin/env python3
"""部署配置的测试。不需要 Prometheus、Grafana、MinIO：age 和 mc 用假的，真实工具在 CI 的 ops-deploy 任务里跑。

python3 deploy/test_deploy.py（需要 pyyaml、go、jq、openssl）
"""
import importlib.util
import json
import os
import pathlib
import re
import shutil
import stat
import subprocess
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request

import yaml

ROOT = pathlib.Path(__file__).resolve().parent
OBS = ROOT / "observability"
DASH = OBS / "grafana/dashboards/tianting.json"


def run(*cmd, env=None, check=True, cwd=None):
    e = {**os.environ, **(env or {})}
    r = subprocess.run([str(c) for c in cmd], capture_output=True, text=True, env=e, cwd=cwd)
    if check and r.returncode != 0:
        raise AssertionError(f"{cmd}: {r.returncode}\n{r.stdout}\n{r.stderr}")
    return r


def sha256(path):
    import hashlib
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()


class Targets(unittest.TestCase):
    def table(self, rows):
        d = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, d)
        p = pathlib.Path(d) / "nodes.json"
        p.write_text(json.dumps(rows))
        return p

    def test_generates_node_labels_without_key_refs(self):
        p = self.table([
            {"nodeId": "n1", "addr": "10.0.0.1:9091", "metricsAddr": "10.0.0.1:9090", "keyFile": "/secret/n1.key", "kid": "k1"},
            {"nodeId": "n2", "addr": "10.0.0.2:9091", "metricsAddr": "10.0.0.2:9090", "keyFile": "/secret/n2.key"},
        ])
        r = run(OBS / "gen-targets", p)
        self.assertEqual(json.loads(r.stdout), [
            {"targets": ["10.0.0.1:9090"], "labels": {"node_id": "n1"}},
            {"targets": ["10.0.0.2:9090"], "labels": {"node_id": "n2"}},
        ])
        self.assertNotIn("secret", r.stdout)

    def test_rejects_missing_and_duplicates(self):
        self.assertEqual(run(OBS / "gen-targets", self.table([{"nodeId": "n1"}]), check=False).returncode, 1)
        dup = [{"nodeId": "n1", "metricsAddr": "a:1"}, {"nodeId": "n1", "metricsAddr": "b:1"}]
        self.assertEqual(run(OBS / "gen-targets", self.table(dup), check=False).returncode, 1)


class AlertConfig(unittest.TestCase):
    def rules(self):
        return yaml.safe_load((OBS / "prometheus/alerts.yml").read_text())["groups"][0]["rules"]

    def test_eleven_alerts_plus_drain_helper(self):
        names = [r["alert"] for r in self.rules()]
        self.assertEqual(len(names), 12)
        for want in ["节点不可达", "无就绪节点", "5xxx错误率高", "命令耗时高", "在线连接骤降", "慢连接增多", "握手异常", "内存逼近上限", "疑似GC风暴", "备份过期", "版本不一致"]:
            self.assertIn(want, names)
        severities = {r["alert"]: r["labels"]["severity"] for r in self.rules()}
        for name in ["节点不可达", "无就绪节点", "5xxx错误率高", "疑似GC风暴", "备份过期"]:
            self.assertEqual(severities[name], "critical", name)
        for r in self.rules():
            self.assertNotIn("instance", str(r.get("annotations")), r["alert"])

    def test_every_alert_has_a_promtool_case(self):
        tests = yaml.safe_load((OBS / "prometheus/alerts_test.yml").read_text())["tests"]
        firing, silent = set(), set()
        for t in tests:
            for c in t["alert_rule_test"]:
                (firing if c["exp_alerts"] else silent).add(c["alertname"])
        for r in self.rules():
            if r["alert"] == "节点下线中":
                continue
            self.assertIn(r["alert"], firing, "没有触发用例")
            self.assertIn(r["alert"], silent, "没有不触发用例")

    def test_alertmanager_check_passes(self):
        r = run(OBS / "check-alertmanager")
        self.assertIn("office\twebhook", r.stdout)
        self.assertIn("office\temail", r.stdout)

    def bad_config(self, mutate, template=None):
        d = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, d)
        cfg = yaml.safe_load((OBS / "alertmanager/alertmanager.yml").read_text())
        mutate(cfg)
        os.makedirs(f"{d}/templates")
        src = (OBS / "alertmanager/templates/tianting.tmpl").read_text()
        pathlib.Path(f"{d}/templates/t.tmpl").write_text(template or src)
        p = pathlib.Path(d) / "am.yml"
        p.write_text(yaml.safe_dump(cfg))
        return run(OBS / "check-alertmanager", p, check=False)

    def test_alertmanager_check_catches_problems(self):
        def to_self(c):
            c["receivers"][1]["webhook_configs"][0] = {"url": "https://im-server.internal:8081/hook"}
        self.assertEqual(self.bad_config(to_self).returncode, 1)
        self.assertEqual(self.bad_config(lambda c: None, template="{{ .Labels.instance }}").returncode, 1)
        self.assertEqual(self.bad_config(lambda c: c["route"].update(repeat_interval="4h")).returncode, 1)
        self.assertEqual(self.bad_config(lambda c: None).returncode, 0)

    def test_notification_scan(self):
        d = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, d)
        p = pathlib.Path(d) / "n.txt"
        p.write_text("告警：备份过期\n开始时间：2026-11-04T12:34:56Z\n节点：n1")
        self.assertEqual(run(OBS / "check-alertmanager", "--scan", p, check=False).returncode, 0)
        for bad in ["10.0.0.1:9090", "host: 10.1.2.3", "eyJhbGciOi.eyJzdWIi.abc123", "userId=u1", "fe80:1:2:3::1"]:
            p.write_text("告警\n" + bad)
            self.assertEqual(run(OBS / "check-alertmanager", "--scan", p, check=False).returncode, 1, bad)


def exprs():
    rules = yaml.safe_load((OBS / "prometheus/alerts.yml").read_text())["groups"][0]["rules"]
    out = [r["expr"] for r in rules]
    for p in json.loads(DASH.read_text())["panels"]:
        out += [t["expr"] for t in p.get("targets", [])]
    return out


class Testenv(unittest.TestCase):
    """BE-320：编排文件和 tt 生成的配置。真实运行在 CI 的 testenv 工作流（verify.sh）。"""

    HERE = ROOT / "testenv"

    def compose(self, name="compose.yml"):
        return yaml.safe_load((self.HERE / name).read_text())

    def test_images_are_pinned_and_ports_are_loopback(self):
        for name in ["compose.yml", "compose.e2a.yml"]:
            for svc, c in self.compose(name)["services"].items():
                if "image" in c and "build" not in c:
                    self.assertRegex(c["image"], r":[\w.-]*\d", svc)
                for port in c.get("ports", []):
                    self.assertTrue(port.startswith("127.0.0.1:"), f"{svc}: {port}")

    def test_mounts_the_production_config_unchanged(self):
        svcs = self.compose()["services"]
        def mounts(name):
            return svcs[name]["volumes"]
        self.assertIn("../observability/prometheus/alerts.yml:/etc/prometheus/alerts.yml:ro", mounts("prometheus"))
        self.assertIn("../observability/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro", mounts("prometheus"))
        self.assertIn("../observability/alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro", mounts("alertmanager"))
        self.assertIn("../observability/grafana/grafana.ini:/etc/grafana/grafana.ini:ro", mounts("grafana"))
        for name in ["prometheus", "alertmanager", "grafana", "node-exporter", "console"]:
            self.assertEqual(svcs[name]["network_mode"], "service:netns", name)

    def test_console_is_optional(self):
        svcs = self.compose()["services"]
        for name in ["console", "console-migrate", "postgres"]:
            self.assertEqual(svcs[name]["profiles"], ["console"], name)

    def test_no_secret_in_the_repo_files(self):
        for name in ["compose.yml", "compose.e2a.yml"]:
            text = (self.HERE / name).read_text()
            self.assertNotRegex(text, r"(?i)(password|secret|key)\s*[:=]\s*\S")

    def test_generated_config(self):
        with tempfile.TemporaryDirectory() as d:
            env = {**os.environ, "TT_RUN_DIR": d + "/run"}
            run(str(self.HERE / "tt"), "config", "e2a", "--console", env=env)
            r = pathlib.Path(d) / "run"
            parse = lambda f: dict(l.split("=", 1) for l in (r / f).read_text().splitlines())
            # 两个节点同一份 JWT 密钥（都读 im.env），各自的运维公钥。
            self.assertEqual(len(parse("im.env")["IM_JWT_SECRET"]), 64)
            self.assertEqual(len(parse("im.env")["IM_QUARANTINE_KEY"]), 64)
            self.assertNotEqual(parse("im.env")["IM_QUARANTINE_KEY"], parse("im.env")["IM_JWT_SECRET"])
            n1, n2 = parse("n1.env"), parse("n2.env")
            self.assertEqual((n1["IM_NODE_ID"], n2["IM_NODE_ID"]), ("n1", "n2"))
            self.assertNotEqual(n1["IM_OPS_PUBKEYS"], n2["IM_OPS_PUBKEYS"])
            # 密钥文件别人读不了。
            for f in ["secrets", "im.env", "console.env", "minio.env", "postgres.env", "nodes/n1.key", "nodes/n2.key"]:
                self.assertEqual(stat.S_IMODE((r / f).stat().st_mode), 0o600, f)
            self.assertEqual(stat.S_IMODE(r.stat().st_mode), 0o700)
            # 节点表和抓取清单同源。
            rows = json.loads((r / "nodes/nodes.json").read_text())
            self.assertEqual([x["nodeId"] for x in rows], ["n1", "n2"])
            # 非回环地址必须走 TLS，服务名和证书一致；节点拿不到 CA 私钥和别的节点的私钥。
            self.assertEqual([x["tlsServerName"] for x in rows], ["n1.ops.internal", "n2.ops.internal"])
            self.assertEqual(n1["IM_OPS_TLS_KEY"], "/run/tianting/tls/n1.key")
            self.assertEqual(sorted(p.name for p in (r / "tls/n1").iterdir()), ["n1.key", "n1.pem"])
            self.assertEqual(stat.S_IMODE((r / "tls/n1/n1.key").stat().st_mode), 0o600)
            self.assertIn("CON_OPS_CA_FILE", parse("console.env"))
            self.assertFalse((r / "nodes/ca.key").exists())
            self.assertIn("-----BEGIN CERTIFICATE-----", (r / "nodes/ca.pem").read_text())
            out = run("openssl", "x509", "-in", r / "tls/n2/n2.pem", "-noout", "-ext", "subjectAltName").stdout
            self.assertIn("DNS:n2.ops.internal", out)
            targets = json.loads((r / "targets/im.json").read_text())
            self.assertEqual([t["labels"]["node_id"] for t in targets], ["n1", "n2"])
            # 再跑一次（e1 升 e2a 的路径）：密钥不换。
            before = (r / "secrets").read_text(), (r / "n1.env").read_text()
            run(str(self.HERE / "tt"), "config", "e2a", env=env)
            self.assertEqual(before, ((r / "secrets").read_text(), (r / "n1.env").read_text()))
            # backup-age 写的是告警读的那个指标。
            run(str(self.HERE / "tt"), "backup-age", "im", "7260", env=env)
            line = (r / "textfile/im_backup_im.prom").read_text()
            self.assertRegex(line, r'^im_backup_last_success_timestamp_seconds\{source="im",trigger="schedule"\} \d+\n$')
            self.assertAlmostEqual(int(line.split()[-1]), time.time() - 7260, delta=30)

    def test_webhook_records_each_notification(self):
        spec = importlib.util.spec_from_file_location("tt_webhook", self.HERE / "webhook.py")
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        with tempfile.TemporaryDirectory() as d:
            log = d + "/n.jsonl"
            srv = mod.serve(0, log)
            threading.Thread(target=srv.serve_forever, daemon=True).start()
            base = f"http://127.0.0.1:{srv.server_address[1]}"
            def post(body):
                return urllib.request.urlopen(urllib.request.Request(base + "/notify", data=body, method="POST"))
            for i in (1, 2):
                post(json.dumps({"status": "firing", "alerts": [{"labels": {"alertname": "节点不可达"}, "n": i}]}).encode())
            with self.assertRaises(urllib.error.HTTPError) as bad:
                post(b"not json")
            self.assertEqual(bad.exception.code, 400)
            rows = json.load(urllib.request.urlopen(base + "/notifications"))
            self.assertEqual([r["payload"]["alerts"][0]["n"] for r in rows], [1, 2])
            self.assertGreater(rows[0]["receivedAtMs"], 0)
            srv.shutdown()
            srv.server_close()
            # 重启不丢。
            self.assertEqual(len(mod.Store(log).all()), 2)


class NotificationContent(unittest.TestCase):
    """AC-OPS-07-6 和恢复通知时限：webhook 收到的是原始标签，模板管不到。"""

    def test_instance_is_dropped_before_alertmanager(self):
        cfg = yaml.safe_load((OBS / "prometheus/prometheus.yml").read_text())
        self.assertIn({"action": "labeldrop", "regex": "instance"}, cfg["alerting"]["alert_relabel_configs"])

    def test_every_alert_has_node_id(self):
        rules = yaml.safe_load((OBS / "prometheus/alerts.yml").read_text())["groups"][0]["rules"]
        for r in rules:
            expr = r["expr"]
            grouped_away = re.search(r"\b(sum|count|max)\b(?! by \(node_id\))", expr) or "absent" in expr
            if grouped_away and "by (node_id)" not in expr:
                self.assertEqual(r["labels"].get("node_id"), "all", r["alert"])

    def test_resolved_notification_is_not_held_by_group_interval(self):
        route = yaml.safe_load((OBS / "alertmanager/alertmanager.yml").read_text())["route"]
        self.assertEqual(route["group_interval"], "1m")
        self.assertEqual(route["repeat_interval"], "1h")


class Dashboards(unittest.TestCase):
    def setUp(self):
        self.d = json.loads(DASH.read_text())

    def test_generated_file_is_current(self):
        run(OBS / "grafana/gen-dashboards", "--check")

    def test_fourteen_panels_read_only(self):
        self.assertEqual(len(self.d["panels"]), 14)
        self.assertFalse(self.d["editable"])
        p = yaml.safe_load((OBS / "grafana/provisioning/dashboards/tianting.yml").read_text())["providers"][0]
        self.assertFalse(p["allowUiUpdates"])
        self.assertTrue(p["disableDeletion"])
        for panel in self.d["panels"]:
            if panel["type"] != "text":
                self.assertTrue(panel.get("description"), panel["title"])
                for t in panel["targets"]:
                    if "vector(" not in t["expr"]:
                        self.assertIn('node_id=~"$node_id"', t["expr"], panel["title"])

    def test_special_panels(self):
        by = {p["title"]: p for p in self.d["panels"]}
        p99 = by["命令耗时 P50 / P99"]["fieldConfig"]["defaults"]["mappings"][0]
        self.assertEqual(p99["options"]["from"], 2.5)
        self.assertEqual(p99["options"]["result"]["text"], "> 2.5s")
        mem = " ".join(t["expr"] + t["legendFormat"] for t in by["内存"]["targets"])
        self.assertIn("< 2^62", mem)
        self.assertIn("未设置 GOMEMLIMIT", mem)
        self.assertIn(">= 1000", by["每连接存活内存"]["targets"][0]["expr"])
        self.assertIn("30 天", by["数据保留说明"]["options"]["content"])

    def test_normalize_is_stable(self):
        a = run("jq", "-S", "-f", OBS / "grafana/normalize.jq", DASH).stdout
        exported = dict(self.d, id=7, version=3, iteration=123)
        exported["panels"] = [dict(p, id=i) for i, p in enumerate(self.d["panels"])]
        d = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, d)
        f = pathlib.Path(d) / "e.json"
        f.write_text(json.dumps({"dashboard": exported, "meta": {"x": 1}}))
        self.assertEqual(run("jq", "-S", "-f", OBS / "grafana/normalize.jq", f).stdout, a)


class AdminCallersExample(unittest.TestCase):
    """im-admin-callers.example.json：PRD-admin F-KEY-02 规则 4；持有 push.all 必须配 pushTypes（PRD-live F-PUSH-09 规则 3）。"""

    FILE = ROOT / "im-admin-callers.example.json"

    def callers(self):
        return json.loads(self.FILE.read_text())

    def test_console_scopes_follow_the_prd(self):
        by_id = {c["id"]: c for c in self.callers()}
        console = set(by_id["console"]["scopes"])
        self.assertEqual(console, {
            "user.read", "user.write", "user.kick", "user.sessions", "group.read", "group.write", "group.dissolve",
            "social.settings", "social.friends", "msg.meta", "audit.read", "stats.read", "user.ban", "msg.delete", "mute.write", "push.user", "push.all", "account.invite",
        })
        self.assertEqual(set(by_id["console-msgread"]["scopes"]), {"msg.body.group", "msg.quarantine"})
        self.assertTrue(by_id["console-msgread"]["bodyAccess"])

    def test_push_all_has_push_types_and_no_key_is_committed(self):
        for c in self.callers():
            if "push.all" in c["scopes"]:
                self.assertTrue(c.get("pushTypes"), c["id"])
            self.assertTrue(c["publicKey"].startswith("CHANGE_ME"), c["id"])
            self.assertNotIn("secret", c)

    def test_push_types_match_console_templates(self):
        console_dir = pathlib.Path(os.environ.get("TIANTING_CONSOLE_DIR", ROOT.parent.parent / "tianting-console-server"))
        templates = console_dir / "deploy/push-templates.example.json"
        if not templates.exists():
            self.skipTest("tianting-console-server is not available")
        want = set(json.loads(templates.read_text()))
        for c in self.callers():
            if "push.all" in c["scopes"]:
                self.assertEqual(set(c["pushTypes"]), want, c["id"])

    def test_real_server_starts_with_it_and_refuses_without_push_types(self):
        with tempfile.TemporaryDirectory() as d:
            run("go", "build", "-o", f"{d}/im", "./cmd/im-server", cwd=ROOT.parent / "server")
            callers = self.callers()
            for c in callers:
                pair = run("go", "run", "genkey.go", cwd=ROOT / "testenv").stdout.split()
                c["publicKey"] = pair[1]
                c.pop("allowedCIDRs", None)
            path = pathlib.Path(d) / "callers.json"

            def start(items):
                path.write_text(json.dumps(items))
                path.chmod(0o600)
                db = pathlib.Path(d) / "a.db"
                db.unlink(missing_ok=True)
                env = {**os.environ, "IM_DB": str(db), "IM_ADDR": "127.0.0.1:18721", "IM_ADMIN_ADDR": "127.0.0.1:18722", "IM_METRICS_ADDR": "127.0.0.1:18723", "IM_PPROF_ADDR": "off", "IM_OPS_ADDR": "off",
                       "IM_ADMIN_CALLERS": str(path), "IM_PUSH_ENABLED": "1", "IM_V13_ADMIN_FEATURES": "1"}
                return subprocess.Popen([f"{d}/im"], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)

            p = start(callers)
            try:
                for _ in range(100):
                    try:
                        urllib.request.urlopen("http://127.0.0.1:18723/metrics").read()
                        break
                    except OSError:
                        self.assertIsNone(p.poll(), "im-server exited with the example config")
                        time.sleep(0.1)
                else:
                    raise AssertionError("im-server did not start")
            finally:
                p.kill()
                p.communicate()
            del callers[0]["pushTypes"]
            p = start(callers)
            out, _ = p.communicate(timeout=30)
            self.assertNotEqual(p.returncode, 0)
            self.assertIn("console", out)
            self.assertIn("pushTypes", out)
            self.assertNotIn(callers[0]["publicKey"], out)


class MetricsExist(unittest.TestCase):
    """告警和面板用到的指标必须真实存在：写错名字时 PromQL 不报错，只是永远没有数据。"""

    def known(self):
        src = "".join(p.read_text() for p in (ROOT.parent / "server/internal/metrics").glob("*.go"))
        names = set(re.findall(r'Name:\s*"([a-z_0-9]+)"', src))
        with tempfile.TemporaryDirectory() as d:
            run("go", "build", "-o", f"{d}/im", "./cmd/im-server", cwd=ROOT.parent / "server")
            env = {**os.environ, "IM_DB": f"{d}/a.db", "IM_ADDR": "127.0.0.1:18711", "IM_ADMIN_ADDR": "127.0.0.1:18712", "IM_METRICS_ADDR": "127.0.0.1:18713", "IM_PPROF_ADDR": "off", "IM_OPS_ADDR": "off"}
            p = subprocess.Popen([f"{d}/im"], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            try:
                for _ in range(100):
                    try:
                        body = urllib.request.urlopen("http://127.0.0.1:18713/metrics").read().decode()
                        break
                    except OSError:
                        time.sleep(0.1)
                else:
                    raise AssertionError("im-server did not start")
            finally:
                p.kill()
                p.wait()
        for line in body.splitlines():
            m = re.match(r"# TYPE (\S+) ", line)
            if m:
                names.add(m.group(1))
        return names

    def test_all_metrics_exist(self):
        known = self.known()
        derived = set()
        for n in known:
            derived |= {n + "_bucket", n + "_count", n + "_sum"}
        external = {"up", "im_backup_last_success_timestamp_seconds"}
        promql = {"sum", "rate", "increase", "by", "or", "vector", "and", "unless", "offset", "count", "max", "time", "absent", "changes", "histogram_quantile", "bool", "m"}
        used = set()
        for e in exprs():
            clean = re.sub(r'\{[^}]*\}', "", e)
            clean = re.sub(r'\[[^\]]*\]', "", clean)
            clean = re.sub(r'\bby \([^)]*\)', "", clean)
            clean = re.sub(r'offset \d+m', "", clean)
            used |= {w for w in re.findall(r"[a-zA-Z_][a-zA-Z0-9_]*", clean) if w not in promql}
        self.assertEqual(used - known - derived - external, set())
        self.assertGreater(len(used), 15)

    def test_label_values_match_the_server(self):
        src = "".join(p.read_text() for p in (ROOT.parent / "server/internal/metrics").glob("*.go"))
        self.assertIn('"token_invalid"', src)
        self.assertIn('"phase"', src)


FAKE_AGE = """#!/usr/bin/env bash
# 假 age：把文件前面加一行标记。age 本身不测这里，CI 用真的。
[ -n "$FAKE_AGE_FAIL" ] && exit 1
while [ $# -gt 0 ]; do case "$1" in -r) shift 2;; -o) out="$2"; shift 2;; *) in="$1"; shift;; esac; done
{ echo age-encrypted; cat "$in"; } > "$out"
"""
FAKE_MC = """#!/usr/bin/env bash
# 假 mc：alias 是本地目录。
cmd="$1"; shift
while [ "$1" = "--quiet" ]; do shift; done
case "$cmd" in
  cp)
    case "$2" in */backups/*) [ -n "$FAKE_UPLOAD_FAIL" ] && exit 1;; esac
    mkdir -p "$(dirname "$2")"; cp "$1" "$2";;
  ls) [ -d "$1" ] && ls -1 "$1";;
  rm) rm -f "${1:?}";;
esac
"""


class Backup(unittest.TestCase):
    def setUp(self):
        self.dir = pathlib.Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)
        bin_dir = self.dir / "bin"
        bin_dir.mkdir()
        for name, body in (("age", FAKE_AGE), ("mc", FAKE_MC)):
            p = bin_dir / name
            p.write_text(body)
            p.chmod(p.stat().st_mode | stat.S_IEXEC)
        self.im = self.dir / "im-server"
        run("go", "build", "-o", self.im, "./cmd/im-server", cwd=ROOT.parent / "server")
        self.db = self.dir / "im.db"
        run(self.im, "migrate", "up", env={"IM_DB": str(self.db)})
        self.remote = self.dir / "remote"
        self.text = self.dir / "textfile"
        self.text.mkdir()
        self.env = {"PATH": f"{bin_dir}:{os.environ['PATH']}", "AGE_RECIPIENT": "age1test", "BACKUP_REMOTE": str(self.remote), "TEXTFILE_DIR": str(self.text),
                    "BACKUP_WORK_DIR": str(self.dir / "work"), "IM_SERVER_BIN": str(self.im), "IM_DB": str(self.db)}

    def backup(self, *args, env=None, check=True):
        return run(ROOT / "backup/backup.sh", *args, env={**self.env, **(env or {})}, check=check)

    def manifests(self):
        return [json.loads(p.read_text()) for p in sorted((self.remote / "manifests/im").glob("*.json"))]

    def metrics(self):
        return (self.text / "im_backup_im.prom").read_text()

    def test_success(self):
        self.backup()
        m = self.manifests()[0]
        self.assertEqual((m["source"], m["trigger"], m["result"]), ("im", "schedule", "success"))
        self.assertTrue(m["verified"] and m["encrypted"] and m["uploaded"])
        self.assertGreater(m["migrationVersion"], 0)
        obj = self.remote / "backups/im" / m["file"]
        self.assertEqual(m["size"], obj.stat().st_size)
        self.assertEqual(m["sha256"], sha256(obj))
        self.assertNotIn("/", m["file"])
        self.assertTrue(obj.read_bytes().startswith(b"age-encrypted"))
        # 明文快照没有留下。
        self.assertEqual(list((self.dir / "work").iterdir()), [])
        # 指标和清单结束时间相差 ≤ 1 秒（AC-OPS-12-7）。
        v = re.search(r'im_backup_last_success_timestamp_seconds\{source="im",trigger="schedule"\} (\S+)', self.metrics()).group(1)
        self.assertLessEqual(abs(float(v) - m["finishedAt"] / 1000), 1)
        self.assertIn(f'im_backup_last_size_bytes{{source="im"}} {m["size"]}', self.metrics())
        # 对象键是 console-server 约定的格式。
        self.assertRegex(sorted((self.remote / "manifests/im").iterdir())[0].name, r"^\d{13}\.json$")

    def assert_failure(self, reason, env):
        self.backup()
        time.sleep(0.01)
        before = re.search(r'trigger="schedule"\} (\S+)', self.metrics()).group(1)
        r = self.backup(env=env, check=False)
        self.assertEqual(r.returncode, 1)
        m = self.manifests()[-1]
        self.assertEqual((m["result"], m["error"]), ("failure", reason))
        for secret in [str(self.dir), "age1test"]:
            self.assertNotIn(secret, json.dumps(m))
        self.assertEqual(re.search(r'trigger="schedule"\} (\S+)', self.metrics()).group(1), before)
        self.assertIn("last_attempt", self.metrics())
        self.assertEqual(list((self.dir / "work").iterdir()), [])

    def test_encrypt_failure(self):
        self.assert_failure("加密失败", {"FAKE_AGE_FAIL": "1"})

    def test_empty_recipient(self):
        self.assert_failure("加密失败", {"AGE_RECIPIENT": ""})

    def test_upload_failure(self):
        self.assert_failure("上传失败", {"FAKE_UPLOAD_FAIL": "1"})

    def test_snapshot_failure(self):
        self.assert_failure("快照或校验失败", {"IM_DB": str(self.dir / "missing.db")})

    def test_trigger_labels_are_kept_apart(self):
        self.backup()
        time.sleep(0.01)
        self.backup("--trigger", "pre-migrate", "--release", "v1.2.3")
        self.assertIn('trigger="schedule"', self.metrics())
        self.assertIn('trigger="pre-migrate"', self.metrics())
        m = self.manifests()[-1]
        self.assertEqual((m["trigger"], m["release"]), ("pre-migrate", "v1.2.3"))
        self.assertEqual(self.backup("--trigger", "nope", check=False).returncode, 2)

    def test_prune_keeps_latest(self):
        for _ in range(3):
            self.backup(env={"BACKUP_RETAIN": "2"})
            time.sleep(1.05)  # 文件名精确到秒
        self.assertEqual(len(list((self.remote / "backups/im").iterdir())), 2)

    def test_policies(self):
        reader = json.loads((ROOT / "backup/policy-console-reader.json").read_text())
        actions = [a for s in reader["Statement"] for a in s["Action"]]
        self.assertEqual(sorted(actions), ["s3:GetObject", "s3:ListBucket"])
        self.assertNotIn("backups/", json.dumps(reader).replace("tianting-backups/", ""))
        writer = json.loads((ROOT / "backup/policy-backup-writer.json").read_text())
        self.assertNotIn("s3:GetObject", json.dumps(writer))

    # ---- 发布（AC-OPS-13-1、13-2、13-5）----
    def release_env(self):
        cur = self.dir / "current"
        shutil.copy(self.im, cur)
        return cur, {**self.env, "IM_BIN": str(cur), "RESTART_CMD": f"touch {self.dir}/restarted"}

    def fake_new(self, pending, up_log):
        """假的新二进制：migrate status 报告有没有待执行迁移，migrate up 记时间。"""
        p = self.dir / "new"
        status = "  0009_new  pending" if pending else "  0008_x  applied  checksum ok"
        p.write_text(f"""#!/usr/bin/env bash
case "$1 $2" in
  "migrate status") echo "{status}";;
  "migrate up") python3 -c 'import time; print(int(time.time()*1000))' > {up_log};;
esac
""")
        p.chmod(0o755)
        return p

    def test_release_with_pending_backs_up_first(self):
        cur, env = self.release_env()
        log = self.dir / "up.log"
        new = self.fake_new(True, log)
        run(ROOT / "release.sh", new, "v2", env=env)
        m = self.manifests()[-1]
        self.assertEqual((m["trigger"], m["release"], m["result"]), ("pre-migrate", "v2", "success"))
        self.assertTrue(log.exists())
        self.assertLessEqual(m["finishedAt"], int(log.read_text()))
        self.assertEqual(cur.read_bytes(), new.read_bytes())
        self.assertTrue((self.dir / "restarted").exists())

    def test_release_stops_when_backup_fails(self):
        cur, env = self.release_env()
        before = cur.read_bytes()
        log = self.dir / "up.log"
        new = self.fake_new(True, log)
        r = run(ROOT / "release.sh", new, "v2", env={**env, "FAKE_UPLOAD_FAIL": "1"}, check=False)
        self.assertNotEqual(r.returncode, 0)
        self.assertFalse(log.exists(), "迁移不能执行")
        self.assertEqual(cur.read_bytes(), before, "不能切换二进制")
        self.assertFalse((self.dir / "restarted").exists())

    def test_release_without_pending_skips_backup(self):
        cur, env = self.release_env()
        r = run(ROOT / "release.sh", self.fake_new(False, self.dir / "up.log"), "v2", env=env)
        self.assertIn("无待执行迁移，跳过发布前备份", r.stdout)
        self.assertFalse((self.remote / "manifests").exists())
        self.assertFalse((self.dir / "up.log").exists())
        self.assertTrue((self.dir / "restarted").exists())

    def test_real_binary_status_format_is_understood(self):
        # release.sh 靠 "pending" 判断：真实二进制的输出里已执行的迁移不能含这个词。
        out = run(self.im, "migrate", "status", env={"IM_DB": str(self.db)}).stdout
        self.assertIn("applied", out)
        self.assertNotIn(" pending", out)


class Pki(unittest.TestCase):
    def setUp(self):
        self.dir = pathlib.Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)
        self.sh = ROOT / "pki/issue-node-cert.sh"

    def test_ca_and_node_certs(self):
        run(self.sh, "ca", self.dir)
        for n in ("n1", "n2", "n9"):
            run(self.sh, "node", self.dir, n)
        self.assertEqual(stat.S_IMODE((self.dir / "ca.key").stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE((self.dir / "n1.key").stat().st_mode), 0o600)
        self.assertIn("OK", run("openssl", "verify", "-CAfile", self.dir / "ca.pem", self.dir / "n1.pem").stdout)
        self.assertIn("DNS:n1.ops.internal", run("openssl", "x509", "-in", self.dir / "n1.pem", "-noout", "-ext", "subjectAltName").stdout)
        # n9 的证书冒充不了 n2（AC-OPS-16-5 的前提）。
        self.assertNotEqual(run("openssl", "verify", "-CAfile", self.dir / "ca.pem", "-verify_hostname", "n2.ops.internal", self.dir / "n9.pem", check=False).returncode, 0)
        self.assertEqual(run("openssl", "verify", "-CAfile", self.dir / "ca.pem", "-verify_hostname", "n9.ops.internal", self.dir / "n9.pem", check=False).returncode, 0)

    def test_refuses_to_overwrite_ca(self):
        run(self.sh, "ca", self.dir)
        self.assertNotEqual(run(self.sh, "ca", self.dir, check=False).returncode, 0)
        self.assertNotEqual(run(self.sh, "node", self.dir, "../evil", check=False).returncode, 0)


if __name__ == "__main__":
    unittest.main()
