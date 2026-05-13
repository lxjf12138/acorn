# Capability Node Runtime 产品分析报告

> 版本：v0.1  
> 视角：产品经理 / 产品架构  
> 项目暂定名：Capability Node Runtime / CapNode  
> 技术方向：Go + Kratos，本地可部署能力节点，MCP + Signal + Control/Observation/Governance Surfaces

---

## 0. 一句话定位

**这是一个本地/云端均可部署的能力节点运行时：让 Agent 通过 MCP 调用工具，让外部事件通过 Signal 进入 Agent Runtime，让用户通过控制面看到、审查、管理和恢复所有能力调用。**

它不是单纯的 sandbox，不是单纯的 MCP server，也不是单纯的 OpenClaw 替代品。它更像一个：

> **MCP-native + Signal-aware + Observable + Governed 的 Capability Provider Runtime。**

---

## 1. 背景与机会

### 1.1 当前用户真正想要什么

从 OpenClaw 这类个人 AI assistant 的真实使用场景看，用户并不只是想要一个聊天机器人。他们想要一个能真正做事的常驻助手：

- 从 WhatsApp / Telegram / Slack / Discord / iMessage 等入口发消息。
- 总结邮件、创建草稿、安排日历。
- 操作浏览器、下载文件、填写表单。
- 执行本地 shell、读写文件、跑脚本、部署服务。
- 控制智能家居或企业内部系统。
- 连接自己的知识库、Obsidian、Pinecone、SQLite、文件夹。
- 处理定时任务、webhook、监控告警、下载完成、进程崩溃等外部事件。

这些需求有一个共同点：

> 用户希望 Agent 能够连接真实世界的能力，但又希望这些能力可见、可控、可审查、可恢复。

### 1.2 现有方案的不足

#### 纯 MCP server 的不足

MCP 很适合给模型暴露工具、资源、提示词，但纯 MCP server 通常缺少：

- 本地 UI。
- 权限审批。
- 详细审计。
- 事件触发机制。
- 运行时观测。
- 文件/产物管理。
- sandbox 生命周期管理。
- 多 provider 的统一控制面。
- 插件安全与治理。

#### Python Agent Harness 的不足

当前很多 Agent harness 是库式、进程内、Python-first 的：

- 工具 provider 和 agent runtime 耦合。
- sandbox 不是独立服务边界。
- 控制面和观测不足。
- 本地用户能力难以安全接入云端模型。
- 企业内网工具接入不标准。
- 用户自定义工具缺少统一的审查、审计、UI 和生命周期管理。

#### OpenClaw 类产品的不足

OpenClaw 证明了用户确实需要“能做事的个人 AI assistant”，但也暴露了几个产品风险：

- 技能/插件安全风险高。
- 用户会给 Agent 很深的本地权限。
- 消息入口、工具、事件、sandbox、browser、skill 的边界容易混杂。
- 对普通用户来说，安装、配置、权限理解和事故恢复门槛仍然高。
- 对企业来说，审计、审批、权限、隔离、合规需要更系统的抽象。

我们的机会是把这些能力抽象成一套更清晰的本地能力节点架构。

---

## 2. 产品要解决的问题

### 问题 1：Agent 能聊天，但不能可靠地做事

普通聊天产品的问题是：模型知道该做什么，但无法稳定、安全、可审计地调用用户本地或企业内部能力。

本项目提供：

- MCP Agent Surface：让 Agent 调用工具。
- Provider Runtime：真正执行能力。
- Governance Kernel：统一审查、审批、审计。
- Resource Surface：保留结果、文件、artifact。

---

### 问题 2：本地能力接入云端模型很危险

用户可能希望模型服务在云端，但邮件、浏览器、文件、本地 shell、企业内网工具留在本地。

本项目提供：

- 本地 Capability Node 主动出站连接云端。
- 云端只发起“能力请求”，不直接访问用户主机。
- 本地节点决定是否执行。
- 敏感动作走本地审批。
- 所有调用本地留痕。

