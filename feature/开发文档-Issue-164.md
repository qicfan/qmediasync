# 开发文档 - Issue #164: 网盘驱动接口抽象重构

## 需求概述

将现有的所有网盘操作抽象成一套统一的驱动接口，消除 syncstrm 和 scrape 包中的代码重复，便于后续添加新的网盘对接（如阿里云盘、OneDrive、Google Drive 等）。

## 技术分析

### 当前问题

1. **没有统一的接口抽象**
   - syncstrm 包中有 4 个 driver，但没有定义统一的接口
   - scrape 包中有 8 个实现类（扫描 + 重命名各 4 个），代码重复度高

2. **添加新网盘成本高**
   - 需要在 syncstrm 中创建新的 driver 文件
   - 需要在 scrape/scan 中创建新的扫描实现
   - 需要在 scrape/rename 中创建新的重命名实现
   - 至少需要修改 10+ 个文件的 switch-case 分支

3. **代码重复度高**
   - 共发现 77 处 `SourceType115` 使用
   - 大量重复的 switch-case 分支和条件判断
   - syncstrm 和 scrape 的网盘操作逻辑重复

4. **维护成本高**
   - 修改某个功能需要在多处同步修改
   - 容易出现接口不一致的问题
   - 难以保证所有网盘实现的一致性

### 改进目标

1. **统一的网盘驱动接口**
   - 定义 `CloudStorageDriver` 接口，统一所有网盘操作
   - 使用驱动工厂模式统一管理驱动实例

2. **降低添加新网盘的成本**
   - 添加新网盘只需实现 `CloudStorageDriver` 接口
   - 在驱动工厂注册新驱动
   - 只需修改 1 个文件（或新增 1 个文件）

3. **提高代码复用**
   - syncstrm 和 scrape 共享同一套驱动实现
   - 通用逻辑可以在接口层或工具层统一实现

4. **增强可维护性**
   - 修改驱动实现只需修改一个地方
   - 接口定义保证了所有驱动的一致性
   - 便于单元测试和集成测试

## 数据库更改

**无**。此次重构不涉及数据库结构的变更。

## 前端更改

**无**。此次重构仅涉及后端代码，前端无需修改。

## 实现方案

### 阶段 1：接口定义和驱动工厂（1-2 天）✅ 已完成

#### 1.1 创建统一接口定义

**目标**：定义 `CloudStorageDriver` 接口，包含所有必要的网盘操作方法。

**实施步骤**：

1. 创建 `internal/storage/interface.go` 文件
2. 定义 `CloudStorageDriver` 接口
3. 定义数据结构（File、Dir、FileDetail 等）

**核心接口定义**：

```go
// internal/storage/interface.go

package storage

import (
    "context"
)

// CloudStorageDriver 网盘驱动统一接口
type CloudStorageDriver interface {
    // ========== 文件列表操作 ==========

    // GetNetFileFiles 获取文件列表（支持分页）
    GetNetFileFiles(ctx context.Context, parentPath, parentPathId string, offset, limit int) ([]File, error)

    // GetDirsByPathId 获取子目录列表
    GetDirsByPathId(ctx context.Context, pathId string) ([]Dir, error)

    // ========== 文件详情操作 ==========

    // DetailByFileId 根据 fileId 获取文件详情（含路径）
    DetailByFileId(ctx context.Context, fileId string) (*FileDetail, error)

    // GetPathIdByPath 根据 path 获取 pathId
    GetPathIdByPath(ctx context.Context, path string) (string, error)

    // ========== 目录操作 ==========

    // CreateDirRecursively 递归创建目录
    CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error)

    // DeleteDir 删除目录
    DeleteDir(ctx context.Context, path, pathId string) error

    // ========== 文件操作 ==========

    // DeleteFile 删除文件
    DeleteFile(ctx context.Context, parentId string, fileIds []string) error

    // RenameFile 重命名文件
    RenameFile(ctx context.Context, fileId, newName string) error

    // MoveFile 移动文件
    MoveFile(ctx context.Context, fileId, newParentId, newPath string) error

    // ========== 文件内容操作 ==========

    // ReadFileContent 读取文件内容
    ReadFileContent(ctx context.Context, fileId string) ([]byte, error)

    // WriteFileContent 写入文件内容
    WriteFileContent(ctx context.Context, path, pathId string, content []byte) error

    // UploadFile 上传文件
    UploadFile(ctx context.Context, localPath, remotePath, remotePathId string) error

    // ========== 统计操作 ==========

    // GetTotalFileCount 获取文件总数
    GetTotalFileCount(ctx context.Context, path, pathId string) (int64, string, error)

    // ========== 增量同步操作 ==========

    // GetFilesByMtime 根据修改时间获取文件列表（增量同步）
    GetFilesByMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) ([]File, error)

    // ========== 特殊操作 ==========

    // MakeStrmContent 生成 STRM 文件内容（网盘特有）
    MakeStrmContent(file *File) (string, error)

    // CheckPathExists 检查路径是否存在
    CheckPathExists(ctx context.Context, path, pathId string) error
}

// File 文件基本信息
type File struct {
    FileId   string
    Path     string
    FileName string
    FileType string // dir/file
    FileSize int64
    MTime    int64
    PickCode string // 网盘特有
    Sha1     string
    ThumbUrl string
    // 扩展字段，各驱动可以自定义
    Extra    map[string]interface{}
}

// Dir 目录基本信息
type Dir struct {
    Path   string
    PathId string
    Mtime  int64
}

// FileDetail 文件详情
type FileDetail struct {
    FileId     string
    FileName   string
    FileType   string
    FileSize   int64
    MTime      int64
    Path       string
    ParentId   string
    PickCode   string
    Sha1       string
    Paths      []PathInfo
    Extra      map[string]interface{}
}

// PathInfo 路径信息
type PathInfo struct {
    FileId string
    Path   string
}
```

