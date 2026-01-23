# 通知渠道管理 API 文档

## 基础信息
- **Base URL**: `/setting/notification`
- **Response Format**: 所有接口返回统一的 JSON 格式

```json
{
  "code": 0,      // 0=成功, 1=失败
  "message": "",  // 提示信息
  "data": null    // 返回数据
}
```

---

## 1. 获取所有通知渠道

**接口**: `GET /setting/notification/channels`

**说明**: 获取所有已配置的通知渠道列表及其详细配置信息

**请求参数**: 无

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": [
    {
      "id": 1,
      "channel_type": "telegram",
      "channel_name": "我的 Telegram Bot",
      "is_enabled": true,
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z",
      "config": {
        "bot_token": "123456:ABC-DEF...",
        "chat_id": "123456789",
        "proxy_url": "http://127.0.0.1:7890"
      },
      "rules": [
        {
          "id": 1,
          "channel_id": 1,
          "event_type": "sync_finish",
          "is_enabled": true
        }
      ]
    }
  ]
}
```

**返回的 channel_type 类型**:
- `telegram`: Telegram Bot
- `meow`: MeoW
- `bark`: Bark (iOS)
- `serverchan`: Server酱
- `webhook`: 自定义 Webhook

---

## 2. 创建 Telegram 渠道

**接口**: `POST /setting/notification/channels/telegram`

**说明**: 创建新的 Telegram Bot 通知渠道，代理配置从系统设置中获取

**请求体**:
```json
{
  "channel_name": "我的 Telegram Bot",  // 必填，渠道显示名称
  "bot_token": "123456:ABC-DEF...",    // 必填，Bot Token
  "chat_id": "123456789"               // 必填，Chat ID
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "创建成功",
  "data": {
    "id": 1,
    "channel_type": "telegram",
    "channel_name": "我的 Telegram Bot",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:00:00Z"
  }
}
```

**说明**:
- 创建渠道时会自动生成 6 种事件类型的默认规则（全部启用）
- 默认事件类型: `sync_finish`, `sync_error`, `scrape_finish`, `system_alert`, `media_added`, `media_removed`
- 代理配置从系统设置 (STRM配置) 中读取，无需在渠道配置中设置

---

## 3. 更新 Telegram 渠道

**接口**: `PUT /setting/notification/channels/telegram`

**说明**: 更新 Telegram Bot 通知渠道配置

**请求体**:
```json
{
  "channel_id": 1,                      // 必填，渠道 ID
  "channel_name": "更新后的 Telegram Bot", // 可选，更新渠道显示名称
  "bot_token": "new_token",             // 可选，更新 Bot Token
  "chat_id": "987654321",               // 可选，更新 Chat ID
  "description": "更新后的备注"           // 可选，更新备注
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": 1,
    "channel_type": "telegram",
    "channel_name": "更新后的 Telegram Bot",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-02T14:30:00Z"
  }
}
```

**说明**:
- 所有字段均为可选，只更新提供的字段
- 代理配置从系统设置 (STRM配置) 中读取，无需在渠道配置中设置
- 更新后会自动重新加载通知管理器以应用新配置

---

## 4. 创建 MeoW 渠道

**接口**: `POST /setting/notification/channels/meow`

**说明**: 创建新的 MeoW 通知渠道

**请求体**:
```json
{
  "channel_name": "我的 MeoW",              // 必填，渠道显示名称
  "nickname": "my_nickname",               // 必填，MeoW 昵称
  "endpoint": "http://api.chuckfang.com"   // 可选，默认: http://api.chuckfang.com
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "创建成功",
  "data": {
    "id": 2,
    "channel_type": "meow",
    "channel_name": "我的 MeoW",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:00:00Z"
  }
}
```

**说明**:
- 创建渠道时会自动生成 6 种事件类型的默认规则（全部启用）
- 默认事件类型: `sync_finish`, `sync_error`, `scrape_finish`, `system_alert`, `media_added`, `media_removed`

---

## 5. 更新 MeoW 渠道

**接口**: `PUT /setting/notification/channels/meow`

**说明**: 更新 MeoW 通知渠道配置

**请求体**:
```json
{
  "channel_id": 2,                        // 必填，渠道 ID
  "channel_name": "更新后的 MeoW",          // 可选，更新渠道显示名称
  "nickname": "new_nickname",            // 可选，更新 MeoW 昵称
  "endpoint": "http://new.api.com",     // 可选，更新 API 地址
  "description": "更新后的备注"            // 可选，更新备注
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": 2,
    "channel_type": "meow",
    "channel_name": "更新后的 MeoW",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-02T14:30:00Z"
  }
}
```

**说明**:
- 所有字段均为可选，只更新提供的字段
- 更新后会自动重新加载通知管理器以应用新配置

---

## 6. 创建 Bark 渠道

**接口**: `POST /setting/notification/channels/bark`

**说明**: 创建新的 Bark (iOS 推送) 通知渠道

**请求体**:
```json
{
  "channel_name": "我的 iPhone",            // 必填，渠道显示名称
  "device_key": "your_device_key_here",    // 必填，设备密钥
  "server_url": "https://api.day.app",     // 可选，默认: https://api.day.app
  "sound": "alert",                        // 可选，默认: alert
  "icon": "https://example.com/icon.png"   // 可选，通知图标 URL
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "创建成功",
  "data": {
    "id": 3,
    "channel_type": "bark",
    "channel_name": "我的 iPhone",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:00:00Z"
  }
}
```

**说明**:
- 创建渠道时会自动生成 6 种事件类型的默认规则（全部启用）
- 默认事件类型: `sync_finish`, `sync_error`, `scrape_finish`, `system_alert`, `media_added`, `media_removed`

---

## 7. 更新 Bark 渠道

**接口**: `PUT /setting/notification/channels/bark`

**说明**: 更新 Bark (iOS 推送) 通知渠道配置

**请求体**:
```json
{
  "channel_id": 3,                        // 必填，渠道 ID
  "channel_name": "更新后的 iPhone",        // 可选，更新渠道显示名称
  "device_key": "new_device_key",        // 可选，更新设备密钥
  "server_url": "https://new.api.com",  // 可选，更新服务器地址
  "sound": "chime",                      // 可选，更新通知声音
  "icon": "https://new.icon.png",       // 可选，更新通知图标
  "description": "更新后的备注"            // 可选，更新备注
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": 3,
    "channel_type": "bark",
    "channel_name": "更新后的 iPhone",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-02T14:30:00Z"
  }
}
```

**说明**:
- 所有字段均为可选，只更新提供的字段
- 更新后会自动重新加载通知管理器以应用新配置

---

## 8. 创建 Server酱 渠道

**接口**: `POST /setting/notification/channels/serverchan`

**说明**: 创建新的 Server酱 (微信推送) 通知渠道

**请求体**:
```json
{
  "channel_name": "我的微信推送",           // 必填，渠道显示名称
  "sc_key": "SCU1234567890abcdef",        // 必填，Server酱 SCKEY
  "endpoint": "https://sc.ftqq.com"       // 可选，默认: https://sc.ftqq.com
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "创建成功",
  "data": {
    "id": 4,
    "channel_type": "serverchan",
    "channel_name": "我的微信推送",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:00:00Z"
  }
}
```

**说明**:
- 创建渠道时会自动生成 6 种事件类型的默认规则（全部启用）
- 默认事件类型: `sync_finish`, `sync_error`, `scrape_finish`, `system_alert`, `media_added`, `media_removed`

---

## 9. 更新 Server酱 渠道

**接口**: `PUT /setting/notification/channels/serverchan`

**说明**: 更新 Server酱 (微信推送) 通知渠道配置

**请求体**:
```json
{
  "channel_id": 4,                        // 必填，渠道 ID
  "channel_name": "更新后的微信推送",       // 可选，更新渠道显示名称
  "sc_key": "NEW_SCU1234567890",         // 可选，更新 Server酱 SCKEY
  "endpoint": "https://new.sc.com",     // 可选，更新 API 地址
  "description": "更新后的备注"            // 可选，更新备注
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": 4,
    "channel_type": "serverchan",
    "channel_name": "更新后的微信推送",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-02T14:30:00Z"
  }
}
```

**说明**:
- 所有字段均为可选，只更新提供的字段
- 更新后会自动重新加载通知管理器以应用新配置

---

## 10. 创建 Webhook 渠道

**接口**: `POST /setting/notification/channels/webhook`

**说明**: 创建自定义 Webhook 通知渠道，支持 `GET`/`POST`，`POST` 支持 `json`/`form`/`text` 三种格式。模板将按变量渲染：`{{title}}`, `{{content}}`, `{{timestamp}}`, `{{image}}`。

**请求体**:
```json
{
  "channel_name": "我的 Webhook",           // 必填，渠道显示名称
  "endpoint": "https://example.com/hook",  // 必填，请求地址（HTTP/HTTPS）
  "method": "POST",                        // 必填，GET 或 POST
  "format": "json",                        // POST 必填：json / form / text；GET 可不填
  "template": "{\n  \"title\": \"{{title}}\",\n  \"content\": \"{{content}}\",\n  \"timestamp\": \"{{timestamp}}\"\n}",
  "query_param": "q",                      // GET 可选，默认 q；将渲染内容作为该参数值
  "auth_type": "none",                    // 可选：none|bearer|basic|header|query
  "auth_token": "xxxxx",                  // bearer/header/query 使用
  "auth_user": "user",                    // basic 用户名
  "auth_pass": "pass",                    // basic 密码
  "auth_header_key": "X-Api-Key",         // header 模式下的头名
  "auth_query_key": "token",              // query 模式下的查询参数名
  "headers": {"X-Custom": "v1"},        // 额外请求头（对象）
  "description": "用于第三方系统的通知"      // 可选，备注说明
}
```

**验证规则**:
- `method=POST, format=json`: 将模板中的变量替换为空字符串后，应能被 JSON 解析为合法对象。
- `method=POST, format=form`: 模板必须是 `key=value&key2=value2` 的形式；值部分支持变量表达式。
- `method=POST, format=text`: 不做结构校验，直接作为原始文本发送。
- `method=GET`: 不校验模板结构；渲染后的内容将通过查询参数（默认 `q`）拼接到 `endpoint`。

**模板变量说明**:
- `{{title}}`: 通知标题
- `{{content}}`: 通知正文内容
- `{{timestamp}}`: 发送时间（ISO8601 字符串）
- `{{image}}`: 图片 URL（若事件带图）

**格式示例**:
- POST JSON 模板:
```json
{
  "title": "{{title}}",
  "content": "{{content}}",
  "time": "{{timestamp}}",
  "image": "{{image}}"
}
```
- POST Form 模板:
```
title={{title}}&content={{content}}&time={{timestamp}}&image={{image}}
```
- POST Text 模板:
```
【{{title}}】{{content}} @ {{timestamp}}
```
- GET: 渲染内容作为查询参数，例如：`https://example.com/hook?q=【{{title}}】{{content}}`