---

### 问题 3：MCP 只解决工具调用，不解决外部事件进入 Agent

发送消息可以是 tool call，但接收消息不是 tool call。类似地：

- Telegram 收到消息。
- Gmail 来了新邮件。
- 浏览器下载完成。
- sandbox CPU 过高。
- 文件发生变化。
- 定时任务到点。
- webhook 被触发。

这些都不是 “Agent 主动调用工具”，而是 “外部世界主动进入 Agent Runtime”。

本项目引入：

> **Signal Surface：provider 主动向 runtime 发出 typed signal。**

Signal 进入 Trigger Engine 后，可以：

- 唤醒 agent。
- 插入 session 上下文。
- 调用另一个 MCP tool。
- 请求审批。
- 只更新 UI。
- 只写审计。
- 只聚合为告警。

---

### 问题 4：用户看不见 Agent 正在做什么

Agent 一旦能执行 shell、浏览器、文件、邮件，用户会自然产生这些问题：

- 它现在在跑什么？
- 它改了哪些文件？
- 产物在哪里？
- CPU/GPU 占用是不是异常？
- 哪个工具刚才被调用了？
- 为什么触发了这个动作？
- 这个邮件是不是已经发送？
- 它访问了哪些凭据？
- 出错后如何恢复？

本项目提供：

- Observation Surface：事件、日志、指标、trace、截图、文件变化。
- Control Surface：provider 状态、生命周期、配置、规则。
- Resource Surface：workspace、artifact、快照、下载。
- Audit Surface：所有工具调用、trigger action、审批结果留痕。

---

### 问题 5：工具扩展越多，安全风险越大

OpenClaw 生态里出现过恶意 skill / 插件风险，原因并不是某个单点错误，而是 Agent 工具生态天然具有高风险：

- 插件可能诱导用户执行恶意命令。
- 插件可能读取 SSH key、浏览器密码、钱包文件。
- prompt injection 可能驱使 Agent 调用危险工具。
- 用户经常不理解自己授权了什么。
- 工具 marketplace 容易被滥用。

本项目从架构上把安全作为 Kernel 横切能力：

- Provider manifest 声明风险级别。
- Tool call 先过 PolicyEngine。
- 高风险动作进入 ApprovalManager。
- 所有动作进入 AuditStore。
- Secret 访问独立记录。
- 默认本地节点监听 127.0.0.1。
- 云端通过 outbound connector 请求能力。
- sandbox/host target 明确区分。

---

## 3. 目标用户与使用场景

### 3.1 个人高级用户

特征：

- 想要一个能从 Telegram / WhatsApp / Slack 调用的个人助手。
- 愿意本地部署。
- 有邮件、日历、文件、浏览器、脚本自动化需求。
- 关心隐私，但也愿意使用云端模型。

关键价值：

- 一个聊天入口连接个人数字生活。
- 本地工具由本地节点执行。
- 敏感动作可审批。
- 执行状态可见。
- 文件和产物可下载。

典型场景：

> 用户在 Telegram 里说：“帮我总结今天的重要邮件，然后给老板起草一个回复。”  
> Messaging Provider 收到消息发出 Signal，Trigger Engine 唤醒 Agent。Agent 调用 mail.search、mail.read_thread、mail.create_draft。草稿进入 Approval UI。用户本地确认后，Agent 调用 mail.send_draft。

---

### 3.2 开发者 / 运维用户

特征：

- 经常跑脚本、查日志、部署服务、管理服务器。
- 需要 sandbox、workspace、artifact、shell、SSH。
- 希望 Agent 能执行任务，但必须可见可控。

关键价值：

- 每个 session 有持久 workspace。
- execution environment 可重启、可替换。
- stdout/stderr、process、CPU/GPU、文件变化可见。
- 产物可下载。
- 高风险 shell 命令可审批。

典型场景：

