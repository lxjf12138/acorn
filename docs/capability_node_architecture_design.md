# 分布式 Agent 能力节点平台：整体架构设计文档

> 版本：v0.1  
> 日期：2026-05-13  
> 技术栈建议：Go + Kratos  
> 核心架构：Cloud Agent Control Plane + many Capability Nodes

---

## 1. 项目定位

本项目不是一个单纯的 sandbox runtime，也不是一个普通 MCP server，而是一个面向 Agent 的 **分布式能力运行时与控制面**。

一句话定义：

> 一个云端 Agent 控制面，连接和编排多个 Capability Node；Capability Node 可以部署在用户本地、企业内网、云端或边缘环境；Agent 通过 MCP 调用能力，节点通过 Signal 将外部事件、状态和触发输入送回控制面。

核心结构：

```text
Cloud Agent Control Plane
        |
        | Connector / Tunnel
        |
many Capability Nodes
        |
Provider Runtimes
```

---

## 2. 核心目标

### 2.1 产品目标

解决的问题：

1. Agent 需要安全使用用户本地、企业内网、云端和自定义工具能力。
2. 现有很多 agent harness 工具是 Python 进程内架构，缺少服务化、控制面、观测和审计能力。
3. 普通 MCP server 可以暴露工具，但通常缺少：
   - 本地/企业节点治理；
   - 工具调用审查；
   - 审计记录；
   - Signal/Trigger 机制；
   - 工作区、artifact、日志、指标、UI 观测；
   - 多节点统一注册和路由。
4. 用户希望能力可以本地化部署，但模型和 Agent Runtime 可以在云端运行。
5. 企业希望内网能力不暴露公网，由本地节点主动出站连接云端控制面。

### 2.2 技术目标

1. 用统一的 Capability Provider 抽象覆盖 sandbox、browser、filesystem、mail、calendar、messaging、knowledge、enterprise tools、custom provider。
2. 兼容普通 MCP 服务，并提供增强的 Native Capability Provider 模型。
3. 将 Agent 调用和外部触发分开：
   - MCP：Agent 主动调用能力；
   - Signal：外部世界或 Provider Runtime 主动输入事实；
   - TriggerEngine：Signal 到动作、上下文或 Agent 唤醒的编排。
4. 将控制面能力从 agent-facing tool 中分离出来：
   - HTTP/gRPC：控制、配置、生命周期、审批、资源；
   - Event/Stream：观测、日志、指标、trace、artifact；
   - Governance：policy、approval、audit。
5. 用 Go + Kratos 作为统一服务底座，支持个人轻量部署、企业部署和云端托管部署。

---

## 3. 整体架构