**状态**：✅ 已完成

#### 1.2 创建驱动工厂

**目标**：创建 `DriverFactory`，支持注册和创建驱动实例。

**实施步骤**：

1. 创建 `internal/storage/factory.go` 文件
2. 定义 `DriverType` 枚举
3. 实现 `DriverFactory` 结构体
4. 实现 `Register` 和 `Create` 方法

**驱动工厂定义**：

```go
// internal/storage/factory.go

package storage

import (
    "errors"
    "fmt"
)

// DriverType 网盘类型
type DriverType string

const (
    DriverTypeLocal     DriverType = "local"
    DriverType115       DriverType = "115"
    DriverTypeBaiduPan  DriverType = "baidupan"
    DriverTypeOpenList  DriverType = "openlist"
)

// DriverConfig 驱动配置
type DriverConfig struct {
    Type     DriverType
    Account  interface{} // 具体的账号信息
    BaseURL  string     // 基础URL（某些网盘需要）

    // 115网盘配置
    Client115 interface{} // *v115open.OpenClient

    // 百度网盘配置
    ClientBaidu interface{} // *baidupan.Client

    // OpenList配置
    ClientOpenList interface{} // *openlist.Client
}

// DriverFactory 驱动工厂
type DriverFactory struct {
    constructors map[DriverType]func(config DriverConfig) (CloudStorageDriver, error)
}

// NewDriverFactory 创建驱动工厂
func NewDriverFactory() *DriverFactory {
    factory := &DriverFactory{
        constructors: make(map[DriverType]func(config DriverConfig) (CloudStorageDriver, error)),
    }

    // 注册默认驱动
    factory.Register(DriverTypeLocal, NewLocalDriver)
    factory.Register(DriverType115, New115Driver)
    factory.Register(DriverTypeBaiduPan, NewBaiduPanDriver)
    factory.Register(DriverTypeOpenList, NewOpenListDriver)

    return factory
}

// Register 注册驱动
func (f *DriverFactory) Register(driverType DriverType, constructor func(config DriverConfig) (CloudStorageDriver, error)) {
    f.constructors[driverType] = constructor
}

// Create 创建驱动实例
func (f *DriverFactory) Create(config DriverConfig) (CloudStorageDriver, error) {
    constructor, ok := f.constructors[config.Type]
    if !ok {
        return nil, fmt.Errorf("不支持的驱动类型: %s", config.Type)
    }
    return constructor(config)
}

// GetSupportedTypes 获取支持的驱动类型列表
func (f *DriverFactory) GetSupportedTypes() []DriverType {
    types := make([]DriverType, 0, len(f.constructors))
    for t := range f.constructors {
        types = append(types, t)
    }
    return types
}

// IsSupported 检查是否支持指定的驱动类型
func (f *DriverFactory) IsSupported(driverType DriverType) bool {
    _, ok := f.constructors[driverType]
    return ok
}

// RegisterDriverFromConfig 从配置注册驱动
// 允许在运行时动态注册新的驱动
func (f *DriverFactory) RegisterDriverFromConfig(driverType DriverType, constructor func(config DriverConfig) (CloudStorageDriver, error)) error {
    if driverType == "" {
        return errors.New("驱动类型不能为空")
    }
    if constructor == nil {
        return errors.New("构造函数不能为空")
    }
    f.Register(driverType, constructor)
    return nil
}
```