> 用户说：“在这个 repo 里跑测试，修掉失败的 case，然后生成报告。”  
> Agent 调用 sandbox.bash、workspace.read/write。UI 同时展示测试日志、文件 diff、CPU 使用率、outputs/report.md。任务结束后 artifact 可下载。

---

### 3.3 企业团队

特征：

- 有内网系统、工单、数据库、知识库、DevOps 平台。
- 不希望云端模型直接访问内网。
- 需要权限、审计、审批、合规。

关键价值：

- 企业内网部署 local node / enterprise node。
- 内部工具以 provider 形式注册。
- 云端 runtime 只能通过 connector 请求能力。
- 本地策略决定是否执行。
- 审计日志可导出。
- 支持自定义 provider 和已有 MCP server。

典型场景：

> 企业部署一个 Enterprise Capability Node。它暴露 ticket.search、ticket.update、kb.query、deploy.plan、deploy.execute。Agent 可以生成部署计划，但真正执行 deploy.execute 需要双人审批。

---

### 3.4 第三方工具开发者

特征：

- 想给 Agent 提供某种能力。
- 不想自己实现完整 UI、权限、审计、云端连接。
- 可能已有 HTTP/gRPC/MCP 服务。

关键价值：

- 用 provider SDK 注册能力。
- 自动获得 MCP Agent Surface。
- 自动接入控制面、事件、审计、审批。
- 支持纯 trigger provider。
- 支持已有 MCP server adapter。

---

## 4. 产品体验设计

### 4.1 第一次安装体验

目标：让用户理解“本地节点是能力边界”。

推荐体验：

1. 用户下载单个二进制。
2. 运行：

   ```bash
   capnode serve
   ```

3. 浏览器自动打开：

   ```text
   http://127.0.0.1:17890
   ```

4. UI 引导用户完成：

   - 选择运行模式：完全本地 / 连接云端。
   - 绑定账号或跳过。
   - 启用 provider：sandbox、filesystem、browser、mail、calendar、messaging。
   - 配置默认权限：只读 / 需要审批 / 自动允许。
   - 创建第一个 workspace。
   - 测试一次 tool call。
   - 看到第一条 audit log。

体验目标：

> 用户必须在 5 分钟内理解：我授权了哪些能力、它们能做什么、在哪里看日志、如何暂停。

---

### 4.2 日常使用体验

用户日常不会一直打开控制台。最自然的入口可能是：

- Telegram / WhatsApp / Slack。
- Cloud chat UI。
- 本地 Web UI。
- CLI。
- 企业内部 IM。

典型链路：

```text
外部消息
  -> Signal
  -> Trigger Engine
  -> Agent Runtime
  -> MCP Tool Calls
  -> Provider Runtime
  -> Event/Audit/Artifact
  -> 用户收到结果
```

用户在聊天里只看到简洁结果：

> “我已经总结了 12 封邮件，发现 3 封需要你回复。我起草了两个草稿，需要你确认。”

用户在本地 UI 里可以看到完整细节：

- 读了哪些邮件。
- 调用了哪些工具。
- 哪些草稿等待审批。
- 哪个 provider 执行的。
- 是否有敏感权限。
- 结果 artifact 在哪里。

---

### 4.3 控制台体验

控制台不是聊天界面，而是能力节点仪表盘。

主要页面：

#### Providers

- 已启用 provider。
- 健康状态。
- MCP tools 列表。
- Signal types 列表。
- 权限范围。
- 最近调用。

#### Sessions

- 当前会话。
- 绑定 workspace。
- 绑定 provider。
- 当前 agent 状态。
- pending signals。
- 最近 tool calls。

#### Workspace

- 文件树。
- uploads。
- outputs。
- artifacts。
- snapshot。
- diff。
- 下载。

#### Observability

- 事件流。
- stdout/stderr。
- CPU/GPU/memory。
- browser screenshot。
- network/console。
- internal service calls。
- OTel trace link。

#### Approvals

- 待审批 tool call。
- 待发送邮件。
- 待点击网页按钮。
- 待执行 shell 命令。
- 待访问敏感凭据。
- 历史审批记录。