```text
┌─────────────────────────────────────────────────────────────────────┐
│                     Cloud Agent Control Plane                        │
│                                                                     │
│  ┌──────────────────────┐     ┌──────────────────────────────────┐  │
│  │ Model Service         │     │ Agent Runtime / Harness           │  │
│  │ - cloud LLM           │     │ - agent template                  │  │
│  │ - local model proxy   │     │ - session runtime                 │  │
│  │ - model router        │     │ - context builder                 │  │
│  └──────────────────────┘     │ - MCP tool router                 │  │
│                               │ - signal consumer                 │  │
│                               └───────────────┬──────────────────┘  │
│                                               │                     │
│  ┌──────────────────────┐     ┌───────────────▼──────────────────┐  │
│  │ Cloud Console         │     │ Connector Hub / Node Registry     │  │
│  │ - sessions            │     │ - node registration               │  │
│  │ - nodes/providers     │     │ - capability registry             │  │
│  │ - approvals/audit     │     │ - tunnel multiplexing             │  │
│  │ - events/artifacts    │     │ - heartbeat                       │  │
│  └──────────────────────┘     └───────────────┬──────────────────┘  │
│                                               │                     │
│  ┌──────────────────────┐     ┌───────────────▼──────────────────┐  │
│  │ Global Policy/Audit   │     │ Signal Router / Trigger Engine    │  │
│  │ - org policy          │     │ - signal routing                  │  │
│  │ - risk model          │     │ - session matching                │  │
│  │ - global audit        │     │ - wake agent / call tool          │  │
│  └──────────────────────┘     └──────────────────────────────────┘  │
└──────────────────────────────────────┬──────────────────────────────┘
                                       │
                                       │ outbound connector / grpc stream
                                       │
┌──────────────────────────────────────▼──────────────────────────────┐
│                         Capability Node                              │
│              local / cloud / enterprise / edge deployment            │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │ Node Kernel                                                    │  │
│  │                                                               │  │
│  │  MCP Server / MCP Adapter                                      │  │
│  │  Signal Emitter / optional Local Signal Ingest                 │  │
│  │  Provider Manager                                              │  │
│  │  Node Control API                                              │  │
│  │  EventBus / Observation Stream                                 │  │
│  │  Local Policy / Approval / Audit                               │  │
│  │  Resource / Artifact Manager                                   │  │
│  │  Workspace Manager                                             │  │
│  │  Credential Manager                                            │  │
│  │  Connector Client                                              │  │
│  │  Embedded Local UI                                             │  │
│  └───────────────────────────────┬───────────────────────────────┘  │
│                                  │                                  │
│  ┌───────────────────────────────▼───────────────────────────────┐  │
│  │ Providers                                                      │  │
│  │                                                               │  │
│  │  sandbox        browser        filesystem                      │  │
│  │  messaging      mail           calendar                        │  │
│  │  knowledge      monitor        scheduler                       │  │
│  │  webhook        smart-home     enterprise-tools                │  │
│  │  custom-http    custom-grpc    existing-mcp-adapter            │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. 核心组件职责

### 4.1 Cloud Agent Control Plane

Cloud Agent Control Plane 是整个系统的大脑。

职责：

- 用户、组织、租户管理；
- Agent Template 管理；
- Session / Conversation 管理；
- Agent Runtime / Harness；
- Model Router；
- Context Builder；
- MCP Tool Router；
- Signal Router；
- 主 Trigger Engine；
- Capability Registry；
- Node Registry；
- Connector Hub；
- Global Policy；
- Audit Aggregator；
- Cloud Console。

它决定：

- 当前 session 使用哪个 agent；
- agent 可以访问哪些节点和 provider；
- 收到 signal 后是否唤醒 agent；
- signal 如何注入上下文；
- agent 的 tool call 路由到哪个 Capability Node；
- 高风险动作是否需要审批；
- 审计和观测数据如何展示。

### 4.2 Capability Node

Capability Node 是能力执行和治理节点。

它可以部署在：

- 用户个人电脑；
- 企业内网服务器；
- 云端托管环境；
- K8s 集群；
- 边缘节点；
- 开发者 VPS。

职责：

- 托管 providers；
- 暴露 MCP tools/resources/prompts；
- 兼容普通 MCP servers；
- 向云端控制面上报 signals；
- 提供 node-level control API；
- 提供本地 UI；
- 管理本地 workspace、artifacts、credentials；
- 执行本地 policy、approval、audit；
- 采集本节点事件、日志、指标和状态；
- 通过 connector 主动连接云端控制面。

### 4.3 Provider Runtime

Provider Runtime 是真实能力的执行实现。

例子：

- sandbox provider：Docker/local/SSH/K8s/remote runtime；
- browser provider：Playwright/CDP/host browser/sandbox browser；
- messaging provider：Telegram/Slack/WhatsApp/Discord；
- mail provider：Gmail/IMAP/Outlook；
- enterprise provider：企业 HTTP/gRPC/DB/internal API；
- mcp-adapter provider：已有 MCP server；
- trigger provider：scheduler、webhook、monitor、file watcher。

---

## 5. 核心抽象：Capability Provider

### 5.1 Provider 不是 Tool

本项目的一等抽象不是 Tool，而是 Capability Provider。

Tool 只是 Provider 给 Agent 暴露的一种 Agent Surface。

```text
Capability Provider
  ├─ Agent Surface
  ├─ Signal Surface
  ├─ Control Surface
  ├─ Observation Surface
  ├─ Resource Surface
  ├─ Governance Hook
  └─ Runtime Driver
```

### 5.2 Provider Surface

#### Agent Surface

面向 Agent，默认使用 MCP。

用于表达：

- Agent 可以调用哪些工具；
- 参数 schema；
- 返回结果；
- resources；
- prompts。

例子：

```text
sandbox.bash
workspace.read
workspace.write
browser.open
browser.act
mail.search
mail.create_draft
message.send
calendar.create_event
```

设计原则：

- 简单；
- 稳定；
- request/response；
- 适合模型理解；
- 不承载完整控制面复杂度。

#### Signal Surface

面向 Agent Runtime 和 TriggerEngine。

用于表达外部事实输入：

```text
channel.message.received
mail.message.received
browser.download.completed
browser.console.error
workspace.file.modified
sandbox.cpu.high
sandbox.process.exited
schedule.tick
webhook.github.pr_opened
```

Signal 不一定进入模型。它先进入控制面，由 TriggerEngine 决定：

- 是否忽略；
- 是否只持久化；
- 是否通知 UI；
- 是否插入 ContextInbox；
- 是否唤醒 Agent；
- 是否触发另一个 MCP tool；
- 是否请求审批。

#### Control Surface

面向 Cloud Console、Local UI、管理员和控制面。

负责：

- provider 启停；
- provider 配置；
- health；
- workspace lifecycle；
- environment restart/sleep/destroy；
- browser profiles；
- channel account connect/disconnect；
- trigger rules；
- approval handling；
- credential configuration。

通过 HTTP/gRPC 暴露。

#### Observation Surface

面向人、控制面和监控系统。

负责：

- events；
- logs；
- metrics；
- traces；
- tool call 状态；
- stdout/stderr；
- CPU/GPU/memory；
- browser screenshot stream；
- workspace file changes；
- internal rpc events。

通过：

- SSE；
- WebSocket；
- gRPC stream；
- HTTP query；
- OpenTelemetry export。

#### Resource Surface

负责：

- workspace files；
- uploads；
- outputs；
- artifacts；
- screenshots；
- logs；
- browser traces；
- PDFs；
- CSVs；
- patches；
- snapshots；
- downloadable bundles。

#### Governance Hook

所有 Provider 调用都要接入治理链路：

```text
MCP tool call / Trigger action
  -> PolicyEngine
  -> ApprovalManager
  -> Provider Runtime
  -> AuditStore
  -> EventBus