**响应示例**:
```json
{
  "code": 0,
  "message": "创建成功",
  "data": {
    "id": 5,
    "channel_type": "webhook",
    "channel_name": "我的 Webhook",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-01T10:00:00Z"
  }
}
```

**说明**:
- 创建后会生成 6 种默认通知规则（全部启用）。
- 发送时将根据 `format` 设定 `Content-Type`：`json`→`application/json`，`form`→`application/x-www-form-urlencoded`，`text`→`text/plain`。
- GET 请求会将渲染结果做 URL 编码后附加到 `query_param`。
- 鉴权：
  - `auth_type=none`：不做鉴权
  - `bearer`：添加 `Authorization: Bearer <auth_token>`
  - `basic`：添加 `Authorization: Basic <base64(user:pass)>`（后端自动处理）
  - `header`：添加自定义头 `auth_header_key: auth_token`
  - `query`：在 URL 上追加 `auth_query_key=auth_token`
  - `headers`：将对象中所有键值合并为请求头

---

## 11. 更新 Webhook 渠道

**接口**: `PUT /setting/notification/channels/webhook`

**说明**: 更新自定义 Webhook 渠道的配置，支持更新所有字段（模板、鉴权、方法等）

**请求体**:
```json
{
  "channel_id": 5,                         // 必填，渠道 ID
  "channel_name": "更新后的名称",            // 可选，更新渠道显示名称
  "endpoint": "https://new.example.com",   // 可选，更新请求地址
  "method": "GET",                         // 可选，更新方法（GET/POST）
  "format": "json",                        // 可选，更新格式
  "template": "新模板内容",                 // 可选，更新模板
  "query_param": "msg",                    // 可选，更新 GET 参数名
  "auth_type": "bearer",                   // 可选，更新鉴权类型
  "auth_token": "new_token",               // 可选，更新鉴权令牌
  "auth_user": "user",                     // 可选，更新 basic 用户名
  "auth_pass": "pass",                     // 可选，更新 basic 密码
  "auth_header_key": "Authorization",      // 可选，更新 header 模式头名
  "auth_query_key": "api_key",             // 可选，更新 query 模式参数名
  "headers": {"X-Version": "2.0"},       // 可选，更新额外请求头
  "description": "更新后的备注"              // 可选，更新备注
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": {
    "id": 5,
    "channel_type": "webhook",
    "channel_name": "更新后的名称",
    "is_enabled": true,
    "created_at": "2024-01-01T10:00:00Z",
    "updated_at": "2024-01-02T14:30:00Z"
  }
}
```