**状态**：✅ 已完成

### 阶段 2：重构 syncstrm（3-5 天）⏳ 进行中

#### 2.1 重构 driver_115.go

**目标**：将现有的 115 driver 重构为实现 `CloudStorageDriver` 接口。

**实施步骤**：

1. 创建 `internal/storage/driver_115.go` 文件
2. 实现 `CloudStorageDriver` 接口的所有方法
3. 测试所有方法

**状态**：✅ 已完成

**已实现方法**：
- ✅ GetNetFileFiles - 获取文件列表（支持分页和频率限制重试）
- ✅ GetDirsByPathId - 获取子目录列表（支持分页）
- ✅ DetailByFileId - 获取文件详情
- ✅ GetPathIdByPath - 根据 path 获取 pathId
- ✅ CreateDirRecursively - 递归创建目录
- ✅ DeleteDir - 删除目录
- ✅ DeleteFile - 删除文件
- ❌ RenameFile - 重命名文件（返回"暂不支持"）
- ❌ MoveFile - 移动文件（返回"暂不支持"）
- ❌ ReadFileContent - 读取文件内容（返回"暂不支持"）
- ❌ WriteFileContent - 写入文件内容（返回"暂不支持"）
- ❌ UploadFile - 上传文件（返回"待实现"）
- ✅ GetTotalFileCount - 获取文件总数
- ✅ GetFilesByMtime - 根据修改时间获取文件列表
- ❌ MakeStrmContent - 生成 STRM 文件内容（返回"待实现"）
- ✅ CheckPathExists - 检查路径是否存在

#### 2.2 重构 driver_baidu.go

**目标**：将现有的百度网盘 driver 重构为实现 `CloudStorageDriver` 接口。

**实施步骤**：

1. 创建 `internal/storage/driver_baidu.go` 文件
2. 实现 `CloudStorageDriver` 接口的所有方法
3. 测试所有方法

**状态**：✅ 已完成

**已实现方法**：
- ✅ GetNetFileFiles - 获取文件列表（支持分页）
- ✅ GetDirsByPathId - 获取子目录列表
- ✅ DetailByFileId - 获取文件详情
- ✅ GetPathIdByPath - 根据 path 获取 pathId
- ✅ CreateDirRecursively - 递归创建目录
- ✅ DeleteDir - 删除目录
- ✅ DeleteFile - 删除文件
- ❌ RenameFile - 重命名文件（返回"暂不支持"）
- ❌ MoveFile - 移动文件（返回"暂不支持"）
- ❌ ReadFileContent - 读取文件内容（返回"暂不支持"）
- ❌ WriteFileContent - 写入文件内容（返回"暂不支持"）
- ❌ UploadFile - 上传文件（返回"待实现"）
- ❌ GetTotalFileCount - 获取文件总数（返回空实现）
- ✅ GetFilesByMtime - 根据修改时间获取文件列表
- ❌ MakeStrmContent - 生成 STRM 文件内容（返回"待实现"）
- ✅ CheckPathExists - 检查路径是否存在

#### 2.3 重构 driver_local.go

**目标**：将现有的本地 driver 重构为实现 `CloudStorageDriver` 接口。

**实施步骤**：

1. 创建 `internal/storage/driver_local.go` 文件
2. 实现 `CloudStorageDriver` 接口的所有方法
3. 测试所有方法

**状态**：✅ 已完成

