"""BE-438：对象存储配置样例的静态检查（PRD-chat F-FILE-01 规则 3a、9，F-FILE-06 规则 8，第 11.3、21 节）。只用标准库。

python3 -I -m unittest discover -s deploy/objstore -p 'test_*.py'
这些检查只证明样例本身符合 PRD，不证明厂商接受或生效（AC-FILE-90-1、90-2）。
"""
import json
import pathlib
import re
import unittest

HERE = pathlib.Path(__file__).resolve().parent
CONFIG_GO = HERE.parent.parent / "server/internal/config/config.go"
PRIVATE, PUBLIC = "tianting-chat-private", "tianting-chat-public"


def load(name):
    return json.loads((HERE / name).read_text())


class ServerPolicy(unittest.TestCase):
    """第 11.3 节：临时前缀读写删；最终前缀读写删和按前缀列举；公开桶写删。全桶 List 不给 im-server。"""

    def setUp(self):
        self.st = load("policy-im-server.json")["Statement"]

    def test_only_allow(self):
        for s in self.st:
            self.assertEqual(s["Effect"], "Allow")
            self.assertNotIn("Principal", s)
            for a in s["Action"]:
                self.assertNotIn("*", a, s["Sid"])

    def test_list_is_final_prefix_only(self):
        lists = [s for s in self.st if "s3:ListBucket" in s["Action"]]
        self.assertEqual(len(lists), 1)
        self.assertEqual(lists[0]["Resource"], [f"arn:aws:s3:::{PRIVATE}"])
        self.assertEqual(lists[0]["Condition"], {"StringLike": {"s3:prefix": ["final/*"]}})

    def test_object_resources(self):
        resources = set()
        for s in self.st:
            if "s3:ListBucket" not in s["Action"]:
                resources.update(s["Resource"])
        self.assertEqual(resources, {f"arn:aws:s3:::{PRIVATE}/tmp/*", f"arn:aws:s3:::{PRIVATE}/final/*", f"arn:aws:s3:::{PUBLIC}/avatars/*"})
        public = [s for s in self.st if any(PUBLIC in r for r in s["Resource"])]
        self.assertEqual(sorted(public[0]["Action"]), ["s3:DeleteObject", "s3:PutObject"])  # 公开桶不需要读、不能列举

    def test_no_bucket_admin_actions(self):
        for s in self.st:
            for a in s["Action"]:
                self.assertNotRegex(a, r"Policy|Lifecycle|Cors|Versioning|Acl|Encryption", s["Sid"])


class PublicPolicy(unittest.TestCase):
    def test_anonymous_get_only(self):
        st = load("policy-public-read.json")["Statement"]
        self.assertEqual(len(st), 1)
        self.assertEqual(st[0]["Action"], ["s3:GetObject"])  # 没有 s3:ListBucket：匿名不能列举（规则 9）
        self.assertEqual(st[0]["Resource"], [f"arn:aws:s3:::{PUBLIC}/avatars/*"])


class Lifecycle(unittest.TestCase):
    def test_tmp_one_day(self):
        rules = load("lifecycle-private.json")["Rules"]
        tmp = [r for r in rules if r["Filter"].get("Prefix") == "tmp/"]
        self.assertEqual(len(tmp), 1)
        self.assertEqual(tmp[0]["Status"], "Enabled")
        self.assertEqual(tmp[0]["Expiration"], {"Days": 1})

    def test_nothing_else_expires(self):
        # final/ 由 file_purge 按保留期删除，生命周期不能碰它；没有版本相关规则（F-FILE-06 规则 8：桶不开版本控制）。
        for r in load("lifecycle-private.json")["Rules"]:
            self.assertEqual(r["Filter"], {"Prefix": "tmp/"})
            self.assertNotIn("NoncurrentVersionExpiration", r)


class Cors(unittest.TestCase):
    def test_origins_limited(self):
        for r in load("cors-private.json")["CORSRules"]:
            for o in r["AllowedOrigins"]:
                self.assertNotIn("*", o)
                self.assertTrue(o.startswith("https://"), o)
            self.assertTrue(set(r["AllowedMethods"]) <= {"PUT", "GET", "HEAD"})
            self.assertNotIn("*", r["AllowedHeaders"])


class ExampleEnv(unittest.TestCase):
    def setUp(self):
        self.vars = {}
        for line in (HERE / "im-files.example.env").read_text().splitlines():
            line = line.strip()
            if line and not line.startswith("#"):
                k, _, v = line.partition("=")
                self.vars[k] = v

    def test_no_secrets(self):
        self.assertEqual(self.vars["IM_OBJSTORE_ACCESS_KEY"], "")
        self.assertEqual(self.vars["IM_OBJSTORE_SECRET_KEY"], "")
        for path in HERE.iterdir():
            if path.suffix in (".json", ".env"):
                text = path.read_text()
                self.assertNotRegex(text, r"AKIA[0-9A-Z]{16}", path.name)
                self.assertNotRegex(text, r"[A-Za-z0-9/+]{40}", path.name)

    def test_https_and_buckets(self):
        self.assertTrue(self.vars["IM_OBJSTORE_ENDPOINT"].startswith("https://"))
        self.assertTrue(self.vars["IM_OBJSTORE_PUBLIC_BASE_URL"].startswith("https://"))
        self.assertEqual(self.vars["IM_OBJSTORE_BUCKET"], PRIVATE)
        self.assertEqual(self.vars["IM_OBJSTORE_PUBLIC_BUCKET"], PUBLIC)

    def test_names_exist_in_server(self):
        known = set(re.findall(r'"(IM_[A-Z0-9_]+)"', CONFIG_GO.read_text()))
        for k in self.vars:
            self.assertIn(k, known, k)


if __name__ == "__main__":
    unittest.main()
