# 敏感词和举报的部署、运维和上线门槛（PRD-chat C6，BE-498）

只写 C6 多出来的部分。数字都未校准。**仓库、镜像、部署示例里都不放真实词库**（PRD-chat F-MOD-01 规则 5）；本文的例子只用特征词。

## 1. 配置

| 配置 | 默认 | 说明 |
|---|---|---|
| `IM_MOD_ENABLED` | 关 | C6 开关。关闭时不检查敏感词、客户端 `report` 返回 403 / 4030 `featureDisabled`、`features.moderation=false`；**管理接口（词库、举报队列、证据、结案）和清理任务（`report_purge`、`moderation_hits_purge`）照常**（P-48 ④）：开过又关掉，已有的举报也要能处理完，保留期照样到期删除 |
| `IM_REPORT_KEY` | 无 | 举报证据的 AES-256-GCM 密钥，见第 2 节。开关打开时必须有，**任何环境都没有默认值** |
| `IM_REPORT_DAILY` | 20 | 每人每天（Asia/Shanghai 自然日）举报次数，4040 也算，幂等的重复不算 |
| `IM_REPORT_EVIDENCE_DAYS` | 180 | 证据和举报记录一起到期删除（P-48 ②）；证据引用的文件按它延长保留 |
| `IM_MOD_HITS_DAYS` | 90 | 命中记录（用户、场景、词 ID、时间，没有内容）的保留期 |
| `IM_CLIENT_CUSTOM_MSG` | 生产 0，其他 1 | 客户端能不能发 `custom`（`custom` 不过敏感词检查）。**不跟 `IM_MOD_ENABLED` 走**：C6 的版本部署到生产后，客户端就不能发 `custom` 了（不管开关）。生产打开时配置检查告警 |
| `IM_PUBLIC_OPEN_ACK` | 0 | 生产 `IM_REGISTER_MODE=open` 的确认项之一（见第 6 节），C6 上线不需要 |

启动检查（不满足就非 0 退出，错误信息不含密钥）：

- 开关打开：`IM_REPORT_KEY` 缺失、不是 32 字节的 base64、或者和 `IM_QUARANTINE_KEY`、JWT 密钥相同。开关关闭时设置了也校验格式。
- 生产 + 开关打开：要 `IM_V13_ADMIN_FEATURES=1`（举报了要有删除、禁言、封禁的手段）和 `IM_PRIVACY_NOTICE_VERSION`（隐私说明要写 C6 的告知项）。
- 开关打开时启动加载词库失败（库读不了）：启动失败。
- 生产 `open` 模式：逐项指出缺 `IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=1`、`IM_PUBLIC_OPEN_ACK=1` 中的哪一条，词库为空也拒绝（迁移之后读库）。所以要先用 `invite` 模式起来、经后台导入词库，再改 `open`。

配置检查（运维后台"配置检查"页，`GET /internal/v1/config`）：`moderation` 项显示开关、`IM_REPORT_KEY` 已设置 / 未设置（不显示值）、`custom` 打开的告警；`moderation_words` 显示启用的词数，为空时告警（为空时发送不拦截）。

## 2. `IM_REPORT_KEY`

- 生成：32 字节随机值的 base64，例如 `openssl rand -base64 32`。**和 `IM_QUARANTINE_KEY`、JWT 密钥、调用方密钥都要不同**（同一把密钥泄露不能同时打开两类数据）。
- 保管：和 `IM_QUARANTINE_KEY` 一样放在密钥管理里，不进仓库、不进镜像、不写进日志；部署示例只放占位值。多个节点用同一把。
- 备份：密钥和数据库备份**分开保存**（备份里有证据密文，密钥跟着备份走等于没加密）。恢复数据库时要用同一把密钥，否则旧证据读不出来。
- **第一批不支持轮换**（backend-chat 第 99 条）。换了密钥以后，旧证据读取返回 `undecryptable: true`，后台显示"证据无法解密"，`im_report_undecryptable_total` 增加；元数据（举报人、对象、理由）和结案不受影响。只有在密钥泄露时才换，换之前先把待处理的举报处理完。
- 开过 `IM_MOD_ENABLED` 又关掉时**保留这把密钥**：关掉之后已有的举报还要能看证据。

## 3. 调用方配置

示例见 `deploy/im-admin-callers.example.json`：`console` 加 `moderation.words`、`report.read`、`report.handle`；`console-msgread`（`bodyAccess`）加 `report.evidence`。

- `report.evidence` 是正文类 scope，只能在唯一的 `bodyAccess` 调用方上，那个调用方不能有别的非正文 scope；配错（比如 `console` 同时有 `report.evidence` 和 `msg.meta`）im-server 启动失败。
- **已部署环境要手动加**这几个 scope（配置文件不随版本自动更新），然后重启。没加时后台的词库、举报页面都是 403 / 4030。
- 后台角色（console-server）：`moderation.words` 给运营和审核，`report.*` 只给审核，超管默认没有（PRD-admin v1.5 第 5.2 节）。

## 4. 词库运维（CH-19）