```

---

## 6. MCP 兼容设计

### 6.1 普通 MCP 服务作为一等接入方式

系统必须兼容普通 MCP server。

普通 MCP server 不需要改写成 Native Provider，而是由 Capability Node 使用 `MCPAdapterProvider` 包装。

```text
Existing MCP Server
        |
        | stdio / http / sse / streamable http
        v
Capability Node: MCPAdapterProvider
        |
        v
ProviderManager
        |
        ├─ Agent Surface
        ├─ Control Surface
        ├─ Observation Surface
        └─ Governance Surface
```

### 6.2 MCPAdapterProvider 职责

- 启动或连接普通 MCP server；
- 调用 tools/list、resources/list、prompts/list；
- 自动生成基础 ProviderManifest；
- 将普通 MCP tools 纳入统一 ToolRouter；
- 给每次 tool call 套上 policy、approval、audit；
- 提供 health、logs、enable/disable、tool visibility；
- 允许用户为 MCP tools 补充 risk、requires_approval、timeout、allowed_sessions 等配置。

### 6.3 支持的 MCP 接入方式

#### stdio MCP server

```yaml
providers:
  - id: filesystem
    type: mcp
    transport: stdio
    command: "npx"
    args:
      - "@modelcontextprotocol/server-filesystem"
      - "/Users/emma/projects"
```

#### remote MCP server

```yaml
providers:
  - id: company-tools
    type: mcp
    transport: http
    url: "https://mcp.company.internal/mcp"
```

#### managed MCP preset

```yaml
providers:
  - id: postgres
    type: mcp
    preset: postgres
    env:
      DATABASE_URL: "${secret:db_url}"
```

### 6.4 普通 MCP 与 Native Provider 的差异

| 能力 | 普通 MCP Server | Native Capability Provider |
|---|---|---|
| Agent tools | 支持 | 支持 |
| Resources/prompts | 支持 | 支持 |
| Signal | 默认不支持 | 原生支持 |
| Control API | 弱，由 adapter 包装 | 强 |
| Observation | Adapter 统一包装 | 原生丰富 |
| Policy/Audit | 外层补齐 | 原生集成 |
| Workspace/artifact | 不统一 | 可深度集成 |
| Lifecycle | Adapter 管理 | Provider 自己管理 |
| UI panel | 通用面板 | 可定制面板 |

定位：

> MCP 是最低成本接入方式；Native Provider 是深度集成方式。

---

## 7. Signal 与 Trigger 设计

### 7.1 为什么需要 Signal

MCP 解决的是：

```text
Agent -> Provider
```

Signal 解决的是：

```text
External World / Provider Runtime -> Agent Runtime
```

典型场景：

- Telegram 收到消息；
- Gmail 收到新邮件；
- sandbox CPU 过高；
- browser 下载完成；
- 文件变化；
- 定时任务到点；
- GitHub webhook；
- 企业监控告警；
- 审批结果。

### 7.2 Signal Envelope

建议兼容 CloudEvents 风格。

```json
{
  "id": "sig_123",
  "source": "provider://telegram/main",
  "type": "channel.message.received",
  "subject": "telegram/chat/123/message/456",
  "time": "2026-05-13T12:00:00Z",

  "node_id": "node_local_1",
  "provider_id": "telegram",
  "session_hint": "channel:telegram:chat-123",

  "input_kind": "user_message",
  "severity": "info",
  "priority": "normal",
  "dedupe_key": "telegram:chat-123:456",
  "causality_id": "cause_abc",
  "trace_id": "trace_xyz",

  "data": {
    "from": "emma",
    "text": "帮我总结今天的邮件"
  }
}
```

### 7.3 Signal 类型

```text
user_message
system_observation
runtime_alert
external_event
scheduled_event
resource_event
approval_event
```

### 7.4 Observation Event 与 Signal 的区别

```text
所有 Signal 都可以记录为 Event。
但不是所有 Event 都应该成为 Signal。
```

例子：

- `environment.metrics.updated` 是高频 observation event；
- `sandbox.cpu.high` 是经过规则提升后的 agent-level signal。

### 7.5 TriggerEngine

TriggerEngine 是 Signal 到动作的编排器。

主 TriggerEngine 应该在 Cloud Agent Control Plane 中，因为它需要知道：

- session；
- agent template；
- context；
- node/provider registry；
- global policy；
- model/runtime 状态。

Capability Node 可以有轻量 Local TriggerEngine，用于：

- 本地过滤；
- 本地合并；
- 本地安全阻断；
- 本地 UI 通知；
- 离线模式。

Trigger action：

```text
ignore
persist_only
notify_ui
append_context
wake_agent
start_isolated_turn
call_mcp_tool
request_approval
emit_signal
create_task
coalesce
```

示例：消息触发 agent。

```yaml
name: telegram-message
when:
  type: channel.message.received
