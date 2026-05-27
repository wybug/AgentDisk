# 文件预览接口

提供文件在线预览功能，支持多种文件类型的渲染。对于 Markdown、代码文件、文本文件和图片等类型，返回预签名 URL 供前端渲染。对于 HTML 文件，提供沙箱化的直接内容输出。

## 获取文件预览信息

获取文件的预览信息，包括文件类型和用于预览的预签名 URL。

```
GET /v1/disk/preview/:id
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "fileType": "md",
    "url": "https://oss.example.com/bucket/user_abc123/1/readme_v1_1718452800.md?X-Amz-Algorithm=...&X-Amz-Expires=3600"
  }
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `fileType` | `string` | 文件类型（扩展名），如 `md`、`pdf`、`png`、`jpg`、`txt`、`json`、`py`、`go` 等 |
| `url` | `string` | OSS 预签名 URL，可直接用于获取文件内容进行渲染 |

### 支持预览的文件类型

| 类型 | 扩展名 | 说明 |
|------|--------|------|
| Markdown | `.md` | 前端使用 Markdown 渲染器展示 |
| 代码文件 | `.py`、`.go`、`.js`、`.ts`、`.java`、`.json`、`.yaml`、`.xml` 等 | 前端使用代码高亮展示 |
| 文本文件 | `.txt`、`.csv`、`.log` 等 | 纯文本展示 |
| 图片 | `.png`、`.jpg`、`.jpeg`、`.gif`、`.svg`、`.webp` 等 | 直接展示图片 |
| PDF | `.pdf` | 使用浏览器内置 PDF 查看器或 PDF.js |
| HTML | `.html`、`.htm` | 通过沙箱 iframe 安全渲染（见下方接口） |

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/preview/10 \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 401 | Token 无效或已过期 |
| 500 | 预览信息获取失败 |

---

## HTML 文件沙箱预览

直接返回 HTML 文件的原始内容，并附带严格的安全响应头，用于在 iframe 中安全渲染。此接口不返回 JSON，而是直接返回 `text/html` 内容。

```
GET /v1/disk/preview/:id/html
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 文件 ID |

### 响应头

| 响应头 | 值 | 说明 |
|--------|-----|------|
| `Content-Type` | `text/html; charset=utf-8` | HTML 内容类型 |
| `X-Content-Type-Options` | `nosniff` | 禁止 MIME 类型嗅探 |
| `Content-Security-Policy` | `default-src 'none'; style-src 'unsafe-inline'; img-src * data: blob:; ...` | 严格 CSP 策略 |
| `X-Frame-Options` | `DENY` | 禁止被嵌入 iframe |
| `Referrer-Policy` | `no-referrer` | 不发送 Referrer |

::: warning 安全说明
HTML 预览采用严格的内容安全策略（CSP）：
- 禁止执行 JavaScript 脚本
- 仅允许内联样式
- 图片允许从任意来源加载
- 禁止网络请求、表单提交、嵌入对象

这些限制确保了潜在的恶意 HTML 内容不会对系统造成安全威胁。
:::

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/preview/15/html \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 400 | `id` 格式无效 |
| 401 | Token 无效或已过期 |
| 500 | 文件内容获取失败 |
