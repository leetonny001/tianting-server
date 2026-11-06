# 待 Frank 授权 / 确认的事项

> AI 遇到需要授权或拍板的事项时记在这里（规则见 [自主工作规则](../process/AUTONOMY.md) 第 3 节），每天的日报会列出未处理的项。
> 处理后把状态改为"已处理"，并写明结论；也可以直接在聊天里回复。

## 未处理

| # | 日期 | 事项 | 为什么需要你 | 不处理的影响 | 建议 |
|---|---|---|---|---|---|
| P-9 | 2026-11-04 | M3 手工验收（FE-248 及任务单记录的手工项）：iPhone 15 Safari 安全区截图、macOS / iOS Safari 拼音输入录屏、VoiceOver | 需要真机和人工操作，AI 无法独立完成 | 不阻塞开发，但 M3 不能算验收完成 | 社交页面完成后一起做，一次跑完 M3 演示路径；回复"社交页面完成后我来做"或指定其他人 |
| P-10 | 2026-11-04 | 下载 SecLists 的 `10-million-password-list-top-10000.txt` 作为弱密码表（PRD-admin F-CON-04 规则 1，约 80KB，来自 github.com/danielmiessler/SecLists） | 下载外部文件需要确认 | 目前用手写的 406 条暂行表，能拦住常见长口令，但覆盖不如正式列表；不阻塞开发，上线前必须换 | 回复"可以下载"，或自己放到 `tianting-console-server/assets/weak-passwords.txt` |
| P-6 | 2026-11-03 | 法务确认后台能否查看单聊消息正文（X-7），以及宿主 App 隐私政策是否要更新 | 法律问题 | 在确认前后台不开放单聊正文查看（默认 ①），不阻塞其他工作 | 找法务确认后回复结论 |
| P-7 | 2026-11-03 | 两个不阻塞的确认项：① F-1 告警发给谁、是否 7×24、严重告警是否买电话 / 短信（PRD-ops；默认先发给 Frank：办公 IM 机器人 + 邮件）；② D-1 ws 错误里"哪个用户"统一用 `details.userId`，不单独加 `userId` 字段（PRD-core v1.3 12.3 节；M4 前确认） | 产品 / 安全口径 | 都有默认方案，不确认就按默认做。**2026-11-05 更新**：Q-1 和 D-1 的确认时点都已到：主仓库未提交改动里已有 `0012_msg_grants.sql`（BE-225 看起来已开工）；M4 已提交（`46bf340`），已提交的协议和服务端用单独的 `error.userId`，和 D-1 的默认方案不一致，SDK 两种都兼容 | 回复"按默认"或逐项改；按默认的话把 `error.userId` 改为 `details.userId`，现在还没有外部客户端，成本最低 |
| P-2 | 2026-11-02 | 创建"自主开发"定时任务 `im-autodev`（每天 09:30、14:30） | 安全分类器要求本人确认：任务会自动改代码并推送 | 只有日报，开发要在聊天里让 AI 做 | 按 [AUTODEV_TASK.md](../process/AUTODEV_TASK.md) 在应用里创建，或允许 AI 创建 |
| P-1 | 2026-11-02 | gh 令牌补 `workflow` 权限：`gh auth refresh -h github.com -s workflow` | 账号权限变更 | 无阻塞：AI 修改 `.github/workflows/` 时改用 SSH 推送即可 | 有空时做一次 |
| P-11 | 2026-11-04 | AC-OPS-04-2 今日累计的验收条件。PRD-ops v1.1 已放宽到 3%，但 5 轮实测 1229.5、1237.2、1262.9、1251.4、1296.8（误差 2.5%～8.1%） | `increase()` 的外推误差取决于抓取历史和当时的抓取相位，波动大，约 13 分钟的历史时超过 3%，最大到 8.1%。AC 的前提“干净的 Prometheus”本身不成立，要改 AC，属于产品决定 | testenv 工作流持续失败（只有这一项），其他项都通过 | 保留 3%，前提改为“已抓取该节点 30 分钟以上”，验收时把这一步排到后面（按外推估算约 1.4%，要跑了才知道）。原来的 5% 方案已被实测否定。**2026-11-05 更正**：CI 日志实际是 5 轮，还有 `1b6e284` 的 1297.6（误差 8.1%），范围是 2.5%～8.1%。建议先实测再定；仍不稳定就改用计数器差值验收 |
| P-12 | 2026-11-04 | 提供 EP 压测用的测试机（Linux，8 核以上，16GB，能装 docker），或决定用付费的 GitHub 大规格 runner | 要花钱或要你的机器。开发机不装 Prometheus、Grafana；GitHub 的标准 runner 配置查不到，按惯例这个私有仓库大概只有 2 核，同机放不下 1 万连接、压测客户端和观测组件 | AC-OPS-06-2、06-10、07-8，90-1 和 90-2 的 Grafana、Prometheus 部分没法验收，AC-PUSH-04-2 的发布结论也要目标机，第一批运维不能算验收完成。不影响继续开发 | 目标服务器确定后直接用它，C-50 上线前本来就要在目标机重测，可以合并成一次 |
| P-13 | 2026-11-04 | 上线前找人做独立渗透测试（AC-OPS-90-6），并在 Firefox、Safari、Edge 和真机上回归（FE-507） | AI 的自查和实现有共同的盲点，AC 要求手工；真机和人工 AI 做不了。检查表和重点见 pentest-ops-2026-11-04.md | 第一批运维不能算验收完成。不影响继续开发 | 和 P-12 的目标服务器一起安排，部署后做 |
| P-14 | 2026-11-05 | 在开发机安装 Rust（Q16 PC Tauri 壳，FE-312、FE-313） | 安装系统软件要你授权 | PC 端不开工；H5、原生端不受影响 | 等要做 PC 时回复“授权安装 Rust” |
| P-15 | 2026-11-05 | 运行 `tianting-sdk-kotlin/scripts/setup-ci-deploy-key.sh`、`tianting-sdk-swift/scripts/setup-ci-deploy-key.sh`、`tianting-android/scripts/setup-ci-deploy-key.sh`、`tianting-ios/scripts/setup-ci-deploy-key.sh`（只读部署密钥 + Actions secret） | 建密钥和 secret 要你本人操作 | 两个原生 SDK 的 CI 不跑集成测试；Android 和 iOS 的 CI 不跑构建和单测，显示绿色但只跑了静态检查 | 有空时运行一次 |
| P-16 | 2026-11-05 | 查看 emulator-5554：`com.vrevneoiw.eqfe.yi_sport` 的 `NativePromotionActivity` 从 12:18 起有 ANR（12:18、12:32 两份记录），弹窗还开着。开发 Android App 的子代理在这段时间向这台模拟器注入过点击和返回键，弹窗重新出现时还向当时的焦点窗口发过一次 `input text backtest` 和一次返回键，点过一次“等待” | 这是你的业务 App 和模拟器，AI 不动。ANR 原因没查，不能排除和这些注入有关 | 弹窗占着焦点，影响这台模拟器上的操作 | 确认 yi 的页面状态和输入框没有多出 `backtest`；需要时自己关掉弹窗。以后天听 Android 测试用别的 AVD（`ANDROID_SERIAL`） |
| P-17 | 2026-11-05 | Android 首版的 2 个口径：① 4001 文案仍是 PRD 的“当前版本过低，请刷新页面”，原生端其实没有“刷新”（PRD-ui 3.5 节规定不能覆盖）；② 好友制发起单聊的提示对官方账号也会弹（AC-UI-05-6 没写这种情况，弹窗时拿不到资料），H5 和 Android 一致 | 产品文案和规则，改 AC 是你的决定 | 不阻塞 | 都有默认：① 原生端改成“当前版本过低，请更新 App”；② 保持现状 |
| P-18 | 2026-11-05 | 是否结束遗留进程 `im-server`（PID 87183，路径在本会话 scratchpad 下，运行约 28 小时，只监听 127.0.0.1:60971～60975，检查时没有已建立的连接） | AI 尝试结束时被权限规则拒绝，所以留给你决定 | 只占少量内存和本机端口，不影响开发 | 确认不需要后运行 `kill 87183` |
| P-28 | 2026-11-06 | PRD-chat-v1.0 第 20.0 节的 3 个事实问题：有没有已发给真实用户的 App 版本；有没有生产环境和真实用户数据；目标用户在哪里、用什么手机 | 只有你知道 | 前两个按"没有"冻结，假设不成立要走变更；第三个 C3 开工前必须答 | 见第 20.0 节 |
| P-21 | 2026-11-06 | GitHub Actions 没有预算：Frank 的 Actions 预算是 $0，也没有额外预算（2026-11-06 确认）。CI 暂停 | 花钱是你的决定 | 所有仓库的 CI 都不运行，结果只有本地验证；真实工具的 `ops-deploy`、Android、iOS CI 跑不了。上线前的结论要标明 | 有预算或免费额度时告诉我，我补跑 |