then:
  - action: append_context
    session: "channel:telegram:{{data.chat_id}}"
    input_kind: user_message
    content: "{{data.text}}"
  - action: wake_agent
```

示例：浏览器下载触发 workspace import。

```yaml
name: import-browser-download
when:
  type: browser.download.completed
then:
  - action: call_mcp_tool
    tool: workspace.import_file
    args:
      source: "{{data.path}}"
      dest: "/downloads/{{data.filename}}"
  - action: notify_ui
```

示例：sandbox CPU 告警。

```yaml
name: sandbox-cpu-alert
when:
  type: sandbox.cpu.high
  duration: 2m
then:
  - action: append_context
    session: "{{scope.session_id}}"
    input_kind: system_observation
    content: "Sandbox CPU has been above 95% for 2 minutes."
  - action: wake_agent
    condition: "session.running_task == true"
```

### 7.6 防循环机制

Signal -> Tool -> Signal 可能产生循环。

必须支持：

```text
dedupe_key
causality_id
parent_event_id
trace_id
max_depth
rate_limit
cooldown
coalesce_window
loop_detection
```

---

## 8. Session 与 Context 设计

### 8.1 Session

Session 是 Agent 运行上下文。

```text
Session
  ├─ transcript
  ├─ context_inbox
  ├─ workspace
  ├─ active_runs
  ├─ bound_nodes
  ├─ bound_providers
  ├─ policy_context
  └─ resource_links
```

### 8.2 ContextInbox

不要将所有 Signal 直接塞进对话 transcript。

每个 session 有一个 ContextInbox，存放：

- pending signals；
- coalesced observations；
- resource links；
- priority；
- consumed/unconsumed state；
- event summaries；
- approval results。

Agent turn 开始时：

```text
ContextBuilder
  -> load pending signals
  -> redact
  -> summarize / coalesce
  -> attach resource links
  -> build model messages
```

不同类型 Signal 的注入方式：

| Signal 类型 | 注入方式 |
|---|---|
| user_message | user message |
| system_observation | runtime observation |
| runtime_alert | high priority system observation |
| resource_event | summary + resource link |
| approval_event | wake waiting state machine |
| scheduled_event | isolated turn 或 scheduled session turn |

---

## 9. Provider 设计示例

### 9.1 Sandbox Provider

核心模型：

```text
Workspace 持久
Environment 可替换
```

```text
Session
  -> Workspace
       /workspace
       /uploads
       /outputs
       /.metadata
       snapshots
       artifacts

Environment
  -> env_id
  -> workspace_id
  -> driver: local/docker/ssh/k8s/remote
  -> status: running/sleeping/error/stopped
