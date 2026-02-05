---
date: 2026-02-05
title: Telegram Message Reactions
status: draft
---

# Telegram Reactions（需求文档）

## 1) 背景与目标
目前 Telegram 模式下，机器人只能“发消息回复”。但在很多场景下，**合适的 emoji reaction** 就足够表达确认/收到/赞同/已处理，不需要额外文字，减少噪音并更贴近人类交流习惯。

### 目标
- 支持机器人对消息添加 **emoji reaction**。
- 当“反应已足够”时，允许 **仅 reaction、不发文本**。
- 仍保留在需要信息传达时的 **正常文字回复**。

### 非目标
- 不做“表情包/贴纸/图片回复”的替代方案。
- 不做跨平台（非 Telegram）统一 reaction 体系。

## 2) 典型场景
- 用户给出短确认类请求：“行/好/收到/👌？” → 机器人用 ✅ 或 👍 反应即可。
- 用户分享信息无需回答：“FYI/仅告知” → 👀 / ✅。
- 用户提交完成项：“完成了/已处理” → 🎉 / ✅。
- 用户对机器人回答表示感谢 → 🙏（可选）。
- 用户发来进度更新：“在做了/处理中/稍后补” → 👀 / 👍。
- 用户仅发送表情或贴图 → 用匹配语义的单个 reaction（如 👍/👀/😂）。
- 用户发来链接/文件但未提问 → 👀（表示已看，等待后续）。
- 用户复述任务要求、无需回复 → ✅。
- 用户确认安排时间/计划：“就这样/按这个来” → ✅。
- 用户否定/取消：“算了/不用了/取消” → 👎 或 ❗（谨慎）。
- 用户提出轻微抱怨：“有点慢/还没好” → 👀 / 🙏（承接但不打断）。
- 用户发来完成截图/结果 → 🎉 / ✅。
- 群聊中他人@机器人并表示“收到/确认” → ✅（不再刷屏）。

## 3) 功能范围
### 3.1 触发条件（反应 vs 文本）
以下满足任一，即可 **优先 reaction**：
- 用户意图为“确认/收到/已读/简单肯定/简单否定”，且不需要补充信息。
- 用户发出“FYI/仅通知/无需回复”之类语义。
- 用户请求为“轻量确认”且无后续动作需求。
 - 意图推断结果显示 deliverable 为空或为“轻量回应/确认”，且适合无文本交付。

以下必须 **文本回复**：
- 需要给出事实、步骤、解释、结果或长文本。
- 需要调用工具并输出结果。
- 需要澄清（即使极少发问，也需要文字表达假设或要求）。
 - 意图推断结果显示 deliverable 需要具体内容或结构化输出。

### 3.2 Reaction 选择策略
基础策略：**给出最少且最合适的一枚 reaction**。

建议默认 emoji 映射（可配置）：
- 确认/完成：✅
- 赞同/认可：👍
- 已读/关注：👀
- 否定/取消：👎（谨慎）
- 需要注意/提醒：❗ 或 ⚠️
- 祝贺/完成里程碑：🎉
- 感谢/致谢：🙏
- 不确定/需跟进：🕒 或 🔍（谨慎使用）
- 轻度认可/挺好：👌
- 好的/收到（更轻量）：👍 或 ✅
- 开心/正向反馈：😊（仅在语境明确时）

选择规则补充：
- 如果是“确认执行/已完成”，优先 ✅。
- 如果是“认可观点/不错/OK”，优先 👍。
- 如果是“我看到了/正在看”，优先 👀。
- 如果涉及“取消/否定”，优先 👎，避免攻击性 emoji。
- 避免连续多 emoji，除非用户明确要求。

### 3.3 与上下文/历史的处理
- reaction 成功后，在历史里记录一个 **轻量标记**（如 `[reacted: ✅]`），避免“对话断裂”。
- 若 reaction 失败，则回落为文本回复（提示失败原因或简短兜底）。

### 3.4 与心跳的关系
心跳输出必须 **以文本总结**发送；不使用 reaction 代替心跳摘要。

## 4) 交互与行为
- **默认：能 reaction 就 reaction**（不发文本），除非必须输出内容。
- 若用户明确要求“回复内容/解释/列出”等，必须文本回复。
- 如果 reaction 无法表达明确含义，回退文本。