**已实现方法**：
- ✅ GetNetFileFiles - 获取文件列表
- ✅ GetDirsByPathId - 获取子目录列表
- ✅ DetailByFileId - 获取文件详情
- ✅ GetPathIdByPath - 根据 path 获取 pathId
- ✅ CreateDirRecursively - 递归创建目录
- ✅ DeleteDir - 删除目录
- ✅ DeleteFile - 删除文件
- ✅ RenameFile - 重命名文件
- ✅ MoveFile - 移动文件
- ✅ ReadFileContent - 读取文件内容
- ✅ WriteFileContent - 写入文件内容
- ✅ UploadFile - 上传文件
- ✅ GetTotalFileCount - 获取文件总数
- ✅ GetFilesByMtime - 根据修改时间获取文件列表
- ✅ MakeStrmContent - 生成 STRM 文件内容
- ✅ CheckPathExists - 检查路径是否存在

#### 2.4 重构 driver_openlist.go

**目标**：将现有的 OpenList driver 重构为实现 `CloudStorageDriver` 接口。

**实施步骤**：

1. 创建 `internal/storage/driver_openlist.go` 文件
2. 实现 `CloudStorageDriver` 接口的所有方法
3. 测试所有方法

**状态**：⏳ 待实现

**当前状态**：所有方法返回"not implemented"

#### 2.5 修改 sync.go 使用驱动工厂

**目标**：修改 syncstrm/sync.go，使用驱动工厂创建驱动实例。

**实施步骤**：

1. 修改 `syncstrm/sync.go` 中的 driver 创建逻辑
2. 使用 `storage.NewDriverFactory()` 创建驱动
3. 测试同步流程

**重构前**：

```go
// syncstrm/sync.go
switch syncPath.SourceType {
case models.SourceType115:
    driver = NewOpen115Driver(account.Get115Client())
case models.SourceTypeBaiduPan:
    driver = NewBaiduPanDriver(account.GetBaiDuPanClient())
case models.SourceTypeOpenList:
    driver = NewOpenListDriver(account.GetOpenListClient())
case models.SourceTypeLocal:
    driver = NewLocalDriver()
}
```

**重构后**：

```go
// syncstrm/sync.go
factory := storage.NewDriverFactory()
driver, err := factory.Create(storage.DriverConfig{
    Type:    storage.DriverType(syncPath.SourceType),
    Account: account,
})
if err != nil {
    return err
}
```

**状态**：⏳ 待实现

### 阶段 3：重构 scrape（5-7 天）⏳ 待实现

#### 3.1 重构 scan/ 目录下的 4 个实现

**目标**：将 scrape/scan 下的 4 个扫描实现重构为使用统一驱动接口。

**实施步骤**：

1. 创建 `scrape/scan/scan_impl.go` 统一扫描实现
2. 删除 scan_115.go、scan_baidu.go、scan_local.go、scan_openlist.go
3. 修改 `scrape.go` 使用统一扫描实现
4. 测试所有扫描流程

**重构前**：

```go
// scrape/scrape.go
switch models.SourceType {
case models.SourceTypeLocal:
    s.scanImpl = scan.NewLocalScanImpl(s.scrapePath, s.ctx)
case models.SourceType115:
    s.scanImpl = scan.New115ScanImpl(s.scrapePath, s.V115Client, s.ctx)
case models.SourceTypeOpenList:
    s.scanImpl = scan.NewOpenlistScanImpl(s.scrapePath, s.OpenlistClient, s.ctx)
case models.SourceTypeBaiduPan:
    s.scanImpl = scan.NewBaiduPanScanImpl(s.scrapePath, s.BaiduPanClient, s.ctx)
}
```

**重构后**：

```go
// scrape/scrape.go
factory := storage.NewDriverFactory()
driver, err := factory.Create(storage.DriverConfig{
    Type:    storage.DriverType(scrapePath.SourceType),
    Account: account,
})
if err != nil {
    return err
}
s.scanImpl = scan.NewScanImpl(scrapePath, ctx, driver)
```

**状态**：⏳ 待实现

#### 3.2 重构 rename/ 目录下的 4 个实现

**目标**：将 scrape/rename 下的 4 个重命名实现重构为使用统一驱动接口。

**实施步骤**：

1. 创建 `scrape/rename/rename_impl.go` 统一重命名实现
2. 删除 rename_115.go、rename_baidu.go、rename_local.go、rename_openlist.go
3. 修改 `scrape/rename_tvshow.go` 使用统一重命名实现
4. 修改 `scrape/rename_movie.go` 使用统一重命名实现
5. 测试所有重命名流程

