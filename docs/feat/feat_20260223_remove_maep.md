---
date: 2026-02-23
title: Hard Remove MAEP（实施方案）
status: draft
---

# Hard Remove MAEP（实施方案）

## 1) 背景与目标

当前仓库中，MAEP 虽然默认运行关闭（`server.with_maep=false`、`telegram.with_maep=false`），但代码层面深度耦合到：
- CLI 命令与参数
- Telegram/Daemon runtime
- Bus channel/adapters
- Contacts 路由与发送器
- 安装流程与本地状态目录

本方案目标是**硬移除 MAEP**，将系统收敛为：
- Telegram + Slack 两条通道
- Contacts 仅在 Telegram/Slack 之间路由
- Bus 仅保留 telegram/slack/discord（保留 discord 预埋）

## 2) 第一性原则

- 只保留当前真实需要的能力，删掉长期未使用的复杂度。
- 一个能力不再支持，就同时删除其入口、实现、测试、配置与文档，避免“僵尸代码”。
- 拆除以“可编译、可测试、可回滚”为边界，分批提交。

## 3) 非目标

- 不在本次重构中引入新的 P2P 协议替代 MAEP。
- 不重做 contacts 业务模型（仅删除 MAEP 相关字段/分支）。
- 不改动 heartbeat/memory 主流程（除非受编译依赖牵连）。

## 4) 现状摘要（耦合面）

- CLI 入口：
  - `mistermorph maep` 子命令
  - `telegram --with-maep`、`serve --with-maep`
- Runtime：
  - Telegram 内嵌 MAEP node + MAEP 入站 auto-reply 流程
  - Daemon 可选启动 MAEP listener
- Bus：
  - `ChannelMAEP`、`BuildMAEPPeerConversationKey(...)`
  - `internal/bus/adapters/maep/*`
- Contacts：
  - `ChannelMAEP` 路由分支
  - `maep_node_id` / `maep_dial_address` 字段
  - `contactsruntime` 发送器支持 MAEP publish
- Install/State：
  - install 时自动初始化 MAEP identity
  - `statepaths.MAEPDir()`
- 依赖：
  - `go-libp2p` / `go-multiaddr` 及其大量传递依赖

## 5) 方案总览

按 6 个批次实施，每批均要求：`go test ./...` 通过后再进入下一批。

### 批次 A：入口与配置下线

- 删除 `cmd/mistermorph/root.go` 中 `maepcmd.New()` 注册。
- 删除 `cmd/mistermorph/telegramcmd/command.go`:
  - `--with-maep`
  - `--maep-listen`
  - 对应输入字段映射
- 删除 `cmd/mistermorph/daemoncmd/serve.go`:
  - `--with-maep`
  - `--maep-listen`
  - 启动 embedded MAEP 的分支
- 删除默认配置与样例配置中的：
  - `maep.*`
  - `server.with_maep`
  - `telegram.with_maep`

### 批次 B：Telegram Runtime 去 MAEP 化

- 删除 `internal/channelruntime/telegram/runtime.go` 中：
  - `withMAEP` 分支
  - `maepEventCh` 与 MAEP bus dispatch
  - MAEP inbound feedback/session 限流/auto-reply 逻辑
- 删除 `internal/channelruntime/telegram/runtime_task.go` 中：
  - `runMAEPTask(...)`
  - `buildMAEPRegistry(...)`
  - MAEP prompt policy 注入分支（若仅用于 MAEP）
- 删除 MAEP 专属 prompt 模板与渲染器（若无其他引用）。

### 批次 C：Bus 与 Adapter 收敛

- 删除 `internal/bus/message.go` 的 `ChannelMAEP`。
- 删除 `internal/bus/conversation_key.go` 的 MAEP builder 与 MAEP prefix 分支。
- 删除目录 `internal/bus/adapters/maep/` 及其测试。
- 修复 `internal/bus/adapters/inbound_flow.go` 等共享校验里对 MAEP 的合法值引用。

### 批次 D：Contacts 与发送路径收敛

- 删除 `contacts/types.go`:
  - `ChannelMAEP`
  - `MAEPNodeID` / `MAEPDialAddress`
  - `ShareDecision.PeerID`（若确认仅用于 MAEP）
- 删除 `contacts/service.go` 中 MAEP 路由回退分支。
- 删除 `internal/contactsruntime/sender.go` 中：
  - MAEP publish/delivery/node 初始化逻辑
  - `MAEPDir` 选项
- 收敛 `tools/builtin/contacts_send.go` 文案与参数说明（移除 MAEP 示例）。

### 批次 E：状态与安装流程收敛

- 删除 `internal/statepaths/statepaths.go` 的 `MAEPDir()`。
- 删除 install 中 MAEP identity 初始化逻辑。
- 删除 `cmd/mistermorph/registry.go` / runtime snapshot 中 `MAEPDir` 字段透传。

### 批次 F：删除 MAEP 代码与依赖

- 删除目录：
  - `maep/`
  - `cmd/mistermorph/maepcmd/`
  - `internal/maepruntime/`
- 清理依赖并执行 `go mod tidy`，移除 libp2p/multiaddr 相关依赖。
- 清理文档入口：
  - `docs/README.md` 中 MAEP 项
  - 与 MAEP 强绑定的实现文档改为“removed”说明或删除。

## 6) 数据兼容与迁移策略

### 6.1 配置兼容

- 对已存在的 `maep.*`、`*.with_maep` 键采取“忽略但不报错”策略（由 viper 天然容忍未知键）。
- CLI 层直接移除参数，用户若继续传入会得到参数不存在错误（预期行为）。

### 6.2 Contacts 存量数据

- 历史 `contacts` 文件中若存在 MAEP 字段：
  - 读取时忽略未知字段（YAML 解码容忍）
  - 路由时不再走 MAEP，若联系人仅有 MAEP reachability，则返回明确错误：`unable to resolve delivery channel`
- 不做自动数据迁移脚本（本次目标是代码硬移除）。

## 7) 风险与控制

- 风险 1：删除 `ChannelMAEP` 触发 bus/contacts 广泛编译错误。  
  控制：按批次 C -> D 处理，逐层收敛并保持每批可测试。

- 风险 2：`contacts_send` 行为变化导致已有 MAEP 联系人发送失败。  
  控制：明确错误信息，更新工具描述和文档。

- 风险 3：依赖移除后触发构建脚本或文档引用残留。  
  控制：最终批次执行 repo 级 grep 与全量测试。

## 8) 验收标准（Definition of Done）

- `go test ./...` 全绿。
- `mistermorph --help` 不再出现 `maep` 子命令。
- `mistermorph telegram --help` / `mistermorph serve --help` 不再出现 MAEP 参数。
- 仓库中不再存在 MAEP runtime 代码路径（文档历史记录除外）。
- `go.mod` 不再依赖 `go-libp2p`、`go-multiaddr`。
- `contacts_send` 文案与行为仅覆盖 Telegram/Slack。

## 9) 提交策略

- 推荐按批次拆成 4-6 个 commit：
  1. CLI/config 下线
  2. Telegram/Daemon runtime 去 MAEP
  3. Bus + Contacts + sender 收敛
  4. 删除 MAEP 目录与依赖清理
  5. 文档与说明收尾

这样每个 commit 都可独立 review 与回滚。