```

Agent Surface：

```text
sandbox.bash
workspace.read
workspace.write
workspace.list
workspace.grep
workspace.glob
```

Control Surface：

```text
workspace.download
workspace.snapshot
workspace.restore
environment.restart
environment.sleep
environment.destroy
process.list
process.kill
```

Signal Surface：

```text
sandbox.cpu.high
sandbox.memory.high
sandbox.process.exited
workspace.artifact.created
workspace.file.modified
```

Observation Surface：

```text
stdout/stderr
metrics
process list
workspace events
artifact list
file tree
```

### 9.2 Browser Provider

Agent Surface：

```text
browser.open
browser.tabs
browser.snapshot
browser.screenshot
browser.act
browser.console
```

Signal Surface：

```text
browser.download.completed
browser.console.error
browser.network.failed
browser.tab.changed
browser.dialog.opened
```

Control Surface：

```text
profiles
start/stop
tabs
current screenshot
console
network
trace
permissions
human takeover
```

Runtime Driver：

```text
Playwright
CDP
host browser
sandbox browser
remote browser
```

### 9.3 Messaging Provider

Messaging 是双向 provider。

入站：

```text
external channel -> Signal
```

出站：

```text
Agent -> MCP tool -> message.send
```

Agent Surface：

```text
message.send
message.read_thread
message.list_threads
message.mark_read
```

Signal Surface：

```text
channel.message.received
channel.message.sent
channel.thread.updated
channel.delivery.failed
```

Control Surface：

```text
connect account
allowlist contacts
allowlist groups
auto-reply policy
message audit
delivery status
```

Runtime：

```text
Telegram
WhatsApp
Slack
Discord
iMessage
Email gateway
Webhook
```

### 9.4 Pure Trigger Provider

例子：

- system monitor；
- scheduler；
- webhook；
- file watcher；
- external alert receiver。

它可以没有 Agent Surface。

```json
{
  "id": "system-monitor",
  "type": "trigger",
  "agent_surface": null,
  "signal_surface": {
    "emits": [
      "system.cpu.high",
      "system.memory.high",
      "system.disk.low",
      "process.crashed"
    ]
  },
  "control_surface": {
    "features": [
      "thresholds",
      "rules",
      "history"
    ]
  },
  "observation_surface": {
    "metrics": [
      "cpu.percent",
      "memory.bytes",
      "disk.bytes"
    ]
  }
}
```

---

## 10. Provider Manifest

每个 provider 必须声明自身能力。

```json
{
  "id": "local-browser",
  "type": "browser",
  "version": "0.1.0",
  "display_name": "Local Browser",

  "agent_surface": {
    "protocol": "mcp",
    "tools": [
      {
        "name": "browser.open",
        "risk": "network_navigation",
        "side_effect": "read"
      },
      {
        "name": "browser.act",
        "risk": "ui_action",
        "side_effect": "write",
        "requires_approval": "policy"
      }
    ]
  },

  "signal_surface": {
    "emits": [
      {
        "type": "browser.download.completed",
        "input_kind": "resource_event"
      },
      {
        "type": "browser.console.error",
        "input_kind": "system_observation"
      }
    ]
  },

  "control_surface": {
    "features": [
      "profiles",
      "tabs",
      "screenshots",
      "console",
      "network",
      "trace",
      "permissions"
    ]
  },

  "observation_surface": {
    "events": [
      "tool.call.*",
      "browser.tab.changed",
      "browser.console.*"
    ],
    "metrics": []
  },

  "security": {
    "targets": ["sandbox", "host"],
    "default_target": "sandbox"
  }
}
```

普通 MCP server 自动生成基础 Manifest，用户可用配置补充治理信息：

```yaml
providers:
  - id: github-mcp
    type: mcp
    command: "npx @modelcontextprotocol/server-github"
    tools:
      github.create_issue:
        risk: write_external
        requires_approval: false
      github.merge_pr:
        risk: destructive
        requires_approval: true
      github.delete_branch:
        risk: destructive
        requires_approval: true
```

---

## 11. Kratos 技术选型

### 11.1 为什么选择 Kratos

Kratos 适合本项目的原因：

- Go 单二进制部署；
- HTTP/gRPC transport；
- Protobuf-first；
- 中间件；
- 配置；
- 日志；
- metrics；
- OpenTelemetry；
- 服务生命周期；
- registry 扩展；
- 工程化良好。

本项目不是因为“要做微服务”才用 Kratos，而是需要一个统一的 Go 服务底座。

### 11.2 不要过早微服务化

早期建议服务拆分：

```text
services/
  agent-control-plane/
  connector-hub/
  capability-node/
```

每个服务内部模块化，不要一开始拆成：

```text
provider-service
signal-service
trigger-service
audit-service
workspace-service
approval-service
...
```

### 11.3 Capability Node 的个人轻量 Profile

个人本地 Node 必须做到：

- 单二进制；
- 单进程；
- 一个本地端口；
- 默认只监听 127.0.0.1；
- 内嵌静态 UI；
- SQLite/bbolt 本地状态；
- 无需注册中心；
- 无需配置中心；
- 无需 K8s；
- provider 按需启用。

命令：

```bash
capnode serve --profile personal
```

企业和云端可以启用更完整功能：

```bash
capnode serve --profile enterprise
capnode serve --profile cloud
```

### 11.4 Core 与 Kratos Transport 解耦

业务核心不应绑定 Kratos handler。

建议：

```text
capability-node
  ├─ core kernel
  │   ├─ ProviderManager
  │   ├─ SignalEmitter
  │   ├─ EventBus
  │   ├─ Policy/Audit
  │   └─ WorkspaceManager
  │
  ├─ transports
  │   ├─ mcp
  │   ├─ http
  │   ├─ grpc
  │   └─ connector
  │
  └─ providers
      ├─ sandbox
      ├─ browser
      ├─ filesystem
      └─ ...
