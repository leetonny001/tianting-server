# 待 Frank 授权 / 确认的事项

> AI 遇到需要授权或拍板的事项时记在这里（规则见 [自主工作规则](../process/AUTONOMY.md) 第 3 节），每天的日报会列出未处理的项。
> 处理后把状态改为"已处理"，并写明结论；也可以直接在聊天里回复。

## 未处理

| # | 日期 | 事项 | 为什么需要你 | 不处理的影响 | 建议 |
|---|---|---|---|---|---|
| P-9 | 2026-11-04 | M3 手工验收（FE-248 及任务单记录的手工项）：iPhone 15 Safari 安全区截图、macOS / iOS Safari 拼音输入录屏、VoiceOver | 需要真机和人工操作，AI 无法独立完成 | 不阻塞开发，但 M3 不能算验收完成 | 社交页面完成后一起做，一次跑完 M3 演示路径；回复"社交页面完成后我来做"或指定其他人 |
| P-6 | 2026-11-03 | 法务确认后台能否查看单聊消息正文（X-7），以及宿主 App 隐私政策是否要更新 | 法律问题 | 在确认前后台不开放单聊正文查看（默认 ①），不阻塞其他工作 | 找法务确认后回复结论 |
| P-7 | 2026-11-03 | 三个不阻塞的确认项：① F-1 告警发给谁、是否 7×24、严重告警是否买电话 / 短信（PRD-ops；默认先发给 Frank：办公 IM 机器人 + 邮件）；② Q-1 群消息查看用单独的 `console-msgread` 凭证、主凭证永远不能读正文（PRD-admin；第二批开工前确认）；③ D-1 ws 错误里"哪个用户"统一用 `details.userId`，不单独加 `userId` 字段（PRD-core v1.3 12.3 节；M4 前确认） | 产品 / 安全口径 | 都有默认方案，不确认就按默认做 | 回复"按默认"或逐项改 |
| P-2 | 2026-11-02 | 创建"自主开发"定时任务 `im-autodev`（每天 09:30、14:30） | 安全分类器要求本人确认：任务会自动改代码并推送 | 只有日报，开发要在聊天里让 AI 做 | 按 [AUTODEV_TASK.md](../process/AUTODEV_TASK.md) 在应用里创建，或允许 AI 创建 |
| P-1 | 2026-11-02 | gh 令牌补 `workflow` 权限：`gh auth refresh -h github.com -s workflow` | 账号权限变更 | 无阻塞：AI 修改 `.github/workflows/` 时改用 SSH 推送即可 | 有空时做一次 |

## 已处理

| # | 日期 | 事项 | 结论 |
|---|---|---|---|
| P-5 | 2026-11-04 | 本机安装 Postgres | 2026-11-04 Frank 授权。`brew install postgresql@17`（17.11），只监听本机。没有开启开机自启：需要时运行 `$(brew --prefix postgresql@17)/bin/pg_ctl -D $(brew --prefix)/var/postgresql@17 -l $(brew --prefix)/var/postgresql@17/server.log start`。开发库 `tianting_console_dev`、测试库 `tianting_console_test` |
| P-8 | 2026-11-03 | tianting-web 的 CI 读取私有 SDK / 服务端 | 2026-11-03 Frank 运行 setup-ci-deploy-keys.sh；CI 检出两个仓库、构建 SDK、检查、单元测试、构建、端到端测试全部通过 |
| P-4 | 2026-11-03 | tianting-sdk-ts 的 CI 读取私有 tianting-server | 2026-11-03 Frank 运行 setup-ci-deploy-key.sh：只读部署密钥 + Actions secret；CI 集成测试已实际执行并通过。仓库保持私有 |
| P-3 | 2026-11-03 | 产品规划拍板（A～D 14 项 + 社交 6 项） | 2026-11-03 全部按建议通过，产品名天听；X-3 安装 Postgres 转为 P-5，X-7 法务确认转为 P-6 |
| — | 2026-11-02 | ARCH-v1 决策 A1～A4 | 按建议通过（Postgres、首帧 auth、B2.5 排在 B3 前、NATS + Redis） |