**说明**:
- 所有字段均为可选，只更新提供的字段
- 更新 `template` 时会根据当前或新的 `method`/`format` 进行校验
- 更新 `auth_type` 时会校验对应所需字段是否提供
- 更新后会自动重新加载通知管理器以应用新配置

---

## 12. 查询 Telegram 渠道配置

**接口**: `GET /setting/notification/channels/telegram/:id`

**说明**: 查询单个 Telegram 渠道的详细配置，用于编辑前填充表单。代理配置从系统设置中读取

**请求参数**:
- `id`: 渠道 ID（URL 路径参数）

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": {
    "channel": {
      "id": 1,
      "channel_type": "telegram",
      "channel_name": "我的 Telegram Bot",
      "is_enabled": true,
      "description": "主要通知机器人",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    },
    "config": {
      "id": 1,
      "channel_id": 1,
      "bot_token": "123456:ABC-DEF...",
      "chat_id": "123456789",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    }
  }
}
```

---

## 13. 查询 MeoW 渠道配置

**接口**: `GET /setting/notification/channels/meow/:id`

**说明**: 查询单个 MeoW 渠道的详细配置，用于编辑前填充表单

**请求参数**:
- `id`: 渠道 ID（URL 路径参数）

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": {
    "channel": {
      "id": 2,
      "channel_type": "meow",
      "channel_name": "MeoW 通知",
      "is_enabled": true,
      "description": "MeoW 推送",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    },
    "config": {
      "id": 2,
      "channel_id": 2,
      "meow_key": "meow_12345",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    }
  }
}
```