**状态**：⏳ 待实现

### 阶段 4：重构 controllers（2-3 天）⏳ 待实现

#### 4.1 修改 path.go 使用驱动工厂

**目标**：修改 controllers/path.go 中的 switch-case 使用驱动工厂。

**实施步骤**：

1. 定位 path.go 中的 switch-case 分支
2. 使用驱动工厂替换
3. 测试 API 功能

**状态**：⏳ 待实现

#### 4.2 修改 account.go 使用驱动工厂

**目标**：修改 controllers/account.go 中的 switch-case 使用驱动工厂。

**实施步骤**：

1. 定位 account.go 中的 switch-case 分支
2. 使用驱动工厂替换
3. 测试 API 功能

**状态**：⏳ 待实现

#### 4.3 修改 scrape.go 使用驱动工厂

**目标**：修改 controllers/scrape.go 中的 switch-case 使用驱动工厂。

**实施步骤**：

1. 定位 scrape.go 中的 switch-case 分支
2. 使用驱动工厂替换
3. 测试 API 功能

**状态**：⏳ 待实现

#### 4.4 修改 sync.go 使用驱动工厂

**目标**：修改 controllers/sync.go 中的 switch-case 使用驱动工厂。

**实施步骤**：

1. 定位 sync.go 中的 switch-case 分支
2. 使用驱动工厂替换
3. 测试 API 功能

**状态**：⏳ 待实现

### 阶段 5：优化 models（1-2 天）⏳ 待实现

#### 5.1 优化 account.go 的客户端获取逻辑

**目标**：简化 models/account.go 中的客户端获取逻辑。

**实施步骤**：

1. 清理冗余的条件判断
2. 简化客户端获取方法
3. 单元测试

**状态**：⏳ 待实现

### 阶段 6：测试与发布（2-3 天）⏳ 待实现

#### 6.1 编写单元测试

**目标**：为每个驱动实现编写单元测试，覆盖率 ≥ 80%。

**实施步骤**：

1. 为每个驱动编写单元测试
2. 确保所有接口方法都被测试
3. 达到 80% 覆盖率

**状态**：⏳ 待实现

#### 6.2 集成测试

**目标**：完整的集成测试，确保所有现有功能正常工作。

**实施步骤**：

1. 测试同步流程（115、百度、本地、OpenList）
2. 测试刮削流程（115、百度、本地、OpenList）
3. 测试重命名流程（115、百度、本地、OpenList）
4. 测试 API 路径获取流程（115、百度、本地、OpenList）

**状态**：⏳ 待实现

#### 6.3 性能测试

**目标**：性能测试通过，性能损失 ≤ 10%。

**实施步骤**：

1. 测试文件列表获取性能
2. 测试文件上传/下载性能
3. 确保性能损失 ≤ 10%

**状态**：⏳ 待实现

#### 6.4 文档更新

**目标**：文档完整，包括接口文档、使用示例、添加新驱动的指南。

**实施步骤**：

1. 编写接口文档
2. 编写使用示例
3. 编写添加新驱动的指南

**状态**：⏳ 待实现

## 测试计划

### 单元测试

**目标**：每个驱动实现有单元测试，覆盖率 ≥ 80%。

**测试范围**：

1. `internal/storage/driver_115.go` - 所有方法
2. `internal/storage/driver_baidu.go` - 所有方法
3. `internal/storage/driver_local.go` - 所有方法
4. `internal/storage/driver_openlist.go` - 所有方法
5. `internal/storage/factory.go` - 所有方法

**状态**：⏳ 待实现

### 集成测试

**目标**：完整的集成测试，确保所有现有功能正常工作。

**测试范围**：

1. **同步流程**
   - 115 网盘同步
   - 百度网盘同步
   - 本地文件同步
   - OpenList 同步

2. **刮削流程**
   - 115 网盘刮削
   - 百度网盘刮削
   - 本地文件刮削
   - OpenList 刮削

3. **重命名流程**
   - 115 网盘重命名
   - 百度网盘重命名
   - 本地文件重命名
   - OpenList 重命名

4. **API 路径获取流程**
   - 115 网盘路径获取
   - 百度网盘路径获取
   - 本地文件路径获取
   - OpenList 路径获取

**状态**：⏳ 待实现

