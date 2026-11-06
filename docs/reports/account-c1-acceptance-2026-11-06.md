# C1 后端验收记录（BE-422，2026-11-06）

环境：本地开发机（10 核，16 GiB），SQLite。**CI 不运行（P-21），都是本地结果。** 基线提交见任务单进度表的 BE-422 行。

## 全量测试

`cd server && go test -race -count=1 ./...`；`python3 -I tools/trace.py --check`。结果见提交说明和任务汇报。

## 变异确认（26 条，全部被发现）

`python3 -I tools/account-mutation/run.py`：逐条改坏，跑对应用例，确认失败，再还原。工具检查了工作区还原干净。

| 改坏了什么 | 规则 | 用例 | 结果 |
|---|---|---|---|
| 代号判定：其他代也放行 | F-ACC-03 规则 4 判定表 | `TestTC_ACC_03_28|TestTC_ACC_03_30|TestTC_ACC_03_42` | 失败（发现了） |
| 代号判定：pending 不转正 | F-ACC-03 规则 4 判定表 | `TestTC_ACC_03_24` | 失败（发现了） |
| 过期 token 也转正 | F-ACC-03 规则 4 | `TestTC_ACC_03_29` | 失败（发现了） |
| overlapMinGen 不判 | RS5-01 | `TestTC_ACC_03_40` | 失败（发现了） |
| 窗口外也软拒绝 | F-ACC-03 规则 4 | `TestTC_ACC_03_02` | 失败（发现了） |
| 限流放到判定之前 | RS6-05 | `TestTC_ACC_03_38` | 失败（发现了） |
| 重发也滑动到期 | RS4-10 | `TestTC_ACC_03_39` | 失败（发现了） |
| 代号复用 | RS5-05 | `TestTC_ACC_03_42|TestAccountSessionServerVectors` | 失败（发现了） |
| MAC 不校验 | RS4-05 | `TestTC_ACC_03_32` | 失败（发现了） |
| 改密不换 sid：当前会话不吊销 | RS4-03 | `TestTC_ACC_04_14` | 失败（发现了） |
| 提交事务不校验版本 | F-ACC-04 规则 8，RS4-06 | `TestCommitCredentialsChecks` | 失败（发现了） |
| 登录提交不校验版本 | F-ACC-04 规则 8 | `TestHookLoginVsRecover` | 失败（发现了） |
| recover 看版本 | F-ACC-04 规则 8，RS5-02 | `TestHookRecoverIgnoresVersion` | 失败（发现了） |
| 旧码优先：作废全部其他代 | RS4-01 | `TestTC_ACC_05_16` | 失败（发现了） |
| 旧码优先：不作废更新的代 | RS4-01 | `TestTC_ACC_04_12` | 失败（发现了） |
| retire_until 被延长 | F-ACC-05 规则 5 | `TestTC_ACC_05_16|TestTC_ACC_04_24` | 失败（发现了） |
| 上限约束所有代 | RS7-02 | `TestTC_ACC_04_29` | 失败（发现了） |
| 命中也受用户名 + IP 锁定 | RS5-02 | `TestTC_ACC_05_03` | 失败（发现了） |
| recoverBefore 按吊销原因：只有改密才带 | RS5-03 | `TestTC_ACC_03_34` | 失败（发现了） |
| 入表后复查出错不断开（失败关闭） | RS5-09 | `TestTC_ACC_03_36` | 失败（发现了） |
| 入表后复查不看代号 | RS5-08 | `TestHookConnStaleGen|TestPeriodicRecheckCoversGen` | 失败（发现了） |
| 周期复查一直不断开 | RS5-09 | `TestTC_ACC_03_37` | 失败（发现了） |
| 周期复查一次失败就断开 | RS5-09 | `TestTC_ACC_03_37` | 失败（发现了） |
| 无 sid 的 token 不看 iat | F-ACC-03 规则 11 | `TestTC_ACC_03_16` | 失败（发现了） |
| 注销清理不看申请是否撤销 | F-ACC-04 规则 8 | `TestFinishDeletionRespectsCancel` | 失败（发现了） |
| 提交事务不校验调用方 sid | RS7-01 | `TestCommitCredentialsChecks` | 失败（发现了） |

第一轮"提交事务不校验版本"没有被发现：接口层面 recover 同时递增版本和吊销，版本和 `sid` 两道校验互相兜底。补了存储层用例 `TestCommitCredentialsChecks` 后两条都能发现。

## 攻击序列

对应表在 `server/e2e/account_acceptance_test.go` 的 `attackSequences`，`TestAttackSequencesHaveTests` 检查用例都存在。