---

## 14. 查询 Bark 渠道配置

**接口**: `GET /setting/notification/channels/bark/:id`

**说明**: 查询单个 Bark 渠道的详细配置，用于编辑前填充表单

**请求参数**:
- `id`: 渠道 ID（URL 路径参数）

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": {
    "channel": {
      "id": 3,
      "channel_type": "bark",
      "channel_name": "Bark 推送",
      "is_enabled": true,
      "description": "Apple Bark 推送",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    },
    "config": {
      "id": 3,
      "channel_id": 3,
      "device_key": "bark_abcdef",
      "server_url": "https://api.day.app",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    }
  }
}
```

---

## 15. 查询 Server酱 渠道配置

**接口**: `GET /setting/notification/channels/serverchan/:id`

**说明**: 查询单个 Server酱 渠道的详细配置，用于编辑前填充表单

**请求参数**:
- `id`: 渠道 ID（URL 路径参数）

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": {
    "channel": {
      "id": 4,
      "channel_type": "serverchan",
      "channel_name": "Server酱",
      "is_enabled": true,
      "description": "微信 Server酱推送",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    },
    "config": {
      "id": 4,
      "channel_id": 4,
      "send_key": "SCT_xxxxxx",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    }
  }
}
```

---

## 16. 查询 Webhook 渠道配置

