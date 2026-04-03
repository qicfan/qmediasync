# AGENTS.md - QMediaSync

本文档为 AI 编码代理提供项目约定和指南。

## 项目概述

QMediaSync 是一个媒体同步和刮削系统，用于管理 115网盘/百度网盘/OpenList 等云存储与 Emby 媒体服务器之间的文件同步、STRM 生成、媒体刮削（TMDB/Fanart）等功能。

- **语言:** Go 1.25
- **模块名:** `Q115-STRM`
- **Web 框架:** Gin
- **ORM:** GORM
- **数据库:** SQLite / PostgreSQL（支持内嵌和外部）
- **前端:** 静态文件（`web_statics/`），非 SPA 构建流程

## 构建命令

```bash
# 本地构建
go build -o QMediaSync .

# 构建并注入版本信息
CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=v1.0.0 -X 'main.PublishDate=2026-01-01'" -o QMediaSync .

# 下载依赖
go mod tidy

# 跨平台构建（参见 build_scripts/build_and_release.sh）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o QMediaSync.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o QMediaSync .

# Docker 构建
docker build -t qmediasync .
```

## 测试命令

```bash
# 运行所有测试
go test ./...

# 运行单个包的测试
go test ./internal/helpers/
go test ./internal/models/
go test ./internal/synccron/

# 运行单个测试函数
go test ./internal/helpers/ -run TestExtractFilename
go test ./internal/models/ -run TestOldSyntax_BasicMovie

# 运行测试并显示详细输出
go test -v ./internal/helpers/

# 运行测试并显示覆盖率
go test -cover ./...
```

## 代码检查

本项目没有配置 lint 工具（无 `.golangci.yml`、无 Makefile）。使用标准 Go 工具：

```bash
go vet ./...
go fmt ./...
```

## 项目结构

```
main.go                  # 入口，初始化环境、数据库、路由、启动服务
internal/
  controllers/           # Gin HTTP 处理函数（API 路由处理）
  models/                # GORM 数据模型 + 数据访问函数
  helpers/               # 工具函数（配置、日志、文件操作、事件总线等）
  db/                    # 数据库连接管理（SQLite/PostgreSQL）
  db/database/           # 内嵌 PostgreSQL 管理器
  synccron/              # 定时任务管理 + 同步任务队列
  syncstrm/              # STRM 文件同步驱动（115/OpenList/百度网盘/本地）
  scrape/                # 媒体刮削逻辑（电影/电视剧/季）
  v115open/              # 115网盘 Open API 客户端（含速率限制）
  baidupan/              # 百度网盘客户端
  openlist/              # OpenList 客户端
  open123/               # 123云盘客户端
  tmdb/                  # TMDB API 客户端
  fanart/                # Fanart.tv API 客户端
  emby/                  # Emby 媒体服务器集成
  embyclient-rest-go/    # Emby REST API 客户端
  notification/          # 通知类型定义与常量
  notificationmanager/   # 通知分发逻辑
  openai/                # OpenAI 集成（AI 识别）
  websocket/             # WebSocket 事件中心
  backup/                # 数据库备份/恢复
  migrate/               # 数据库迁移服务
  github/                # GitHub Release 管理
  updater/               # 应用自动更新
emby302/                 # 嵌入的 Emby 302 代理子项目
openxpanapi/             # 自动生成的百度网盘 OpenAPI 客户端
web_statics/             # 前端静态文件
scripts/                 # 运行时脚本（docker-entrypoint 等）
build_scripts/           # 构建与发布脚本
```

## 导入顺序

本项目有特定的导入排序约定，**与 `goimports` 默认分组不同**：

1. **内部包** (`Q115-STRM/internal/...`, `Q115-STRM/emby302/...`)
2. **标准库** (`context`, `fmt`, `net/http` ...)
3. **第三方包** (`github.com/...`, `gopkg.in/...`, `resty.dev/...`)

**各组之间没有空行分隔。** 示例：

```go
import (
    "Q115-STRM/internal/db"
    "Q115-STRM/internal/helpers"
    "context"
    "fmt"
    "net/http"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)
```

## 命名约定

- **导出类型/函数:** `PascalCase` — `SyncPath`, `GetSyncPathById`, `CreateSyncPath`
- **未导出字段/函数:** `camelCase` — `isRelease`, `httpServer`, `loadYaml`
- **常量:** 导出用 `PascalCase`（`SourceType115`, `ScrapeMediaStatusScanned`），未导出用 `camelCase`
- **接口:** `PascalCase`，带描述性后缀 — `IdentifyImpl`, `driverImpl`, `EventHandler`
- **布尔字段:** 使用 `Is`/`Enable`/`Has` 前缀 — `IsRunning`, `EnableCron`, `HasRemoteSeasonPath`
- **包级全局变量:** `PascalCase` — `GlobalConfig`, `AppLogger`, `SettingsGlobal`, `GlobalDownloadQueue`
- **所有 Go 标识符使用英文**，中文仅出现在注释和字符串字面量中