**没有执行**（只有 Postgres 下可达，SQLite 事务串行）：RS6-01 的 recover 事务内部交错、RS7-01 的持锁期间交错，即 AC-ACC-04-29。`TestTC_ACC_04_27_NotExecuted` 标 skip，**不算通过**。SQLite 上跑的是这些请求停在 Argon2 之后的版本（RS7-01、RS4-06）。

## 性能：开发机只记录，不算通过（发布判定等 P-12）

`IM_PERF_DEV=1 go test ./e2e -run TestPerfDevRecord -v`，不带 -race，Argon2 生产参数。

| 项 | 条件 | 结果 | AC | 判定 |
|---|---|---|---|---|
| 登录 | 串行 200 次（AC 要 ≥ 1000 次、3 次取最差） | p50 103 ms，p99 109 ms | AC-CHAT-90-11 | 未判定 |
| 登录 | 4 并发 200 次 | p50 132 ms，p99 160 ms | AC-CHAT-90-11 | 未判定 |
| 单聊文本，账号 token | 顺序 1000 条（AC 要 500 条/秒） | p50 0.22 ms，p99 0.55 ms | AC-CHAT-90-1 | 未判定 |

没有测：峰值存活内存（AC-CHAT-90-11）、AC-ACC-08-5 清理期间的 ack P99、AC-CHAT-90-2 的 TC-PERF 判定。

## 未执行或只验了服务端的

- AC-ACC-04-29：未执行，见上。
- AC-CHAT-90-1、90-11：开发机记录，不算通过；AC-CHAT-90-2：现有 e2e 全过，TC-PERF 判定等 P-12。
- AC-ACC-08-5 的 P99：没有测。
- 推送令牌（C3）：AC-ACC-03-19、03-27、04-6 的令牌部分。
- 带【前端】或端到端的 AC 的 SDK、界面部分：FE-600～620。AC-CHAT-90-10、90-15 要三套 SDK。
- AC-CHAT-90-5 的部署 e2e（TC-CHAT-90-04 的第二层）和 C2 以后的流程：没有执行，见下一节。

## console-server 部分（CS-200，console-server `6820da8`）

环境：本地开发机，真实 Postgres 17（本机已有的实例）、真实 im-server（SQLite，当前工作区源码构建）。`CON_TEST_REQUIRE_DB=1 go test -race -count=1 ./...` 全过。**都是本地结果。**

| AC | 结果 | 用例 | 说明 |
|---|---|---|---|
| AC-ACC-05-9 | 通过 | `TestTC_ACC_05_08_NoUserCredentialRoute`、`TestTC_ACC_05_08_SourceHasNoUserCredentialCalls`、`TestTC_ACC_05_08_ConsoleScopeAndCalls` | console 路由、业务请求字段、im-server 路径、主凭证 scope 都枚举了。后台账号自己的 3 个接口是白名单，确认不调用 im-server。服务端部分见 BE-421 |
| AC-ACC-09-6 | 通过 | `TestTC_ACC_09_04_Roles`、`TestAdminMatrix` | `support`、`auditor`（还有 `moderator`、`super_admin`）三个路由都 403 / 4030，不调用 im-server，写 denied 审计；`operator` 成功 |
| AC-CHAT-90-5（C1 流程） | 后端 e2e 通过；部署 e2e 未执行；C2 以后的流程未到 | `TestTC_CHAT_90_04_NoSecretsInLogsAndAudit` | 注册、登录、刷新、改密、找回和后台邀请码、详情，含失败路径。两边日志、console 全库、im-server 审计搜不到密码、恢复码、邀请码、access、refresh、`deviceName`、`sid`，`deploy/ops/scan-logs` 再扫一次，并用对照确认工具能发现 |

变异确认 20 条全部发现（脚本 19 条，部署示例多一个 scope 手工 1 条），工作区还原干净：审计记明文、审计记备注、日志记明文、intent 写失败仍执行、跳过权限检查、权限点换错、`support` 和超管多权限、详情透传、详情多 `sid`、取不到时说成没有账号、列表透传、动作名 `account.` 开头、404 说成没上线、scope 多 `account.reset` 和 `moderation.words`、部署示例多一个 scope、多一个改密路由、调用重置路径、响应不是 `no-store`。

没有验证：AC-ACC-09-1 的三端注册和页面、AC-ACC-09-11 的页面（FE-610～619）。部署示例已加 `account.invite`，`deploy/test_deploy.py` 本机缺 pyyaml 没有运行；已部署环境的调用方配置要手动加。