**接口**: `GET /setting/notification/channels/webhook/:id`

**说明**: 查询单个自定义 Webhook 渠道的详细配置，用于编辑前填充表单

**请求参数**:
- `id`: 渠道 ID（URL 路径参数）

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": {
    "channel": {
      "id": 5,
      "channel_type": "webhook",
      "channel_name": "自定义 Webhook",
      "is_enabled": true,
      "description": "企业应用集成",
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    },
    "config": {
      "id": 5,
      "channel_id": 5,
      "endpoint": "https://example.com/webhook",
      "method": "POST",
      "template": "title={{title}}&content={{content}}",
      "format": "form",
      "query_param": "msg",
      "auth_type": "bearer",
      "auth_token": "token_abc123",
      "auth_user": "",
      "auth_pass": "",
      "auth_header_key": "Authorization",
      "auth_query_key": "api_key",
      "headers": {
        "X-Custom": "value",
        "X-Version": "1.0"
      },
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    }
  }
}
```

---

## 17. 启用/禁用渠道

**接口**: `POST /setting/notification/channels/status`

**说明**: 启用或禁用指定的通知渠道

**请求体**:
```json
{
  "channel_id": 1,      // 必填，渠道 ID
  "is_enabled": false   // 必填，true=启用, false=禁用
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": null
}
```

---

## 18. 删除渠道

**接口**: `POST /setting/notification/channels/delete`

**说明**: 删除指定的通知渠道（同时删除关联的配置和规则）

**请求体**:
```json
{
  "channel_id": 1  // 必填，渠道 ID
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "删除成功",
  "data": null
}
```

**说明**: 删除操作会级联删除该渠道的所有配置和通知规则

---

## 19. 获取通知规则

**接口**: `GET /setting/notification/rules`

**说明**: 获取通知规则列表，可按渠道过滤

**请求参数**:
- `channel_id` (可选): 渠道 ID，不传则返回所有规则

**示例**:
- 获取所有规则: `GET /setting/notification/rules`
- 获取指定渠道规则: `GET /setting/notification/rules?channel_id=1`

**响应示例**:
```json
{
  "code": 0,
  "message": "获取成功",
  "data": [
    {
      "id": 1,
      "channel_id": 1,
      "event_type": "sync_finish",
      "is_enabled": true,
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    },
    {
      "id": 2,
      "channel_id": 1,
      "event_type": "sync_error",
      "is_enabled": false,
      "created_at": "2024-01-01T10:00:00Z",
      "updated_at": "2024-01-01T10:00:00Z"
    }
  ]
}
```

**支持的事件类型**:
- `sync_finish`: 同步完成
- `sync_error`: 同步错误
- `scrape_finish`: 刮削完成
- `system_alert`: 系统警告
- `media_added`: 媒体添加
- `media_removed`: 媒体移除

---

## 20. 更新通知规则

**接口**: `POST /setting/notification/rules/update`

**说明**: 更新指定渠道的指定事件类型的通知规则（不存在则创建）

**请求体**:
```json
{
  "channel_id": 1,              // 必填，渠道 ID
  "event_type": "sync_finish",  // 必填，事件类型
  "is_enabled": true            // 必填，true=启用, false=禁用
}
```

**响应示例**:
```json
{
  "code": 0,
  "message": "更新成功",
  "data": null
}
```

**说明**: 如果规则不存在会自动创建，已存在则更新

---

## 21. 测试渠道连接

**接口**: `POST /setting/notification/channels/test`

**说明**: 测试指定渠道是否能正常发送通知

**请求体**:
```json
{
  "channel_id": 1  // 必填，渠道 ID
}
```

**响应示例（成功）**:
```json
{
  "code": 0,
  "message": "测试成功",
  "data": null
}
```

**响应示例（失败）**:
```json
{
  "code": 1,
  "message": "测试失败: connection timeout",
  "data": null
}
```

**说明**: 
- 该接口会发送一条标题为"通知渠道测试"的测试消息
- 超时时间为 15 秒
- 可用于验证渠道配置是否正确

---

## 错误码说明

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 1 | 失败（具体错误信息见 message 字段） |

## 常见错误信息

- `参数错误`: 请求参数不符合要求或缺少必填字段
- `创建渠道失败`: 数据库创建渠道记录失败
- `创建配置失败`: 数据库创建配置记录失败
- `渠道不存在`: 指定的 channel_id 不存在
- `配置不存在`: 渠道存在但配置数据缺失
- `未知的渠道类型`: channel_type 不是支持的类型
- `更新失败`: 数据库更新操作失败
- `删除失败`: 数据库删除操作失败

---

## 前端开发建议

### 1. 渠道列表页面
- 展示所有渠道，显示类型、名称、启用状态
- 提供启用/禁用开关
- 提供删除按钮（需二次确认）
- 提供测试连接按钮

### 2. 创建渠道弹窗
- 先选择渠道类型（Telegram/MeoW/Bark/Server酱/Webhook）
- 根据类型显示对应的表单字段
- 必填字段需前端校验

**Webhook 前端字段建议**:
- 类型选择：`method`（GET/POST），当选 POST 时显示 `format`（json/form/text）。
- `endpoint`: 必填，URL 格式校验。
- `template`: 必填，提供多行文本编辑器。
- `query_param`: 仅 GET 显示，默认 `q` 可编辑。
- 校验：POST+json 时可在前端尝试 JSON 解析（变量先替换为空），POST+form 时正则校验 `key=value(&key=value)*`。

### 3. 通知规则管理
- 为每个渠道展示 6 种事件类型的开关
- 使用表格或卡片形式展示
- 实时更新规则状态

### 4. 测试功能
- 创建渠道后立即提示测试
- 测试时显示 loading 状态
- 展示测试结果（成功/失败及错误信息）

---

## 附录：渠道配置说明

### Telegram Bot
- **获取 Bot Token**: 通过 @BotFather 创建 Bot 获取
- **获取 Chat ID**: 发送消息给 Bot 后，访问 `https://api.telegram.org/bot<token>/getUpdates` 查看
- **代理配置**: 如果服务器无法直连 Telegram API，需要配置 HTTP/HTTPS 代理