```

Kratos 作为 server/transport/middleware/config/lifecycle 外壳。

---

## 12. 推荐仓库结构

```text
repo/
  api/
    proto/
      node/v1/
      provider/v1/
      signal/v1/
      trigger/v1/
      observation/v1/
      approval/v1/
      audit/v1/
      workspace/v1/
      environment/v1/
      connector/v1/

  services/
    agent-control-plane/
      cmd/
      internal/
        agent/
        model/
        session/
        context/
        signal/
        trigger/
        toolrouter/
        registry/
        connector/
        policy/
        audit/
        api/

    connector-hub/
      cmd/
      internal/
        tunnel/
        registry/
        heartbeat/
        routing/
        auth/

    capability-node/
      cmd/
        capnode/
      internal/
        server/
          http/
          grpc/
          mcp/
          static/
        kernel/
          provider/
          signal/
          eventbus/
          policy/
          approval/
          audit/
          credential/
          resource/
          workspace/
          connector/
        providers/
          sandbox/
          browser/
          messaging/
          mail/
          calendar/
          filesystem/
          knowledge/
          monitor/
          scheduler/
          webhook/
          custom/
          mcpadapter/
          httpadapter/
          grpcadapter/
      web/
        dist/

  packages/
    provider-sdk-go/
    provider-sdk-python/
    provider-sdk-ts/
    mcp/
    signal/
    cli/

  deployments/
    docker-compose/
    k8s/
    local/
```

---

## 13. 核心 API 草案

### 13.1 ProviderService

```proto
service ProviderService {
  rpc ListProviders(ListProvidersRequest) returns (ListProvidersResponse);
  rpc GetProvider(GetProviderRequest) returns (ProviderManifest);
  rpc StartProvider(StartProviderRequest) returns (ProviderStatus);
  rpc StopProvider(StopProviderRequest) returns (ProviderStatus);
  rpc GetProviderHealth(GetProviderHealthRequest) returns (ProviderHealth);
}
```

### 13.2 SignalService

```proto
service SignalService {
  rpc EmitSignal(EmitSignalRequest) returns (EmitSignalResponse);
  rpc EmitSignalBatch(EmitSignalBatchRequest) returns (EmitSignalBatchResponse);
}
```

### 13.3 TriggerService

```proto
service TriggerService {
  rpc ListRules(ListRulesRequest) returns (ListRulesResponse);
  rpc CreateRule(CreateRuleRequest) returns (TriggerRule);
  rpc UpdateRule(UpdateRuleRequest) returns (TriggerRule);
  rpc DeleteRule(DeleteRuleRequest) returns (DeleteRuleResponse);
  rpc TestRule(TestRuleRequest) returns (TestRuleResponse);
}
```

### 13.4 ObservationService

```proto
service ObservationService {
  rpc StreamEvents(StreamEventsRequest) returns (stream Event);
  rpc QueryEvents(QueryEventsRequest) returns (QueryEventsResponse);
  rpc StreamMetrics(StreamMetricsRequest) returns (stream MetricFrame);
  rpc StreamLogs(StreamLogsRequest) returns (stream LogEvent);
}
```

### 13.5 ApprovalService

```proto
service ApprovalService {
  rpc ListPendingApprovals(ListPendingApprovalsRequest) returns (ListPendingApprovalsResponse);
  rpc RespondApproval(RespondApprovalRequest) returns (RespondApprovalResponse);
}
```

### 13.6 WorkspaceService

```proto
service WorkspaceService {
  rpc ListFiles(ListFilesRequest) returns (ListFilesResponse);
  rpc ReadFile(ReadFileRequest) returns (stream FileChunk);
  rpc WriteFile(stream FileChunk) returns (WriteFileResponse);
  rpc WatchFiles(WatchFilesRequest) returns (stream FileEvent);
  rpc Download(DownloadRequest) returns (stream FileChunk);
  rpc Snapshot(SnapshotRequest) returns (Snapshot);
  rpc Restore(RestoreRequest) returns (Workspace);
}
```

### 13.7 EnvironmentService

```proto
service EnvironmentService {
  rpc ListEnvironments(ListEnvironmentsRequest) returns (ListEnvironmentsResponse);
  rpc GetEnvironment(GetEnvironmentRequest) returns (Environment);
  rpc RestartEnvironment(RestartEnvironmentRequest) returns (Environment);
  rpc SleepEnvironment(SleepEnvironmentRequest) returns (Environment);
  rpc DestroyEnvironment(DestroyEnvironmentRequest) returns (DestroyEnvironmentResponse);
  rpc ListProcesses(ListProcessesRequest) returns (ListProcessesResponse);
  rpc KillProcess(KillProcessRequest) returns (KillProcessResponse);
}
```

---

## 14. 关键运行链路

### 14.1 Agent 主动调用工具

```text
User
  -> Cloud Agent Control Plane
  -> Agent Runtime
  -> Model
  -> Tool decision
  -> MCP Tool Router
  -> Connector Hub
  -> Capability Node
  -> ProviderManager
  -> PolicyEngine
  -> Provider Runtime
  -> ToolResult
  -> Agent Runtime
  -> User