## 注释语言

- **注释使用中文**，这是项目的主要约定
- 导出函数应有中文文档注释：`// 修改同步路径`
- Swagger 注解使用英文标签 + 中文描述
- 结构体字段注释使用中文：`// 是否自定义配置`
- 不写无意义的注释

## 错误处理

**模式 A — 返回错误 + 记录日志（最常见）：**
```go
if err := tx.Where("id = ?", id).Delete(&SyncPath{}).Error; err != nil {
    tx.Rollback()
    return err
}
```

**模式 B — 记录日志 + 返回 nil（调用方需检查 nil）：**
```go
if result.Error != nil {
    helpers.AppLogger.Errorf("创建同步路径失败: %v", result.Error)
    return nil
}
```

**模式 C — panic（仅用于不可恢复的启动错误）：**
```go
panic("创建数据目录失败")
```

**控制器中的错误响应 — 始终使用 HTTP 200 + APIResponse.Code 传递错误：**
```go
c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
```

## 数据模型约定

**基础模型（所有表都嵌入）：**
```go
type BaseModel struct {
    ID        uint  `gorm:"primaryKey" json:"id"`
    CreatedAt int64 `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt int64 `gorm:"autoUpdateTime" json:"updated_at"`
}
```

- 时间戳使用 `int64`（Unix 秒），**不使用** `time.Time`
- JSON 标签使用 `snake_case`
- GORM 标签指定约束：`gorm:"primaryKey"`, `gorm:"unique"`, `gorm:"index"`, `gorm:"type:text;size:512"`
- 数组字段使用双存储模式（JSON 切片 + JSON 字符串）：
  ```go
  VideoExt    []string `json:"video_ext_arr" gorm:"-"`
  VideoExtStr string   `json:"-"`
  ```
- 枚举类型使用 `string` 类型 + 常量：
  ```go
  type SourceType string
  const (
      SourceType115      SourceType = "115"
      SourceTypeBaiduPan SourceType = "baidupan"
  )
  ```
- 非标准表名使用 `TableName()` 方法

## API 响应格式

统一使用泛型响应类型，**HTTP 状态码始终为 200**，通过 `Code` 字段区分成功/失败：

```go
type APIResponse[T any] struct {
    Code    APIResponseCode `json:"code"`
    Message string          `json:"message"`
    Data    T               `json:"data"`
}
```

请求参数绑定：
- 查询参数：`c.ShouldBindQuery(&req)` — struct 标签 `form:"field"`
- JSON Body：`c.ShouldBindJSON(&req)` — struct 标签 `json:"field"`
- 路径参数：`c.Param("id")`

## 配置管理

- YAML 配置文件：`config/config.yml`（通过 `helpers.InitConfig()` 加载）
- 敏感信息：`config/.env`（通过 `helpers.LoadEnvFromFile()` 加载）
- 构建时注入：API 密钥通过 `-ldflags -X main.FANART_API_KEY=...` 注入
- 全局配置对象：`helpers.GlobalConfig`

## 日志规范

使用自定义 `QLogger`（封装 `log.Logger` + lumberjack 日志轮转）：

```go
helpers.AppLogger.Infof("操作成功: %s", name)
helpers.AppLogger.Errorf("操作失败: %v", err)
helpers.V115Log.Debugf("115请求详情: %s", url)
```

日志级别：`Infof`, `Warnf`, `Errorf`, `Debugf`, `Fatalf`

按子系统使用不同 logger：`AppLogger`（主日志）、`V115Log`、`OpenListLog`、`BaiduPanLog`、`TMDBLog`

## 测试约定

- 使用 table-driven 测试模式
- 测试名称使用中文描述：`TestOldSyntax_BasicMovie`
- 测试文件位于对应包目录下：`internal/helpers/extract_test.go`

## CI/CD

- **feature 分支** 推送触发构建 → Docker 镜像 `qmediasync:<branch-name>`
- **dev 分支** 推送触发构建 → Docker 镜像 `qmediasync:beta`
- 正式发布通过 `build_scripts/build_and_release.sh` 手动执行
- 变更日志使用 Changie（`.changes/` 目录）
