#!/usr/bin/env python3
"""账号安全断言的变异确认（BE-422）：逐条把规则改坏，确认对应的用例失败，再改回来。

用法（在 im-mvp 目录）：python3 -I tools/account-mutation/run.py [名字...]
每条有超时，超时杀整个进程组，不留进程。改过的文件在 finally 里还原；开始前要求工作区这些文件没有未提交的改动。
结果写到 --out 指定的文件（默认只打印）。"""
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
SERVER = ROOT / "server"
TIMEOUT = 200

# (名字, 规则, 文件, 原文, 改成, 期望失败的用例)
MUTATIONS = [
    ("代号判定：其他代也放行", "F-ACC-03 规则 4 判定表", "internal/auth/check.go",
     'if claims.SID == "" || claims.Gen == st.CurrentGen {\n\t\treturn Result{Claims: claims, Verdict: VerdictOK}',
     'if true {\n\t\treturn Result{Claims: claims, Verdict: VerdictOK}',
     "TestTC_ACC_03_28|TestTC_ACC_03_30|TestTC_ACC_03_42"),
    ("代号判定：pending 不转正", "F-ACC-03 规则 4 判定表", "internal/auth/check.go",
     "if slices.Contains(st.PendingGens, claims.Gen) {", "if false && slices.Contains(st.PendingGens, claims.Gen) {",
     "TestTC_ACC_03_24"),
    ("过期 token 也转正", "F-ACC-03 规则 4", "internal/auth/check.go",
     'if errors.Is(verr, ErrExpiredToken) {\n\t\treturn Result{Claims: claims, Verdict: VerdictExpired}\n\t}\n\tif claims.SID',
     'if errors.Is(verr, ErrExpiredToken) && !slices.Contains(st.PendingGens, claims.Gen) {\n\t\treturn Result{Claims: claims, Verdict: VerdictExpired}\n\t}\n\tif claims.SID',
     "TestTC_ACC_03_29"),
    ("overlapMinGen 不判", "RS5-01", "internal/account/refresh.go",
     "if claims.Gen >= d.OverlapMinGen && ms < d.OverlapUntil {", "if ms < d.OverlapUntil {",
     "TestTC_ACC_03_40"),
    ("窗口外也软拒绝", "F-ACC-03 规则 4", "internal/account/refresh.go",
     "if claims.Gen >= d.OverlapMinGen && ms < d.OverlapUntil {", "if claims.Gen >= d.OverlapMinGen {",
     "TestTC_ACC_03_02"),
    ("限流放到判定之前", "RS6-05", "internal/account/refresh.go",
     "\t\tif d.RevokedAt != 0 {\n\t\t\treturn nil, s.refreshErr(ctx, d.UserID, revokeReasonOf(d.RevokeReason))",
     "\t\tif d.IssueCount >= s.cfg.RefreshIssueRate && d.IssueWindowStart+issueWindow.Milliseconds() > s.Clock.Now().UnixMilli() {\n\t\t\treturn nil, rateLimited(60)\n\t\t}\n\t\tif d.RevokedAt != 0 {\n\t\t\treturn nil, s.refreshErr(ctx, d.UserID, revokeReasonOf(d.RevokeReason))",
     "TestTC_ACC_03_38"),
    ("重发也滑动到期", "RS4-10", "internal/account/refresh.go",
     "\tnd.NextGen++ // 严格单调", "\tnd.ExpiresAt = s.slidExpiry(nd, now)\n\tnd.NextGen++ // 严格单调",
     "TestTC_ACC_03_39"),
    ("代号复用", "RS5-05", "internal/account/refresh.go",
     "\tgen := nd.NextGen\n", "\tgen := nd.CurrentGen + 1\n",
     "TestTC_ACC_03_42|TestAccountSessionServerVectors"),
    ("MAC 不校验", "RS4-05", "internal/account/tokens.go",
     "if !ok || !hmac.Equal([]byte(mac), []byte(refreshMAC(key, kid, sid, g, nonce))) {", "if !ok || (!hmac.Equal([]byte(mac), []byte(refreshMAC(key, kid, sid, g, nonce))) && false) {",
     "TestTC_ACC_03_32"),
    ("改密不换 sid：当前会话不吊销", "RS4-03", "internal/store/sqlite_account_tx.go",
     "\tif c.NewSession != nil {\n\t\tif err := insertDeviceSession(ctx, tx, *c.NewSession); err != nil {\n\t\t\treturn CredentialResult{}, err\n\t\t}\n\t}\n\tif err := insertAccountEvent(ctx, tx, c.Event)",
     "\tif c.CallerSID != \"\" {\n\t\tif _, err := tx.ExecContext(ctx, `UPDATE device_sessions SET revoked_at = 0, revoke_reason = '' WHERE sid = ?`, c.CallerSID); err != nil {\n\t\t\treturn CredentialResult{}, err\n\t\t}\n\t}\n\tif c.NewSession != nil {\n\t\tif err := insertDeviceSession(ctx, tx, *c.NewSession); err != nil {\n\t\t\treturn CredentialResult{}, err\n\t\t}\n\t}\n\tif err := insertAccountEvent(ctx, tx, c.Event)",
     "TestTC_ACC_04_14"),
    ("提交事务不校验版本", "F-ACC-04 规则 8，RS4-06", "internal/store/sqlite_account_tx.go",
     "WHERE user_id = ? AND credentials_version = ? AND delete_requested_at = 0 AND password_hash <> ''",
     "WHERE user_id = ? AND (credentials_version = ? OR 1 = 1) AND delete_requested_at = 0 AND password_hash <> ''",
     "TestCommitCredentialsChecks"),
    ("提交事务不校验调用方 sid", "RS7-01", "internal/store/sqlite_account_tx.go",
     "if errors.Is(err, sql.ErrNoRows) || (err == nil && revoked != 0) {", "if errors.Is(err, sql.ErrNoRows) {",
     "TestCommitCredentialsChecks"),
    ("登录提交不校验版本", "F-ACC-04 规则 8", "internal/store/sqlite_account.go",
     "UPDATE accounts SET last_login_at = ? WHERE user_id = ? AND credentials_version = ? AND delete_requested_at = 0",
     "UPDATE accounts SET last_login_at = ? WHERE user_id = ? AND (credentials_version = ? OR 1 = 1) AND delete_requested_at = 0",
     "TestHookLoginVsRecover"),
    ("recover 看版本", "F-ACC-04 规则 8，RS5-02", "internal/account/recover.go",
     "\tgen, err := s.Store.CommitRecover(ctx, store.RecoverCommit{",
     "\tif cur, err := s.Store.GetAccount(ctx, acct.UserID); err != nil || cur.CredentialsVersion != acct.CredentialsVersion {\n\t\treturn nil, credentialsInvalid()\n\t}\n\tgen, err := s.Store.CommitRecover(ctx, store.RecoverCommit{",
     "TestHookRecoverIgnoresVersion"),
    ("旧码优先：作废全部其他代", "RS4-01", "internal/store/sqlite_account_tx.go",
     "WHERE user_id = ? AND gen > ? AND used_at = 0 AND revoked_at = 0`, c.Now, c.UserID, c.Gen)",
     "WHERE user_id = ? AND gen <> ? AND used_at = 0 AND revoked_at = 0`, c.Now, c.UserID, c.Gen)",
     "TestTC_ACC_05_16"),
    ("旧码优先：不作废更新的代", "RS4-01", "internal/store/sqlite_account_tx.go",
     "WHERE user_id = ? AND gen > ? AND used_at = 0 AND revoked_at = 0`, c.Now, c.UserID, c.Gen)",
     "WHERE user_id = ? AND gen > ? AND 1 = 0`, c.Now, c.UserID, c.Gen)",
     "TestTC_ACC_04_12"),
    ("retire_until 被延长", "F-ACC-05 规则 5", "internal/store/sqlite_account_tx.go",
     "WHERE user_id = ? AND acked_at <> 0 AND used_at = 0 AND revoked_at = 0 AND retire_until = 0`",
     "WHERE user_id = ? AND acked_at <> 0 AND used_at = 0 AND revoked_at = 0`",
     "TestTC_ACC_05_16|TestTC_ACC_04_24"),
    ("上限约束所有代", "RS7-02", "internal/store/sqlite_account_tx.go",
     "if c.Gen == latest && count >= c.LimitMax {", "if count >= c.LimitMax {",
     "TestTC_ACC_04_29"),
    ("命中也受用户名 + IP 锁定", "RS5-02", "internal/account/recover.go",
     "\tban, err := s.Store.UserBan(ctx, acct.UserID)",
     "\tif e := s.guard.RecoverMiss(ctx, username, ip); e != nil {\n\t\treturn nil, e\n\t}\n\tban, err := s.Store.UserBan(ctx, acct.UserID)",
     "TestTC_ACC_05_03"),
    ("recoverBefore 按吊销原因：只有改密才带", "RS5-03", "internal/gateway/gateway.go",
     "g.kickEvent(ws, protocol.KickedEvent{Type: protocol.EventKicked, Reason: reason, RecoverBefore: g.revocation.RecoverBefore(ctx, claims.UserID)})",
     "g.kickEvent(ws, protocol.KickedEvent{Type: protocol.EventKicked, Reason: reason})",
     "TestTC_ACC_03_34"),
    ("入表后复查出错不断开（失败关闭）", "RS5-09", "internal/gateway/gateway.go",
     'g.log.Error("recheck after register", "err", res.Err, "uid", uid)\n\t\tc.closeWith(protocol.CloseInternalError, "recheck failed")',
     'g.log.Error("recheck after register", "err", res.Err, "uid", uid)',
     "TestTC_ACC_03_36"),
    ("入表后复查不看代号", "RS5-08", "internal/auth/check.go",
     'if claims.SID != "" && claims.Gen != st.CurrentGen {\n\t\treturn Result{Claims: claims, Verdict: VerdictSuperseded}',
     'if claims.SID != "" && claims.Gen != st.CurrentGen && false {\n\t\treturn Result{Claims: claims, Verdict: VerdictSuperseded}',
     "TestHookConnStaleGen|TestPeriodicRecheckCoversGen"),
    ("周期复查一直不断开", "RS5-09", "internal/gateway/gateway.go",
     "if n := c.recheckFails.Add(1); n >= recheckMaxFailures {", "if n := c.recheckFails.Add(1); n >= 1000 {",
     "TestTC_ACC_03_37"),
    ("周期复查一次失败就断开", "RS5-09", "internal/gateway/gateway.go",
     "if n := c.recheckFails.Add(1); n >= recheckMaxFailures {", "if n := c.recheckFails.Add(1); n >= 1 {",
     "TestTC_ACC_03_37"),
    ("无 sid 的 token 不看 iat", "F-ACC-03 规则 11", "internal/auth/check.go",
     "if acct.CredentialsChangedAt != 0 && claims.IssuedAt.UnixMilli() < acct.CredentialsChangedAt {", "if false {",
     "TestTC_ACC_03_16"),
    ("注销清理不看申请是否撤销", "F-ACC-04 规则 8", "internal/store/sqlite_account_tx.go",
     "UPDATE accounts SET password_hash = '' WHERE user_id = ? AND delete_requested_at <> 0 AND delete_requested_at <= ?`, userID, requestedBefore)",
     "UPDATE accounts SET password_hash = '' WHERE user_id = ? AND ? <> 0`, userID, requestedBefore)",
     "TestFinishDeletionRespectsCancel"),
]