#### Audit

- 所有 tool call。
- 所有 trigger action。
- 所有 policy decision。
- 所有 credential access。
- 所有 provider state changes。

#### Rules

- Signal -> Action 规则。
- 触发条件。
- 去重 / 限流 / cooldown。
- 是否唤醒 agent。
- 是否调用 MCP tool。
- 是否只通知 UI。

---

### 4.4 审批体验

审批体验必须低摩擦，但清晰。

审批卡片应该显示：

- 谁触发了动作。
- 哪个 provider。
- 哪个 tool。
- 参数摘要。
- 风险等级。
- 预期副作用。
- 相关资源链接。
- Policy reason。
- 允许一次 / 永久允许 / 拒绝 / 修改后允许。

例子：

```text
Agent 请求执行：
sandbox.bash("rm -rf ./dist && npm run build")

风险：
filesystem_write + shell_execution

影响范围：
workspace://session-123

建议：
允许。命令仅作用于当前 workspace。
```

又如：

```text
Agent 请求发送邮件：
To: boss@company.com
Subject: 本周项目进展

风险：
external_communication

建议：
需要用户审阅草稿。
```

---

## 5. 核心产品抽象

### 5.1 Capability Provider

Provider 不是狭义工具，而是能力运行时。

```text
Capability Provider =
  Agent Surface
  Signal Surface
  Control Surface
  Observation Surface
  Governance Integration
  Resource Surface
  Runtime Driver
```

不同 provider 可以只实现部分 surface。

---

### 5.2 Agent Surface：MCP

给 Agent 调用的能力。

例子：

- sandbox.bash
- workspace.read
- workspace.write
- browser.open
- browser.act
- mail.search
- mail.create_draft
- message.send
- calendar.create_event
- kb.query

产品原则：

- 简单。
- 稳定。
- 适合模型理解。
- 不承载完整控制面。
- 返回结构化、简洁结果。

---

### 5.3 Signal Surface

Signal 是 provider 主动发出的外部事实。

例子：

- channel.message.received
- mail.message.received
- browser.download.completed
- browser.console.error
- sandbox.cpu.high
- workspace.artifact.created
- file.modified
- schedule.tick
- webhook.received

Signal 可以触发：

- 唤醒 Agent。
- 插入上下文。
- 调用 MCP tool。
- 请求审批。
- 通知 UI。
- 只进入审计。
- 只聚合为告警。

产品原则：

- Signal 不等于一定发给模型。
- Signal 要经过 TriggerEngine。
- 高频 observation 不能直接进入 Agent。
- Signal 要支持去重、限流、合并、上下文策略。

---

### 5.4 Control Surface

给 UI 和控制面使用。

例子：

- provider enable/disable。
- provider config。
- browser start/stop。
- environment restart/sleep。
- workspace snapshot/restore。
- channel connect/disconnect。
- rule management。
- account binding。

---

### 5.5 Observation Surface

给用户看见系统正在发生什么。

例子：

- tool call started/completed/failed。
- CPU/GPU/memory。
- stdout/stderr。
- browser screenshot。
- console/network。
- file changes。
- artifact created。
- internal RPC events。
- logs/traces/metrics。

---

### 5.6 Governance Surface

负责安全、审批、审计和策略。

例子：

- policy evaluation。
- approval request。
- audit log。
- credential access record。
- plugin risk classification。
- provider permission scopes。

---

### 5.7 Resource Surface

负责文件和产物。

例子：

- workspace files。
- uploads。
- outputs。
- artifacts。
- screenshots。
- browser traces。
- logs。
- PDF/CSV/patch。
- snapshots。
- downloads。

---

## 6. 关键用户流程

### 6.1 消息入口触发任务

```text
用户发 Telegram 消息
  -> Messaging Provider 收到消息
  -> Emit Signal: channel.message.received
  -> Trigger Engine 路由到对应 session
  -> Context Inbox 插入 user_message
  -> Agent Runtime 被唤醒
  -> Agent 调用 MCP tools
  -> message.send 返回结果
  -> Audit/Event 记录全链路
```