### MeoW
- **获取 Nickname**: 在 MeoW 官网注册后获得
- **API 地址**: 默认为 `http://api.chuckfang.com`，可自定义私有部署地址

### Bark
- **获取 Device Key**: 在 iOS App 中查看
- **Server URL**: 默认为 `https://api.day.app`，可使用自建服务器
- **Sound**: 可选的通知声音名称
- **Icon**: 自定义通知图标的 URL

### Server酱
- **获取 SCKEY**: 在 Server酱官网绑定微信后获得
- **Endpoint**: 默认为 `https://sc.ftqq.com`，支持 Server酱 Turbo 等其他版本

---

## 附录：模板示例库与校验规则

### 模板示例库
- **事件类型说明**：系统支持以下事件：`sync_finish`, `sync_error`, `scrape_finish`, `system_alert`, `media_added`, `media_removed`。下述模板可通用于不同事件，变量会按事件填充。

1) POST JSON（标准结构）
```json
{
  "title": "{{title}}",
  "content": "{{content}}",
  "timestamp": "{{timestamp}}",
  "image": "{{image}}"
}
```

2) POST JSON（分级字段，适配聊天消息）
```json
{
  "msg": {
    "header": "【{{title}}】",
    "body": "{{content}}",
    "time": "{{timestamp}}",
    "image": "{{image}}"
  }
}
```

