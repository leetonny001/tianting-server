#!/usr/bin/env python3
"""生成追溯矩阵：PRD 的每条 AC → 后端任务 / 前端任务 / 测试用例。

用法：python3 tools/trace.py            # 写入 docs/testcases/traceability.md 并打印缺口
      python3 tools/trace.py --check    # 只检查，有缺口时退出码为 1（可用于 CI）

规则：
- 带【后端】的 AC 必须有 BE 任务；带【前端】的 AC 必须有 FE 任务；
  未标注的（端到端）AC 至少要有 BE 或 FE 任务之一；手工 AC 至少要有 FE 任务。
- 每条 AC 至少要有 1 条测试用例。
"""
import re
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PRD = ROOT / "docs/prd/PRD-core-v1.1.md"
TASKS = {"BE": ROOT / "docs/tasks/backend.md", "FE": ROOT / "docs/tasks/frontend.md"}
TC_DIR = ROOT / "docs/testcases"
OUT = TC_DIR / "traceability.md"

AC_RE = re.compile(r"AC-[A-Z]+-\d+-\d+")


def load_acs():
    acs = []
    for line in PRD.read_text(encoding="utf-8").splitlines():
        m = re.match(r"^- (AC-[A-Z]+-\d+-\d+)(.*)", line)
        if not m:
            continue
        rest = m.group(2)
        layer = "后端" if "【后端】" in rest[:8] else "前端" if "【前端】" in rest[:8] else "手工" if "（手工）" in rest[:8] else "端到端"
        acs.append((m.group(1), layer))
    return acs


def load_tasks(kind, path):
    """返回 {AC: [任务编号]}；只读每个任务的 `- AC：` 行。"""
    mapping = defaultdict(list)
    current = None
    for line in path.read_text(encoding="utf-8").splitlines():
        h = re.match(rf"^### ({kind}-\d+)", line)
        if h:
            current = h.group(1)
            continue
        if current and line.startswith("- AC："):
            for ac in AC_RE.findall(line):
                mapping[ac].append(current)
    return mapping


def load_testcases():
    """返回 {AC: [TC 编号]}；解析 `### TC-...` 标题后的第一行 `- 关联 AC：`。"""
    mapping = defaultdict(list)
    for f in sorted(TC_DIR.glob("TC-*.md")):
        current = None
        for line in f.read_text(encoding="utf-8").splitlines():
            h = re.match(r"^### (TC-[A-Z]+-\d+-\d+)", line)
            if h:
                current = h.group(1)
                continue
            if current and line.lstrip("- ").startswith("关联 AC"):
                for ac in AC_RE.findall(line):
                    mapping[ac].append(current)
                current = None
    return mapping


def main():
    acs = load_acs()
    be = load_tasks("BE", TASKS["BE"])
    fe = load_tasks("FE", TASKS["FE"])
    tc = load_testcases()
    known = {a for a, _ in acs}

    gaps = []
    rows = []
    for ac, layer in acs:
        b, f, t = be.get(ac, []), fe.get(ac, []), tc.get(ac, [])
        problems = []
        if layer == "后端" and not b:
            problems.append("缺 BE")
        if layer == "前端" and not f:
            problems.append("缺 FE")
        if layer == "端到端" and not (b or f):
            problems.append("缺 BE/FE")
        if layer == "手工" and not f:
            problems.append("缺 FE")
        if not t:
            problems.append("缺 TC")
        if problems:
            gaps.append(f"{ac}（{layer}）：{'、'.join(problems)}")
        rows.append(f"| {ac} | {layer} | {'、'.join(b) or '—'} | {'、'.join(f) or '—'} | "
                    f"{'、'.join(t) or '—'} | {'❌ ' + '、'.join(problems) if problems else '✅'} |")

    # 任务或用例里引用了 PRD 中不存在的 AC
    unknown = sorted({a for m in (be, fe, tc) for a in m} - known)

    covered = len(acs) - len(gaps)
    tc_total = len({x for v in tc.values() for x in v})
    lines = [
        "# 追溯矩阵",
        "",
        "> 由 `python3 tools/trace.py` 生成，不要手工编辑。",
        "",
        f"- AC 总数：{len(acs)}，完全覆盖：{covered}，有缺口：{len(gaps)}",
        f"- 关联到 AC 的测试用例数：{tc_total}",
        f"- 引用了不存在的 AC：{'、'.join(unknown) if unknown else '无'}",
        "",
        "| AC | 验证层 | 后端任务 | 前端任务 | 测试用例 | 状态 |",
        "|---|---|---|---|---|---|",
        *rows,
        "",
    ]
    if "--check" not in sys.argv:
        OUT.write_text("\n".join(lines), encoding="utf-8")
        print(f"wrote {OUT.relative_to(ROOT)}")
    print(f"AC {len(acs)} | covered {covered} | gaps {len(gaps)} | TCs {tc_total} | unknown refs {len(unknown)}")
    for g in gaps:
        print("  GAP", g)
    for u in unknown:
        print("  UNKNOWN", u)
    sys.exit(1 if (gaps or unknown) else 0)


if __name__ == "__main__":
    main()
