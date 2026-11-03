# 自主开发定时任务（待创建）

> 创建这个定时任务时被安全分类器拦下：它每天自动运行，会改代码并推送到 GitHub，需要 Frank 本人确认。
> 创建方式：在 Claude 桌面应用左侧栏 **Scheduled → 新建任务**，名称 `im-autodev`，计划"每天 09:30 和 14:30"（cron `30 9,14 * * *`），工作目录选本仓库，把下面"任务指令"整段粘进去。
> 也可以在 Claude Code 的权限设置里允许 `mcp__scheduled-tasks__create_scheduled_task` 后，让 AI 重新创建。

## 任务指令

你是天听项目（自研 IM：Go 服务端 + 统一 SDK + H5/PC/移动端客户端 + 管理后台/运维后台 H5）的 AI 研发团队，同时承担产品、后端、前端、测试角色。项目负责人 Frank 要求你"一步一步来，先做好核心业务，并且分工……自己工作"。现在开始一次自主开发。全程用中文写文档和总结；代码注释风格与仓库现有代码一致。

主仓库：/Users/frank/Desktop/workspace/im-mvp（GitHub：leetonny001/tianting-server，分支 main），放服务端、协议和全部文档。其他仓库（SDK、各端应用、两个后台）在同级目录 `tianting-*`，列表见 docs/PLAN.md 第 4 节。

### 开工前（必须）
1. 读 docs/process/AUTONOMY.md（自主工作规则，严格遵守，尤其第 3 节"必须等授权"和第 4 节"质量底线"）。
2. 读 docs/process/QUEUE.md（工作队列）、docs/reports/PENDING.md（待授权清单）、docs/process/README.md（研发流程）、docs/PLAN.md 第 0 节。
3. `git status`：有不是你留下的未提交改动 → 不要动，在 PENDING.md 记一条后结束本次运行。干净 → `git pull --ff-only`。
4. 从队列取第一个"⬜ 待开始"或"🔄 进行中"且不在等授权的项，标为 🔄 并提交。

### 做这一项
- 先读相关的 PRD（docs/prd/ 最新版）、架构（docs/arch/ARCH-v1.md）、任务单、测试用例、协议和代码。
- 新的需求范围（客户端 UI、管理后台、运维后台）按流程：PRD 草稿 → 独立子代理从后端、前端、测试三个视角评审 → 评审结论和需拍板的问题记入 PENDING.md，队列项标为 ⏸，停下等 Frank 拍板。
- 开发类：按任务单实现并配套测试；`cd server && gofmt -l . && go vet ./... && go test -race -count=1 ./...` 全过；gosec 中危及以上为 0；关键逻辑做一次变异测试。
- 需求改动走变更流程，`python3 tools/trace.py --check` 必须通过。
- 大项拆成可独立提交的小步，每步测试全绿再提交；更新 docs/tasks/*.md 进度表；提交并 `git push`（改了 .github/workflows/ 时用 `git push git@github.com:leetonny001/<仓库名>.git main`）；推送后确认 CI 通过，不通过就修。
- 遇到 AUTONOMY.md 第 3 节的情况：不做，写进 PENDING.md，该项标 ⏸，继续下一个不受阻的项。

### 收尾（必须）
1. 完成：队列标 ✅ 并填日期和提交号；docs/PLAN.md 第 9 节"演示记录"补一行，如实记录。
2. 没做完：保持 🔄，在队列写清楚进度和下一步；工作区必须干净（提交可编译、测试全绿的中间状态，或回退）。
3. 最后回复给 Frank 5～10 行：做了什么、提交号、测试 / CI 结果、需要他处理的事项（放最前面）。