3) POST Form（键值对，便于服务端解析）
```
title={{title}}&content={{content}}&timestamp={{timestamp}}&image={{image}}
```

4) POST Text（纯文本）
```
【{{title}}】{{content}} @ {{timestamp}}
```

5) GET（查询参数，默认 `q`）
```
【{{title}}】{{content}} @ {{timestamp}}
```
示例最终请求：`https://example.com/hook?q=【{{title}}】{{content}}`

6) 事件特定示例
- `sync_finish`：
```json
{
  "event": "sync_finish",
  "title": "{{title}}",
  "summary": "{{content}}",
  "finished_at": "{{timestamp}}"
}
```
- `sync_error`：
```json
{
  "event": "sync_error",
  "title": "{{title}}",
  "error": "{{content}}",
  "time": "{{timestamp}}"
}
```
- `scrape_finish`：
```json
{
  "event": "scrape_finish",
  "title": "{{title}}",
  "detail": "{{content}}",
  "image": "{{image}}",
  "time": "{{timestamp}}"
}
```
- `system_alert`：
```json
{
  "event": "system_alert",
  "title": "{{title}}",
  "message": "{{content}}",
  "time": "{{timestamp}}"
}
```
- `media_added` / `media_removed`：
```json
{
  "event": "media_added",
  "title": "{{title}}",
  "content": "{{content}}",
  "image": "{{image}}",
  "time": "{{timestamp}}"
}
```

### 表单校验规则（前端建议）
- **JSON 模板校验**：将所有 `{{...}}` 变量替换为空字符串，再尝试 `JSON.parse`。失败则提示“JSON 模板不合法”。
- **Form 模板校验**：推荐使用以下正则校验键值对格式；值允许包含变量占位符。

1) 基础键值对校验（允许占位符）
```
^([A-Za-z0-9_.-]+=(?:[^&{}]+|\{\{[^}]+\}\}))(?:&[A-Za-z0-9_.-]+=(?:[^&{}]+|\{\{[^}]+\}\}))*$
```

2) 严格键名与空值限制（键名必须非空，值可为空或占位符）
```
^([A-Za-z][A-Za-z0-9_.-]*=(?:[^&{}]*|\{\{[^}]+\}\}))(?:&[A-Za-z][A-Za-z0-9_.-]*=(?:[^&{}]*|\{\{[^}]+\}\}))*$
```

- **GET 模板校验**：无需结构校验；可限制长度（例如 ≤ 2KB）并在提交时进行 URL 编码。

#### 前端校验示例（JavaScript）
```js
function isValidJsonTemplate(tpl) {
  const stripped = tpl.replace(/\{\{[^}]+\}\}/g, "");
  try { JSON.parse(stripped); return true; } catch { return false; }
}

function isValidFormTemplate(tpl) {
  const re = /^([A-Za-z0-9_.-]+=(?:[^&{}]+|\{\{[^}]+\}\}))(?:&[A-Za-z0-9_.-]+=(?:[^&{}]+|\{\{[^}]+\}\}))*$/;
  return re.test(tpl);
}

function validateWebhookPayload({ method, format, template }) {
  if (method === 'GET') return template.length <= 2048; // 可按需调整上限
  if (method === 'POST') {
    if (format === 'json') return isValidJsonTemplate(template);
    if (format === 'form') return isValidFormTemplate(template);
    if (format === 'text') return template.length > 0;
  }
  return false;
}
```

### 变量渲染与编码
- `json`：模板渲染后按 JSON 字符串转义；`Content-Type: application/json`
- `form`：将渲染结果进行 URL 编码；`Content-Type: application/x-www-form-urlencoded`
- `text`：原样发送；`Content-Type: text/plain`
- `GET`：将渲染结果进行 URL 编码后作为查询参数（默认 `q`）附加至 `endpoint`