用户体验：

- 聊天里看到自然回复。
- 控制台里看到完整调用链。

---

### 6.2 浏览器下载触发 workspace 导入

```text
Browser Provider 检测下载完成
  -> Signal: browser.download.completed
  -> Trigger Rule: import-browser-download
  -> Action Executor 调用 workspace.import_file
  -> Artifact created
  -> UI 文件树更新
```

用户体验：

- 不需要 Agent 主动轮询。
- 下载完成后自动进入 workspace。
- 用户可以下载、预览、引用。

---

### 6.3 Sandbox CPU 过高触发告警

```text
Metrics Collector 检测 CPU > 95% 持续 2 分钟
  -> Signal: sandbox.cpu.high
  -> Trigger Engine
  -> Context Inbox 插入 system_observation
  -> 如果 session 正在运行任务，唤醒 Agent
  -> Agent 决定是否查看进程或终止任务
```

用户体验：

- UI 实时显示 CPU 异常。
- Agent 可以被提醒。
- 是否 kill process 取决于策略和审批。

---

### 6.4 邮件草稿审批

```text
Agent 调用 mail.create_draft
  -> Provider Runtime 创建草稿
  -> ApprovalManager 创建审批卡片
  -> 用户在本地 UI 或聊天里确认
  -> Agent/Runtime 调用 mail.send_draft
  -> Audit 记录
```

用户体验：

- Agent 帮你写，但不会悄悄发。
- 用户可编辑、批准、拒绝。
- 每次发送有记录。

---

## 7. 与现有方案的差异化

### 7.1 相对纯 MCP server

纯 MCP server：

- 给 Agent 暴露工具。
- 通常缺少完整 UI、Signal、审批、观测、资源管理。

本项目：

- MCP 只是 Agent Surface。
- Signal 让外部世界进入 runtime。
- Control/Observation/Governance/Resource 完整产品化。
- Provider 统一注册和治理。

---

### 7.2 相对 OpenClaw

OpenClaw 证明了“聊天入口 + 本地 gateway + 工具执行”的用户价值。

本项目的差异化：

- 更清晰的 provider surface 抽象。
- Signal Surface 一等建模。
- 统一 TriggerEngine。
- 更强治理内核。
- 面向企业和自定义 provider 的 Kratos 服务化架构。
- workspace/environment 分离。
- 可以把 sandbox、browser、messaging、monitor、webhook 都统一在同一机制下。

---

### 7.3 相对 Python Agent Harness

Python harness：

- 更适合开发者快速构建 agent。
- 但通常不是本地能力节点产品。

本项目：

- Go/Kratos 服务化。
- 本地可长期运行。
- 控制面、事件、审批、审计一等。
- provider runtime 可替换。
- 语言无关 provider SDK。

---

## 8. 产品架构

```text
Cloud Agent Runtime / Model
        |
        | MCP calls / Signal stream / Control proxy
        v
Connector Hub
        |
        | outbound tunnel
        v
Local Capability Node
        |
        ├─ MCPGateway
        ├─ SignalIngest
        ├─ TriggerEngine
        ├─ ActionExecutor
        ├─ ProviderManager
        ├─ EventBus
        ├─ PolicyEngine
        ├─ ApprovalManager
        ├─ AuditStore
        ├─ ResourceManager
        ├─ WorkspaceManager
        ├─ CredentialManager
        └─ Providers
             ├─ sandbox
             ├─ browser
             ├─ messaging
             ├─ mail
             ├─ calendar
             ├─ filesystem
             ├─ knowledge
             ├─ monitor
             ├─ scheduler
             ├─ webhook
             └─ custom
```

---

## 9. MVP 建议

### Phase 1：Local Node 内核

目标：