## 已处理

| # | 日期 | 事项 | 结论 |
|---|---|---|---|
| P-19 | 2026-11-06 | 聊天室和迁移的 14 项（L-5～L-15、L-19～L-21） | Frank 决定不替换现有项目的腾讯 IM SDK，按微信式 IM 设计：L2～L4 取消，这 14 项和业务方数据 N-1～N-8 都不再需要 |
| P-24 | 2026-11-06 | CS-133 的 2 个口径：降级也要批准；没有启用超管时可给待激活超管重发激活码 | Frank 回复"保持现状"。PRD-ops 升 v1.3（AC-OPS-01-20） |
| P-29 | 2026-11-06 | PRD-chat v1.0 分发时发现的矛盾和缺口；后台正文页是否放开文件内容 | Frank 回复"都按建议"：升 v1.1 修正；不放开 |
| P-26 | 2026-11-06 | PRD-chat v0.3 第 20 节的 13 个拍板项 | Frank 回复"全部按建议"，已写入 PRD-chat-v0.4。3 个事实问题没有回答，转 P-28 |
| P-27 | 2026-11-06 | 微信式 IM 的范围和顺序 | Frank 回复"按建议，写 PRD"：第一批为账号、图片文件、离线推送、撤回引用 @、会话管理、敏感词举报 |
| P-25 | 2026-11-06 | 工作队列之后做什么（日报提出：是否新增 B6 集群化） | Frank 决定下一步做微信式 IM（QUEUE 阶段五）。B6（BE-070～075）没有排期，AC-ADM-90-1、90-2、CS-140～142、BE-340 仍依赖它 |
| P-22 | 2026-11-06 | P1 的 7 项产品口径 | Frank 回复"全部按建议"。PRD-admin、PRD-ops、PRD-ui 升 v1.2。①的建议有死结（只有一个超管时第二个没人能批准），AI 补充：运维空间启用的超管少于 2 个时可用命令行创建；任务 BE-244、CS-063、CS-133、FE-441、FE-522 |
| P-23 | 2026-11-06 | CS-062 SSO | Frank 回复"全部按建议"：没有明确需求，先不做 |
| P-20 | 2026-11-05 | AC-PUSH-04-4 怎么判 | Frank 回复“按建议改”：单聊 ack P99 ≤ 200ms（未校准，目标机重测），增幅只记录。PRD-live 升 v1.1 |
| P-19（L1 部分） | 2026-11-05 | PRD-live 13.1 节 7 项（L-1～L-4、L-16、L-17、L-18） | Frank 回复“全部按建议”；2026-11-05 随 PRD-live v1.0 冻结，已落实到 PRD-ui v1.1、PRD-admin v1.1 |
| P-14（旧） | 2026-11-05 | 主仓库未提交改动的归属（BE-220 起第二批） | 就是第二批开发中的改动，已提交（`cd3899a`、`e0b3395`、`70f4ead`），不是残留 |
| Q-1 | 2026-11-05 | 群消息正文查看的凭证放在哪里（PRD-admin 15 节） | Frank 确认默认方案：console 主凭证永远不能读正文；正文走单独的 `console-msgread` 凭证，由 console-server 持有，im-server 侧授权、配额和审计。与 PRD-admin v1.0 第 15 节的理解一致，PRD 不用改。不做独立小服务 |
| P-9 | 2026-11-04 | 创建 tianting-console-server 私有仓库，配置和 tianting-server 互读的只读部署密钥 | 2026-11-04 Frank 建仓库并运行 setup-ci-deploy-keys.sh；console-server 的 CI（含真实 im-server 的端到端、真实 MinIO 的 WORM）和带 console-server 的 testenv 都通过。tianting-admin 的端到端测试还没有进 CI |
| P-5 | 2026-11-04 | 本机安装 Postgres | 2026-11-04 Frank 授权。`brew install postgresql@17`（17.11），只监听本机。没有开启开机自启：需要时运行 `$(brew --prefix postgresql@17)/bin/pg_ctl -D $(brew --prefix)/var/postgresql@17 -l $(brew --prefix)/var/postgresql@17/server.log start`。开发库 `tianting_console_dev`、测试库 `tianting_console_test` |
| P-8 | 2026-11-03 | tianting-web 的 CI 读取私有 SDK / 服务端 | 2026-11-03 Frank 运行 setup-ci-deploy-keys.sh；CI 检出两个仓库、构建 SDK、检查、单元测试、构建、端到端测试全部通过 |
| P-4 | 2026-11-03 | tianting-sdk-ts 的 CI 读取私有 tianting-server | 2026-11-03 Frank 运行 setup-ci-deploy-key.sh：只读部署密钥 + Actions secret；CI 集成测试已实际执行并通过。仓库保持私有 |
| P-3 | 2026-11-03 | 产品规划拍板（A～D 14 项 + 社交 6 项） | 2026-11-03 全部按建议通过，产品名天听；X-3 安装 Postgres 转为 P-5，X-7 法务确认转为 P-6 |
| — | 2026-11-02 | ARCH-v1 决策 A1～A4 | 按建议通过（Postgres、首帧 auth、B2.5 排在 B3 前、NATS + Redis） |
