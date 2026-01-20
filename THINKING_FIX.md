# 思维链修复说明

## 问题原因

你的 Claude-opus-4.5-thinking 模型无法显示思维链有以下两个主要原因:

### 1. **缺少 `signature` 字段** (必需)

根据 Claude Code 的要求,thinking 块**必须包含 `signature` 字段**,否则会被当作普通文本显示:

```json
{
  "type": "thinking",
  "thinking": "思考内容...",
  "signature": ""  // ← 必需!没有这个字段,思维链不会被正确隐藏
}
```

### 2. **不支持原生 Claude 格式**

旧代码只支持两种格式:
- OpenRouter: `reasoning_details` 数组
- OpenAI o1/o3: `reasoning_content` 字符串 (仅流式)

但你的服务商可能直接返回 Claude 原生格式的 content 数组。

---

## 已修复的内容

### ✅ 修复 1: 添加 `signature` 字段

**文件**: `internal/converter/converter.go:377`

### ✅ 修复 2: 支持原生 Claude content 数组

**文件**: `internal/converter/converter.go:396-421`

---

## 如何测试

### 步骤 1: 重新编译代理

```bash
go build -o claude-code-proxy.exe cmd/claude-code-proxy/main.go
```

### 步骤 2: 启动调试模式

```bash
.\claude-code-proxy.exe -d -s
```

### 步骤 3: 测试思维链

```bash
$env:ANTHROPIC_BASE_URL="http://localhost:8082"
claude --model opus -p "计算 15 + 27,请展示你的思考过程"
```

### 步骤 4: 检查调试输出

查看代理终端,找到 `=== OPENAI RESPONSE ===` 部分,检查服务商返回的格式。

---

## 验证修复是否成功

### 成功的标志:

1. **Claude Code UI 中看到思维链图标** (可折叠/展开)
2. **思维链内容默认折叠,不直接显示在对话中**
3. **点击展开后能看到完整思考过程**

### 失败的标志:

1. ❌ 思维链内容直接显示在回答中 (当作普通文本)
2. ❌ 完全没有思维链内容
3. ❌ 报错 "invalid thinking block"