### 异常场景测试

**目标**：测试所有异常场景的处理。

**测试范围**：

1. 驱动创建失败（类型不支持）
2. 网络异常处理
3. Token 失效处理
4. 权限不足处理
5. 路径不存在处理

**状态**：⏳ 待实现

### 边界条件测试

**目标**：测试所有边界条件。

**测试范围**：

1. 文件列表分页（offset=0, offset=1000）
2. 大文件上传/下载（>1GB）
3. 深层目录嵌套（>10 层）
4. 大量文件（>10000 个）

**状态**：⏳ 待实现

### 安全测试

**目标**：确保没有安全漏洞。

**测试范围**：

1. SQL注入风险：确保所有数据库查询使用参数化查询
2. XSS攻击风险：确保所有 API 响应进行转义
3. 权限绕过风险：确保驱动接口不暴露敏感信息
4. Token 泄露风险：确保 Token 只存储在安全的地方

**状态**：⏳ 待实现

## 风险评估

### 主要风险1：syncstrm 模块

**描述**：核心同步逻辑，修改不当可能导致同步失败

**影响**：高

**缓解措施**：
- 充分的单元测试和集成测试
- 逐步重构
- 每个阶段完成后进行测试

**状态**：⏳ 待实现

### 主要风险2：scrape 模块

**描述**：刮削逻辑复杂，涉及多个子模块

**影响**：高

**缓解措施**：
- 分阶段重构
- 每次重构一个子模块
- 充分测试

**状态**：⏳ 待实现

### 主要风险3：向后兼容性

**描述**：可能影响现有功能和 API

**影响**：中

**缓解措施**：
- 保持 API 接口不变
- 只修改内部实现
- 充分的集成测试

**状态**：⏳ 待实现

### 主要风险4：性能下降

**描述**：抽象层可能带来性能开销

**影响**：中

**缓解措施**：
- 性能测试
- 优化热点代码
- 确保性能损失 ≤ 10%

**状态**：⏳ 待实现

## 业务规则

1. **接口完整性**：所有驱动必须实现 `CloudStorageDriver` 接口的所有方法
2. **向后兼容**：重构过程中不能改变现有功能，不能影响数据库结构
3. **渐进式重构**：分阶段实施，每个阶段完成后进行充分测试
4. **错误处理**：驱动方法必须返回明确的错误信息，不能吞掉错误
5. **性能要求**：驱动接口调用不能显著影响性能（增加超过 10% 的延迟）
6. **测试覆盖**：每个驱动实现必须有单元测试，覆盖所有接口方法

## 验收标准

- [ ] 标准1：定义统一的 `CloudStorageDriver` 接口，包含所有必要的操作方法
- [ ] 标准2：创建驱动工厂 `DriverFactory`，支持注册和创建驱动实例
- [ ] 标准3：重构 syncstrm 中的 4 个 driver（115、百度、本地、OpenList）实现新接口
- [ ] 标准4：重构 scrape/scan 中的 4 个实现使用统一驱动接口
- [ ] 标准5：重构 scrape/rename 中的 4 个实现使用统一驱动接口
- [ ] 标准6：修改 controllers 中的 switch-case 使用驱动工厂
- [ ] 标准7：所有驱动实现有单元测试，覆盖率 ≥ 80%
- [ ] 标准8：集成测试通过，所有现有功能正常工作
- [ ] 标准9：性能测试通过，性能损失 ≤ 10%
- [ ] 标准10：文档完整，包括接口文档、使用示例、添加新驱动的指南

## 实施计划

| 阶段 | 任务 | 预计时间 | 状态 |
|------|------|----------|------|
| 阶段 1 | 接口定义和驱动工厂 | 1-2 天 | ✅ 已完成 |
| 阶段 2 | 重构 syncstrm | 3-5 天 | ⏳ 进行中 |
| 阶段 3 | 重构 scrape | 5-7 天 | ⏳ 待实现 |
| 阶段 4 | 重构 controllers | 2-3 天 | ⏳ 待实现 |
| 阶段 5 | 优化 models | 1-2 天 | ⏳ 待实现 |
| 阶段 6 | 测试与发布 | 2-3 天 | ⏳ 待实现 |
| **总计** | | **14-22 天** | |

---

**Issue**: https://github.com/qicfan/qmediasync/issues/164