- 单二进制 Kratos local node。
- 内嵌 UI。
- ProviderManager。
- MCPGateway。
- SignalIngest。
- EventBus。
- TriggerEngine。
- Policy/Audit 基础版。

内置 provider：

- filesystem。
- sandbox basic。
- system-monitor。
- scheduler。
- webhook。

证明点：

- Agent 能通过 MCP 调 sandbox。
- system-monitor 能发 Signal。
- Trigger 能调用另一个 MCP tool。
- UI 能看到事件、日志、文件、审批。

---

### Phase 2：Workspace + Sandbox 完整体验

目标：

- workspace 持久。
- environment 可重启。
- 文件树。
- outputs/artifacts。
- snapshot/restore。
- stdout/stderr。
- CPU/memory/process。
- 文件下载。

这是第一个强产品闭环。

---

### Phase 3：Messaging Provider

目标：

- Telegram 或 Slack 作为第一个 channel。
- message.received -> Signal。
- message.send -> MCP tool。
- session routing。
- allowlist。
- 消息审计。

证明点：

- 外部消息能自然进入 Agent。
- Signal Surface 成立。

---

### Phase 4：Browser Provider

目标：

- browser.open/snapshot/act/screenshot。
- browser.download.completed Signal。
- screenshot/tabs/console/network UI。
- 权限审批。
- human takeover。

证明点：

- UI 自动化 provider 也能套同一机制。

---

### Phase 5：Custom Provider SDK

目标：

- Go SDK。
- Python SDK。
- TypeScript SDK。
- external HTTP adapter。
- external gRPC provider。
- existing MCP server adapter。
- pure trigger provider。

证明点：

- 生态可以扩展。

---

## 10. 成功指标

### 激活指标

- 安装到首次成功 tool call 的时间。
- 首次连接 provider 成功率。
- 首次 Signal 触发成功率。
- 首次 artifact 下载率。

### 使用指标

- 每日活跃 session。
- 每日 tool call 数。
- 每日 signal 数。
- signal -> action 转化率。
- agent 完成任务率。
- artifact 生成率。
- provider 启用数量。

### 安全指标

- 高风险 tool call 审批率。
- 用户拒绝率。
- policy deny 次数。
- 凭据访问次数。
- 审批平均耗时。
- 被拦截危险动作数。

### 可靠性指标

- provider crash rate。
- tool call failure rate。
- trigger loop detection 次数。
- event delivery latency。
- workspace restore 成功率。
- connector reconnect time。

### 用户体验指标

- 任务完成满意度。
- 控制台使用率。
- 审批卡片理解率。
- 用户是否能回答“Agent 刚才做了什么”。
- 用户是否能恢复失败任务。

---

## 11. 关键产品风险与应对

### 风险 1：用户不理解授权边界

应对：

- Onboarding 明确展示 provider 权限。
- 每个 tool 有风险标签。
- 默认最小权限。
- host/sandbox target 明确区分。
- 高风险动作默认审批。

---

### 风险 2：Signal 太多导致上下文污染

应对：

- Signal 与 Observation 分离。
- 高频事件先进入 observation。
- 规则提升为 signal。
- ContextInbox 合并、去重、摘要。
- 默认不把文件变化/metrics 直接塞进模型。

---

### 风险 3：事件触发工具形成循环

应对：

- causality_id。
- trace_id。
- max_depth。
- dedupe_key。
- cooldown。
- rate limit。
- loop detection。
- Trigger rule 测试器。

---

### 风险 4：插件和自定义 provider 带来安全问题

应对：

- Provider manifest 强制声明风险。
- 外部 provider 默认隔离。
- 自定义 provider 需要用户显式启用。
- Marketplace 需要签名、扫描、来源信誉。
- 所有 tool call 走 PolicyEngine。
- 所有 secret access 记录。

---

### 风险 5：产品太复杂

应对：

- 初始体验只展示三个核心页面：Providers、Sessions、Approvals。
- Advanced 页面折叠。
- 推荐默认策略。
- 用模板配置 provider。
- 用“发生了什么”时间线解释系统。

