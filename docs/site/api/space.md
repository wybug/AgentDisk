# 空间管理接口

查询当前用户的云盘空间使用情况，包括已用配额和总配额。

## 获取用户空间信息

获取当前登录用户的磁盘配额使用情况。

```
GET /v1/disk/space
```

### 认证方式

需要 JWT Bearer Token 或 API Key 认证。

### 请求参数

无。

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "userId": "user_abc123",
    "totalQuota": 10737418240,
    "usedQuota": 2147483648,
    "rootFolder": "",
    "createdAt": "2025-06-01T08:00:00Z",
    "updatedAt": "2025-06-15T14:30:00Z"
  }
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `uint64` | 空间记录 ID |
| `userId` | `string` | 用户唯一标识 |
| `totalQuota` | `int64` | 总配额，单位：字节，默认 10737418240（10GB） |
| `usedQuota` | `int64` | 已用配额，单位：字节 |
| `rootFolder` | `string` | 根目录标识 |
| `createdAt` | `string` | 创建时间（RFC 3339） |
| `updatedAt` | `string` | 更新时间（RFC 3339） |

### 常用容量换算

| 配额值 | 对应大小 |
|--------|---------|
| `10737418240` | 10 GB（默认） |
| `2147483648` | 2 GB |
| `536870912` | 512 MB |
| `1048576` | 1 MB |

### curl 示例

```bash
curl -X GET http://localhost:9100/v1/disk/space \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 错误场景

| HTTP 状态码 | 场景说明 |
|------------|---------|
| 401 | Token 无效或已过期 |
| 404 | 用户空间记录不存在（新用户首次访问会自动创建） |