def run_tests(pattern):
    """返回 (是否有用例失败, 输出尾部)。超时杀进程组。"""
    p = subprocess.Popen(
        ["go", "test", "-count=1", "-timeout", f"{TIMEOUT - 20}s", "-tags", "imhooks", "./e2e", "-run", pattern],
        cwd=SERVER, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, start_new_session=True,
    )
    try:
        out, _ = p.communicate(timeout=TIMEOUT)
    except subprocess.TimeoutExpired:
        os.killpg(p.pid, signal.SIGKILL)
        out, _ = p.communicate()
        return True, "timeout\n" + out[-400:]
    return p.returncode != 0, out[-600:]


def main():
    args = sys.argv[1:]
    out_path = None
    if "--out" in args:
        i = args.index("--out")
        out_path = args[i + 1]
        del args[i : i + 2]
    dirty = subprocess.run(["git", "status", "--porcelain", "server/internal"], cwd=ROOT, capture_output=True, text=True).stdout
    if dirty.strip():
        sys.exit("server/internal has uncommitted changes; commit first\n" + dirty)
    results = []
    for name, rule, rel, old, new, pattern in MUTATIONS:
        if args and name not in args:
            continue
        path = SERVER / rel
        original = path.read_text()
        if original.count(old) != 1:
            results.append({"name": name, "rule": rule, "status": "NOT_APPLIED", "tests": pattern})
            print("NOT_APPLIED", name, original.count(old), flush=True)
            continue
        start = time.time()
        try:
            path.write_text(original.replace(old, new))
            failed, tail = run_tests(pattern)
        finally:
            path.write_text(original)
        build_failed = "[build failed]" in tail or "[setup failed]" in tail
        status = "BUILD_FAILED" if build_failed else ("KILLED" if failed else "SURVIVED")
        results.append({"name": name, "rule": rule, "status": status, "tests": pattern, "seconds": round(time.time() - start)})
        print(status, name, "::", pattern, flush=True)
        if status != "KILLED":
            print(tail, flush=True)
    still = subprocess.run(["git", "status", "--porcelain", "server/internal"], cwd=ROOT, capture_output=True, text=True).stdout
    if still.strip():
        sys.exit("files not restored:\n" + still)
    if out_path:
        Path(out_path).write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n")
    bad = [r for r in results if r["status"] != "KILLED"]
    print(f"{len(results) - len(bad)}/{len(results)} killed")
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