---

### 风险 6：云端/本地边界不清晰

应对：

- 本地节点主动出站连接云端。
- 云端只请求 capability。
- 本地决定是否执行。
- 本地 UI 永远可暂停。
- 本地保存高敏审计。

---

## 12. 设计原则

1. **Capability Provider 是一等抽象，不是 tool 是一等抽象。**
2. **MCP 是 Agent Surface，不是完整产品协议。**
3. **Signal Surface 是外部事实进入 Agent Runtime 的入口。**
4. **所有 Signal 不一定进入模型。**
5. **Control/Observation/Governance/Resource Surface 服务人和控制面。**
6. **Provider Runtime 处理真实复杂度。**
7. **Provider 之间不直接互调，通过 Signal、Trigger、ActionExecutor 解耦。**
8. **所有 tool call 和 trigger action 都经过 Policy/Audit。**
9. **Workspace 持久，Environment 可替换。**
10. **云端只能请求能力，本地节点决定执行。**
11. **默认可见、可暂停、可恢复。**
12. **安全是产品体验，不是后台配置。**

---

## 13. 推荐产品定位

### 中文定位

> 一个 Go/Kratos 实现的本地能力节点运行时，让 AI Agent 安全、可见、可审查地调用用户本地、云端和企业内网能力。

### 英文定位

> A Go/Kratos-based Capability Node Runtime that exposes tools to agents via MCP, ingests external facts via Signals, and provides control, observability, governance, and resource management for local and enterprise automation.

---

## 14. 推荐首发叙事

不要说：

> 我们做了一个 sandbox。

应该说：

> 我们做了一个本地能力节点，让 AI 不只是聊天，而是能安全地连接你的电脑、浏览器、文件、消息、邮件、日历和内部系统。Agent 通过 MCP 调工具，外部事件通过 Signal 唤醒 Agent，用户通过本地控制台看到和审查一切。

---

## 15. 参考资料

1. OpenClaw 官方站点：<https://openclaw.ai/>
2. OpenClaw Personal Assistant setup：<https://docs.openclaw.ai/start/openclaw>
3. OpenClaw Gateway / Security docs：<https://docs.openclaw.ai/gateway/security>
4. DigitalOcean OpenClaw marketplace docs：<https://docs.digitalocean.com/products/marketplace/catalog/openclaw/>
5. MCP Specification 2025-11-25：<https://modelcontextprotocol.io/specification/2025-11-25>
6. MCP Sampling：<https://modelcontextprotocol.io/specification/2025-11-25/client/sampling>
7. OpenClaw use cases collection：<https://openclaw.rocks/blog/openclaw-use-cases>
8. Mario Hayashi: AI Personal Assistant with OpenClaw：<https://blog.mariohayashi.com/p/i-set-up-an-ai-personal-assistant>
9. The Verge: OpenClaw skill extension security risks：<https://www.theverge.com/news/874011/openclaw-ai-skill-clawhub-extensions-security-nightmare>
10. TechRadar: OpenClaw infostealer risk：<https://www.techradar.com/pro/security/openclaw-ai-agents-targeted-by-infostealer-malware-for-the-first-time>

---

## 16. 最终结论

这个项目的产品价值不在于“又做了一个 MCP server”或“又做了一个 sandbox”。

它真正解决的是：

> 当 Agent 开始连接真实世界能力时，用户需要一个本地能力边界：既能让 Agent 做事，又能让人看到、控制、审查、恢复和治理这些动作。

因此，产品核心应该是：

```text
Capability Node Runtime
  = MCP Agent Surface
  + Signal Surface
  + Trigger Engine
  + Control Surface
  + Observation Surface
  + Governance Surface
  + Resource Surface
  + Provider Runtime
```

这套抽象能够覆盖个人助手、开发者自动化、企业内网工具、浏览器自动化、sandbox workspace、消息入口、监控告警、定时任务和自定义 provider。它是一个可以扩展的产品底座，而不是单点工具。