## 5) 技术实现（高层）
### 5.1 Telegram API 调用
- 新增 Telegram 方法调用：**为指定 message_id 添加 reaction**。
- 传入 chat_id + message_id + emoji（或 emoji 列表）。

### 5.2 系统结构建议
- 新增工具：`telegram_react`（或 `telegram_reaction`）。
- 在 agent 输出层增加 **“reaction-only”** 路径：
  - 若输出为 reaction，则不发送文本。
  - 若 reaction 失败，自动 fallback 为文本。
 - 先跑 intent 推断，再判断是否适合轻回复（reaction）。不适合则走正常文本输出。

### 5.3 工具与规则
在系统 prompt 里新增规则：
- “如果 reaction 足够表达含义且无需传达额外信息，优先使用 reaction。”
- “reaction-only 时不要再发消息，避免重复。”

### 5.4 判定逻辑（伪代码）
```text
intent = infer_intent(task, history)

if intent.ask == true:
  return TEXT_RESPONSE

if intent.deliverable is empty or intent.deliverable in ["确认", "轻量回应", "已读确认", "短确认"]:
  if is_lightweight_intent(intent.goal) and no_tool_needed(task) and not_high_risk(task):
    return REACTION
  else:
    return TEXT_RESPONSE

if intent.requires_structured_output or intent.requires_facts_or_steps:
  return TEXT_RESPONSE

# fallback
return TEXT_RESPONSE
```

### 5.5 判定规则表（简化）
| 条件 | 结果 |
| --- | --- |
| intent.ask = true | 文本回复 |
| deliverable 为空且 goal=确认/已读/感谢/认可 | reaction |
| 需要工具/计算/查找 | 文本回复 |
| 需要结论/解释/清单 | 文本回复 |
| 涉及高风险或不可逆动作 | 文本回复 |
| 其它 | 文本回复 |

## 6) 配置项（建议）
```yaml
telegram:
  reactions:
    enabled: true
    # 允许的 emoji 白名单
    allow: ["✅", "👍", "👀", "🎉", "🙏", "❓"]
    # 每条消息最多 reaction 数量（默认 1）
    max_per_message: 1
```

## 7) 日志与可观测性
- 记录 reaction 结果（成功/失败、emoji、message_id）。
- 失败时记录原因，并回退到文本回复。

## 8) 安全与合规
- 仅对授权 chat_id 生效（与现有 allowed_chat_ids 机制一致）。
- 反应不会携带敏感信息，风险低。

## 9) 验收标准
- 用户发送“收到/OK/感谢”等短消息 → 仅 reaction，无文字。
- 用户请求“帮我查/列出/解释” → 正常文字回复。
- reaction 失败 → 自动回落文字回复（提示失败原因）。
- 心跳仍发送文本总结，不使用 reaction 代替。

## TODO
- [ ] 明确高风险/不可逆动作清单（用于禁止 reaction-only）。
  - [ ] 包含文件写入/删除、网络提交、付费、权限变更等。
  - [ ] 列出需要文字确认的场景（即便 intent 轻量也不 reaction）。
- [ ] 定义轻量意图关键词表（用于快速判定）。
  - [ ] 中文：收到/好的/OK/谢谢/了解/不用了/取消 等。
  - [ ] 英文：ok/thanks/ack/received/not needed 等。
  - [ ] 区分肯定/否定/感谢/已读/祝贺五类。
- [ ] 确认 Telegram API 的 reaction 参数格式与权限限制。
  - [ ] 确认 Bot 是否能对任意消息 reaction（群/私聊差异）。
  - [ ] 确认多 emoji 的上限与失败响应。
- [ ] 设计 `telegram_react` 工具接口与返回结构。
  - [ ] 入参：chat_id、message_id、emoji（或列表）。
  - [ ] 出参：ok、error、applied_emoji。
- [ ] 实现 reaction-only 输出路径与回退逻辑。
  - [ ] 先意图推断，再判定 reaction vs text。
  - [ ] reaction 成功则不发送文本；失败则发文本。
  - [ ] 记录 `[reacted: 😀]` 进 history。
- [ ] 添加端到端测试（reaction 成功/失败/不适用场景）。
  - [ ] reaction-only 的输出分支覆盖。
  - [ ] fallback 文本分支覆盖。