```

### 14.2 外部消息触发 Agent

```text
Telegram / Slack / WhatsApp
  -> MessagingProvider on Capability Node
  -> EmitSignal(channel.message.received)
  -> Connector
  -> Cloud Signal Router
  -> TriggerEngine
  -> ContextInbox append user_message
  -> AgentRuntime wake
  -> Agent calls message.send
```

### 14.3 Signal 触发另一个 MCP Tool

```text
browser.download.completed
  -> Signal
  -> TriggerEngine
  -> ActionExecutor
  -> call MCP tool: workspace.import_file
  -> emit workspace.artifact.created
  -> notify UI
```

### 14.4 监控告警触发 Agent 或 UI

```text
MetricsCollector detects CPU > threshold
  -> node local aggregation
  -> Signal: sandbox.cpu.high
  -> Cloud TriggerEngine
  -> append system_observation
  -> maybe wake agent
  -> UI notification
```

### 14.5 普通 MCP Server 接入

```text
Existing MCP Server
  -> MCPAdapterProvider
  -> Capability Node
  -> ProviderManager
  -> Tool Registry
  -> Cloud Agent ToolRouter
```

---

## 15. 安全架构

### 15.1 基础原则

- Capability Node 默认只监听 127.0.0.1；
- 节点主动出站连接云端；
- 云端不直接访问用户内网；
- 所有 tool call 经过 policy；
- 所有 trigger action 经过 policy；
- 高风险动作需要 approval；
- 所有执行写 audit log；
- Provider 之间不直接互调；
- 凭据由 CredentialManager 管理；
- secrets 默认不进入 sandbox；
- host target 和 sandbox target 明确区分；
- 普通 MCP server 默认不裸暴露 destructive tools。

### 15.2 Policy 输入

```text
actor
session
agent
node
provider
tool
arguments summary
target
risk class
side effect
credential scope
workspace
trigger source
```

### 15.3 Policy 输出

```text
allow
deny
require_approval
redact
sandbox_only
host_allowed
rate_limit
```

### 15.4 普通 MCP Server 安全

普通 MCP server 接入后应该默认：

- unknown tools 默认需要审批或禁用；
- write/destructive tools 默认审批；
- 支持 tool visibility；
- 支持 timeout；
- 支持参数脱敏；
- 支持允许的 session/agent scope；
- stdio MCP command 需要明确用户授权；
- adapter 负责进程生命周期和日志采集。

---

## 16. 部署模式

### 16.1 个人本地节点 + 云端控制面

```text
Cloud Agent Control Plane
        |
Connector
        |
User Laptop Capability Node
        |
filesystem / sandbox / browser / messaging / mail
```

特点：

- 用户只安装一个 `capnode`；
- 模型可以在云端；
- 能力在本地；
- 本地 UI 可审查；
- 节点主动连接云端。

### 16.2 完全本地模式

```text
Local Agent Runtime
        |
Local Capability Node
        |
local model / local tools / local UI
```

适合隐私敏感用户或企业内网。

### 16.3 企业内网节点

```text
Cloud Agent Control Plane
        |
Enterprise Capability Node
        |
internal APIs / databases / knowledge base / approval systems
```

特点：

- 云端不可直接访问内网；
- 企业节点主动出站；
- 本地策略和审计；
- 可对接企业 IAM/SSO/审批系统。

### 16.4 云端托管节点

```text
Cloud Agent Control Plane
        |
Cloud Capability Node
        |
managed sandbox / cloud browser / hosted providers
```

用于托管 sandbox、browser、public SaaS connectors。

---

## 17. 开发者扩展模型

### 17.1 Level 1：接入已有 MCP Server

最低门槛。

```yaml
providers:
  - id: my-local-mcp
    type: mcp
    command: "python my_server.py"
```

### 17.2 Level 2：HTTP Provider

适合脚本和个人工具。

```yaml
providers:
  - id: my-home-tools
    type: http-provider
    base_url: http://127.0.0.1:8899
    tools:
      - name: home.turn_on_light
        path: /tools/turn_on_light
```

### 17.3 Level 3：Provider SDK

提供 Python/TypeScript/Go SDK。

```python
from capnode import Provider, tool, signal

app = Provider("my-tools")

@app.tool("hello.say")
def say(name: str):
    return {"text": f"hello {name}"}

@app.signal("file.changed")
def watch():
    ...
