# 待 Frank 授权 / 确认的事项

> AI 遇到需要授权或拍板的事项时记在这里（规则见 [自主工作规则](../process/AUTONOMY.md) 第 3 节），每天的日报会列出未处理的项。
> 处理后把状态改为"已处理"，并写明结论；也可以直接在聊天里回复。

## 未处理

| # | 日期 | 事项 | 为什么需要你 | 不处理的影响 | 建议 |
|---|---|---|---|---|---|
| P-9 | 2026-11-04 | M3 手工验收（FE-248 及任务单记录的手工项）：iPhone 15 Safari 安全区截图、macOS / iOS Safari 拼音输入录屏、VoiceOver | 需要真机和人工操作，AI 无法独立完成 | 不阻塞开发，但 M3 不能算验收完成 | 社交页面完成后一起做，一次跑完 M3 演示路径；回复"社交页面完成后我来做"或指定其他人 |
| P-10 | 2026-11-04 | 下载 SecLists 的 `10-million-password-list-top-10000.txt` 作为弱密码表（PRD-admin F-CON-04 规则 1，约 80KB，来自 github.com/danielmiessler/SecLists） | 下载外部文件需要确认 | 目前用手写的 406 条暂行表，能拦住常见长口令，但覆盖不如正式列表；不阻塞开发，上线前必须换 | 回复"可以下载"，或自己放到 `tianting-console-server/assets/weak-passwords.txt` |
| P-6 | 2026-11-03 | 法务确认后台能否查看单聊消息正文（X-7），以及宿主 App 隐私政策是否要更新 | 法律问题 | 在确认前后台不开放单聊正文查看（默认 ①），不阻塞其他工作 | 找法务确认后回复结论 |
| P-7 | 2026-11-03 | 三个不阻塞的确认项：① F-1 告警发给谁、是否 7×24、严重告警是否买电话 / 短信（PRD-ops；默认先发给 Frank：办公 IM 机器人 + 邮件）；② Q-1 群消息查看用单独的 `console-msgread` 凭证、主凭证永远不能读正文（PRD-admin；第二批开工前确认）；③ D-1 ws 错误里"哪个用户"统一用 `details.userId`，不单独加 `userId` 字段（PRD-core v1.3 12.3 节；M4 前确认） | 产品 / 安全口径 | 都有默认方案，不确认就按默认做 | 回复"按默认"或逐项改 |
| P-2 | 2026-11-02 | 创建"自主开发"定时任务 `im-autodev`（每天 09:30、14:30） | 安全分类器要求本人确认：任务会自动改代码并推送 | 只有日报，开发要在聊天里让 AI 做 | 按 [AUTODEV_TASK.md](../process/AUTODEV_TASK.md) 在应用里创建，或允许 AI 创建 |
| P-1 | 2026-11-02 | gh 令牌补 `workflow` 权限：`gh auth refresh -h github.com -s workflow` | 账号权限变更 | 无阻塞：AI 修改 `.github/workflows/` 时改用 SSH 推送即可 | 有空时做一次 |
| P-11 | 2026-11-04 | AC-OPS-04-2 今日累计的验收条件。PRD-ops v1.1 已放宽到 3%，但 4 轮实测 1229.5、1237.2、1262.9、1251.4（误差 2.5%～5.2%） | `increase()` 的外推误差取决于抓取历史和当时的抓取相位，波动大，约 13 分钟的历史时超过 3%，偶尔超过 5%。AC 的前提“干净的 Prometheus”本身不成立，要改 AC，属于产品决定 | testenv 工作流持续失败（只有这一项），其他项都通过 | 保留 3%，前提改为“已抓取该节点 30 分钟以上”，验收时把这一步排到后面（按外推估算约 1.4%，要跑了才知道）。原来的 5% 方案已被实测否定 |
| P-12 | 2026-11-04 | 提供 EP 压测用的测试机（Linux，8 核以上，16GB，能装 docker），或决定用付费的 GitHub 大规格 runner | 要花钱或要你的机器。开发机不装 Prometheus、Grafana；GitHub 的标准 runner 配置查不到，按惯例这个私有仓库大概只有 2 核，同机放不下 1 万连接、压测客户端和观测组件 | AC-OPS-06-2、06-10、07-8，90-1 和 90-2 的 Grafana、Prometheus 部分没法验收，第一批运维不能算验收完成。不影响继续开发 | 目标服务器确定后直接用它，C-50 上线前本来就要在目标机重测，可以合并成一次 |
| P-13 | 2026-11-04 | 上线前找人做独立渗透测试（AC-OPS-90-6），并在 Firefox、Safari、Edge 和真机上回归（FE-507） | AI 的自查和实现有共同的盲点，AC 要求手工；真机和人工 AI 做不了。检查表和重点见 pentest-ops-2026-11-04.md | 第一批运维不能算验收完成。不影响继续开发 | 和 P-12 的目标服务器一起安排，部署后做 |

## 已处理

| # | 日期 | 事项 | 结论 |
|---|---|---|---|
| P-9 | 2026-11-04 | 创建 tianting-console-server 私有仓库，配置和 tianting-server 互读的只读部署密钥 | 2026-11-04 Frank 建仓库并运行 setup-ci-deploy-keys.sh；console-server 的 CI（含真实 im-server 的端到端、真实 MinIO 的 WORM）和带 console-server 的 testenv 都通过。tianting-admin 的端到端测试还没有进 CI |
| P-5 | 2026-11-04 | 本机安装 Postgres | 2026-11-04 Frank 授权。`brew install postgresql@17`（17.11），只监听本机。没有开启开机自启：需要时运行 `$(brew --prefix postgresql@17)/bin/pg_ctl -D $(brew --prefix)/var/postgresql@17 -l $(brew --prefix)/var/postgresql@17/server.log start`。开发库 `tianting_console_dev`、测试库 `tianting_console_test` |
| P-8 | 2026-11-03 | tianting-web 的 CI 读取私有 SDK / 服务端 | 2026-11-03 Frank 运行 setup-ci-deploy-keys.sh；CI 检出两个仓库、构建 SDK、检查、单元测试、构建、端到端测试全部通过 |
| P-4 | 2026-11-03 | tianting-sdk-ts 的 CI 读取私有 tianting-server | 2026-11-03 Frank 运行 setup-ci-deploy-key.sh：只读部署密钥 + Actions secret；CI 集成测试已实际执行并通过。仓库保持私有 |
| P-3 | 2026-11-03 | 产品规划拍板（A～D 14 项 + 社交 6 项） | 2026-11-03 全部按建议通过，产品名天听；X-3 安装 Postgres 转为 P-5，X-7 法务确认转为 P-6 |
| — | 2026-11-02 | ARCH-v1 决策 A1～A4 | 按建议通过（Postgres、首帧 auth、B2.5 排在 B3 前、NATS + Redis） |