- 生产词库**只经后台导入**：词库页读 txt（一行一个，UTF-8），每次最多 5000 条，结果显示新增、重复、无效。不进仓库、不进镜像、不进备份以外的地方（词库在数据库里，随数据库备份）。
- 词库负责人、来源、版权、更新频率由 Frank 指定（CH-19），见第 6 节 ①。
- 变更 10 秒内全部节点生效（失效广播 `blockedWords`，60 秒 TTL 兜底）。匹配前做标准化（全角、大小写、变音符、零宽字符），另去掉空白和标点再匹配一次，会有误判，上线前用实际语料抽查一次。
- 词库为空时不拦截，配置检查告警。

## 5. 运维手册

### 词库加载失败（告警"词库加载失败"）

- 现象：`im_mod_reload_errors_total` 增加，日志 `blocked words reload failed; keeping the previous list`（日志里没有词）。
- 影响：**旧词库继续生效**，新加、停用、删除的词没有生效；进程每 2 秒自动重试，恢复后自动用上新词库。
- 处理：查日志里的错误（数据库是否可读、磁盘、锁），查 `blocked_words` 表能不能读。重启前确认库可读：开关打开时启动加载失败会直接启动失败。
- 告警接到谁由 Frank 指定（第 6 节 ①）。这是 C 批次唯一默认发通知的告警；规则在 `deploy/observability/prometheus/alerts.yml`。

### 举报积压

- `im_reports_pending` 是**提交超过 24 小时还没结案**的举报数（没有标签；查询失败时是 -1）。告警默认不开（没有指定处理人之前没有意义，F-MOD-04 规则 7）；指定处理人以后再按处理时效加。

### 清理任务的检查

- `report_purge`（每小时）：证据到期（`IM_REPORT_EVIDENCE_DAYS`）的物理删除，举报行一起删除，次数表清两天前的。
- `moderation_hits_purge`（每小时）：删 `IM_MOD_HITS_DAYS` 之前的命中记录。
- 检查：`im_job_runs_total{job="report_purge"}`、`{job="moderation_hits_purge"}` 的 `result` 没有持续的 `error`；必要时查 `SELECT MIN(created_at) FROM reports`、`SELECT MIN(at) FROM moderation_hits` 不早于保留期。这两个任务**不跟开关**，关掉开关也会跑。
- `file_purge` 会等证据到期（`files.evidence_until`）之后才删被举报文件的对象；普通用户访问仍按原保留期。

### 其他

- C6 上线前已注册的违规用户名不能改名，由后台封禁处理（F-MOD-02 规则 2）。
- 举报人、被举报人都不收任何通知；第一批不给举报人发处理结果。

## 6. 上线门槛清单（生产打开 `IM_MOD_ENABLED=1` 之前逐条确认，backend-chat 第 93 条）

| # | 条件 | 依据 | 确认人 / 日期 |
|---|---|---|---|
| ① | CH-19：词库负责人、来源、版权、更新已由 Frank 指定；生产词库已经从后台导入；词库加载失败告警的接收人已指定 | PRD-chat 第 20.3 节、F-MOD-01 规则 5 | |
| ② | CH-22：举报处理人已指定，并在管理后台授予 `moderator`（敏感角色，授予时通知审计员） | PRD-chat 第 20.3 节、PRD-admin 第 5.1 节 | |
| ③ | PRD-admin 第 12 节 ②③④ 完成（隐私政策和用户协议发布、个人信息保护影响评估、后台操作规程和培训），**举报证据查看适用** | PRD-chat 第 12.3 节、第 17 节，PRD-admin v1.5 第 12 节 | |
| ④ | `IM_PRIVACY_NOTICE_VERSION` 已配置，隐私说明含 C6 的告知项（`docs/reports/privacy-notice-checklist-draft.md` 第 24～29 项） | AC-ACC-10-5 | |
| ⑤ | `IM_V13_ADMIN_FEATURES=1` | AC-MOD-01-10 | |
| ⑥ | `IM_REPORT_KEY` 已生成、保管、和备份分开存放（第 2 节） | backend-chat 第 99 条 | |
| ⑦ | 调用方 scope 已加（第 3 节），console-server 的角色已配置 | PRD-chat 第 14.1 节 | |
| ⑧ | 生产 `IM_CLIENT_CUSTOM_MSG=0`（默认）；配置检查没有 `custom` 告警 | F-MOD-02 规则 4 | |
| ⑨ | `IM_C2C_BODY_ACCESS` 保持 `off`；打开要 PRD-admin 第 12 节第一行 ①（法务书面确认）。因此单聊举报基本只能按次数判断 | CH-22、R-10 | |
| ⑩ | 三端已发布带 `features.moderation` 的版本（SDK 发布后再开） | PRD-chat 第 17 节 | |
| ⑪ | 管理后台第一批的渗透测试通过 | PRD-admin 第 12 节最后一行，P-13 | |

开放注册 `open` **不在 C6 上线范围**：另要 `IM_PUBLIC_OPEN_ACK=1`、非空词库和 PRD-chat 第 12.3 节的法务手续。