```

### 17.4 Level 4：Native Kratos Provider

适合官方和企业深度 provider。

实现：

- ProviderService；
- SignalService；
- Control API；
- Observation events；
- Health；
- Policy integration。

---

## 18. 里程碑路线

### Phase 1：核心内核 MVP

目标：证明架构成立。

实现：

- agent-control-plane 最小 session + tool router；
- capability-node 单二进制；
- MCPGateway；
- MCPAdapterProvider；
- ProviderManager；
- SignalService；
- EventBus；
- Policy/Audit 简单版；
- Embedded Local UI；
- filesystem provider；
- sandbox provider 简化版；
- system-monitor pure trigger provider。

验证：

- agent 调 MCP tool；
- 普通 MCP server 接入；
- provider 发 Signal；
- Signal 触发另一个 MCP tool；
- UI 看到事件、日志、审批、文件。

### Phase 2：Sandbox Provider 完整化

实现：

- Workspace 持久；
- Environment 可替换；
- Docker/local driver；
- outputs/artifacts；
- stdout/stderr；
- CPU/memory；
- process list；
- snapshot/restore。

### Phase 3：Messaging Provider

实现：

- message.send MCP tool；
- channel.message.received Signal；
- session routing；
- ContextInbox；
- agent wakeup；
- allowlist；
- message audit。

### Phase 4：Browser Provider

实现：

- browser.open；
- browser.snapshot；
- browser.act；
- browser.download.completed Signal；
- browser.console.error Signal；
- screenshots；
- tabs；
- trace；
- permissions；
- human takeover。

### Phase 5：Provider SDK 与企业能力

实现：

- external gRPC provider；
- external HTTP provider；
- existing MCP server adapter 完整化；
- pure trigger provider；
- enterprise connector；
- policy DSL；
- audit backend；
- OTel export。

---

## 19. 主要风险与应对

### 19.1 过度抽象

风险：

- Provider、Surface、Signal、Trigger 太复杂，拖慢 MVP。

应对：

- 先做最小内核；
- Manifest 先简单；
- Trigger rule 先支持基础 YAML/JSON；
- Provider SDK 后置；
- 优先支持 MCPAdapterProvider。

### 19.2 自动化循环

风险：

```text
Signal -> Tool -> Signal -> Tool
```

应对：

- causality_id；
- trace_id；
- parent_event_id；
- max_depth；
- cooldown；
- dedupe_key；
- rate limit；
- coalesce window。

### 19.3 本地安全边界

风险：

- 云端 Agent 通过本地节点操作用户机器；
- 普通 MCP server 暴露高危能力；
- host browser / shell / filesystem 权限过大。

应对：

- 本地节点最终裁决；
- 高危动作审批；
- 本地 UI 可审查；
- host/sandbox target 区分；
- destructive tool 默认禁用或审批；
- 凭据不直接暴露给模型；
- 全量 audit。

### 19.4 Provider 生态质量

风险：

- 用户 provider 不稳定；
- 恶意 provider；
- MCP server 工具声明不准确；
- 工具副作用不可控。

应对：

- manifest 校验；
- health check；
- timeout；
- signature/source；
- sandboxed execution；
- tool visibility；
- audit；
- disable/rollback；
- provider marketplace 后置。

---

## 20. 最终架构原则

1. Cloud Agent Control Plane 是系统大脑。
2. Capability Node 是可部署的能力执行和治理单元。
3. Capability Provider 是一等抽象，Tool 不是。
4. MCP 是 Agent Surface，用于 agent 主动调用能力。
5. Signal 是外部世界进入 Agent Runtime 的入口。
6. TriggerEngine 负责 Signal 到上下文、Agent 唤醒、MCP tool call 或 UI 通知的编排。
7. HTTP/gRPC/Event 是控制、观测、治理和资源面的实现通道。
8. 普通 MCP server 是最低成本接入方式。
9. Native Provider 是深度集成方式。
10. Provider 之间不直接互调，通过 Signal、Trigger、ActionExecutor 解耦。
11. 所有 tool call 和 trigger action 都经过 Policy/Audit。
12. Workspace 持久，Environment 可替换。
13. 本地、云端、企业节点使用同一 Capability Node 架构，不同 deployment profile。
14. Kratos 是服务底座，不应该侵入业务核心。
15. 个人节点必须单二进制、单进程、轻量部署。

---

## 21. 一句话总结

本项目的整体架构是：

```text
Cloud Agent Control Plane + many Capability Nodes
```

Cloud Agent Control Plane 负责 agent session、模型调用、上下文、Signal/Trigger、工具路由、节点注册和全局治理。

Capability Node 负责提供和执行能力，可以部署在本地、云端、企业内网或边缘环境。

Agent 通过 MCP 调用能力；Provider 通过 Signal 把外部事实送回控制面；HTTP/gRPC/Event 支撑控制、观测、治理和资源；Provider Runtime 负责真实世界复杂度。

这套架构可以统一 sandbox、browser、messaging、mail、calendar、knowledge、filesystem、monitor、scheduler、webhook、enterprise tools、普通 MCP server 和用户自定义能力。
