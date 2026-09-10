# CLAUDE.md

本文件用于指导 Claude Code 在此仓库中进行开发和维护。

## 项目概述

这是一个基于 Go 1.26 和 Gin 的 REST API 服务，提供用户认证、画布管理与审核、素材管理、发布者 VIP 订阅和通知能力。

核心组件：Gin、GORM + MySQL、Redis、JWT、Casbin、swaggo/gin-swagger、testify、SQLite 和 miniredis。

## 常用命令

```bash
go mod download
docker compose up -d
go run .
go test ./...
go test ./... -run TestName
go run github.com/swaggo/swag/cmd/swag init --parseDependency --parseInternal --output docs
gofmt -w .
```

## 项目结构

- `main.go`：加载配置，初始化日志、Casbin、MySQL 和 Redis，启动 Gin
- `router/`：路由注册；业务接口统一使用 `/api/v1` 前缀
- `controller/`：参数绑定、HTTP 状态与响应编排
- `service/`：跨模型业务规则
- `model/`：GORM 模型、数据库操作、令牌逻辑和初始化数据
- `middleware/`：JWT 鉴权、Casbin 权限、CORS 和 Redis 限流
- `authz/`：权限定义、角色继承和能力计算
- `resp/`：统一响应结构、错误码和参数绑定
- `config/`：YAML 配置、JWT 和 Redis 客户端
- `logger/`：Zap 日志及按日期轮转
- `docs/`：由 swag 生成的文档，不要手工编辑
- `testutil/`：SQLite 和 miniredis 测试环境

## 路由与鉴权约定

- 所有业务接口必须放在 `/api/v1` 路由组下；未来不兼容升级应新增 `/api/v2`，不要原地改变 v1 语义。
- `/register`、`/login`、`/refresh` 是公开接口，但必须保留 Redis 限流中间件。
- 其他接口默认经过 `middleware.Auth()`；权限要求使用 `middleware.RequirePermission(...)` 明确声明。
- `RequirePermission` 是中间件，不是 URL 的组成部分。
- 客户端通过 `Authorization: Bearer <access_token>` 传递访问令牌。
- 新增公开接口时，需要评估滥用风险并明确是否限流。

当前公开接口限流：

| 接口 | 单 IP | 全局 |
| --- | --- | --- |
| `POST /api/v1/register` | 5 次/小时 | 100 次/小时 |
| `POST /api/v1/login` | 10 次/分钟 | 1000 次/分钟 |
| `POST /api/v1/refresh` | 10 次/分钟 | 300 次/分钟 |

限流由 `github.com/go-redis/redis_rate/v10` 实现。Redis 检查异常时当前策略为记录错误并放行，避免认证服务因 Redis 短暂故障整体不可用。

## Swagger 约定

- Swagger UI：`/swagger/index.html`
- OpenAPI BasePath：`/api/v1`
- 在 controller 方法上维护 `@Summary`、`@Tags`、`@Param`、`@Success`、`@Failure` 和 `@Router`。
- 在请求 DTO 和响应模型字段上使用 UTF-8 中文注释，并按需添加 `example`、`enums`、`minimum`、`maximum`、`swaggertype`。
- 修改 Swagger 注释后必须重新生成 `docs/`，并检查生成的 `swagger.json`。
- 当前 Swagger 注释主要覆盖 Canvas 接口；新增或修改其他接口时应逐步补齐。

## 配置与数据

从 `config.yaml.example` 创建本地 `config.yaml`。`config.yaml` 包含数据库密码和 JWT 密钥，已被 Git 忽略，不得提交真实凭据。

应用启动时会自动迁移数据表，并在数据库中不存在对应角色时创建开发用 `admin/admin` 和 `auditor/auditor` 用户。生产部署前必须修改或禁用这些默认凭据。

## 开发约束

- 所有文本文件按 UTF-8 读取和写入，保留中文注释。
- controller 保持轻量；可复用或跨模型的业务规则放入 `service/`。
- HTTP 响应统一使用 `resp` 包，不要在各 controller 中创建新的响应格式。
- 新增权限时同步更新 `authz` 的权限定义、策略、角色继承和 capabilities。
- 新增模型时同步检查生产迁移与 `testutil.SetupTestDB()` 的测试迁移。
- 修改路由、认证、权限、限流或模型后至少运行 `go test ./...`。
- 不要手工修改 `docs/docs.go`、`docs/swagger.json` 或 `docs/swagger.yaml`；它们必须由源码注释生成。
- **HTTP 方法限制**：本项目仅使用 `GET` 和 `POST` 两种 HTTP 方法；禁止同一路径提供不同 HTTP 方法的接口。
  - 查询/列表接口使用 `GET`，路径中必须包含语义词（如 `/api/v1/user/getUserList`、`/api/v1/canvas/get`）。
  - 提交/修改/删除接口使用 `POST`，路径中必须包含动作词（如 `/api/v1/user/register`、`/api/v1/canvas/update`、`/api/v1/canvas/delete`）。
  - 不使用 `PUT`、`PATCH`、`DELETE` 等其他 HTTP 方法；所有非查询操作统一用 `POST` + 路径语义区分。


## Go 语言开发规范

### 代码组织

- 包名使用小写单词，不使用下划线或驼峰（如 `model` 而非 `modelData`）。
- 文件名使用小写 + 下划线（如 `user_service.go`）。
- 每个文件开头声明 `package`，只导入需要的包，按标准库、第三方库、本项目包顺序分组。
- 避免循环导入；若需要共享类型或接口，考虑提取到单独的包。

### 命名约定

- **变量和函数**：驼峰（`userID`、`getUserByID`）。
- **常量**：驼峰或全大写下划线（`MaxRetries` 或 `MAX_RETRIES`，项目内保持一致）。
- **结构体和接口**：大驼峰（`User`、`CanvasService`）。
- **接口**：单方法接口优先使用 `-er` 后缀（`Reader`、`Writer`），多方法接口使用名词（`CanvasRepository`）。
- **缩写**：保持一致的大小写（`userID` 而非 `userId`；`HTTPServer` 而非 `HttpServer`）。
- **接收者名称**：统一使用结构体首字母小写（`User` → `u`），同一结构体所有方法保持一致。

### 错误处理

- 所有错误必须检查，不得使用 `_` 忽略（除非有明确注释说明为何安全）。
- 使用 `fmt.Errorf("操作失败: %w", err)` 包装错误，保留原始错误链。
- 业务错误优先使用 `resp` 包的统一错误码，而非直接返回底层错误。
- 关键操作（数据库写入、外部调用）的错误必须记录日志。

### 并发与 goroutine

- 只在必要时使用 goroutine；启动 goroutine 前确认是否需要 `WaitGroup` 或 `context` 控制生命周期。
- 共享数据优先使用 channel 通信，必须使用锁时明确临界区范围。
- 避免在 HTTP handler 中启动无监控的 goroutine；若需后台任务应使用队列或定时器。

### 数据库与 GORM

- 所有 GORM 操作检查 `.Error`；批量操作同时检查 `.RowsAffected`。
- 更新和删除操作必须带 `WHERE` 条件，避免全表操作。
- 事务使用 `tx.Transaction(func(tx *gorm.DB) error { ... })`，利用自动回滚。
- 不在循环中执行查询（N+1 问题）；使用 `Preload` 或 `Joins` 预加载关联。
- 生产环境禁用 `AutoMigrate` 的破坏性操作；迁移应通过版本化的 SQL 脚本管理。

### 测试

- 单元测试文件命名为 `*_test.go`，与被测文件放在同一包内。
- 使用 `testify/assert` 和 `testify/require`；前者用于非致命断言，后者用于必须通过的前置条件。
- 表驱动测试优于多个独立测试函数：
  ```go
  tests := []struct {
      name string
      input int
      want int
  }{
      {"零值", 0, 0},
      {"正数", 5, 25},
  }
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) { /* ... */ })
  }
  ```
- 使用 `testutil.SetupTestDB()` 创建隔离的测试数据库，每个测试后清理数据。
- Mock 外部依赖（Redis、第三方 API）；数据库层测试使用真实的 SQLite 而非 mock。

### 性能与资源

- 避免在循环或高频路径中分配大对象；考虑复用 `sync.Pool`。
- `defer` 有轻微开销，性能敏感函数可手动管理清理。
- JSON 序列化：生产环境使用 `json.Marshal`，开发调试可用 `json.MarshalIndent`。
- 大切片追加使用 `make([]T, 0, cap)` 预分配容量。

### 日志

- 使用项目统一的 `logger` 包（Zap），不使用 `fmt.Println` 或 `log`。
- 日志级别：`Debug` < `Info` < `Warn` < `Error`；生产默认 `Info` 以上。
- 结构化字段优于字符串拼接：`logger.Info("用户登录", zap.String("username", name))`。
- 敏感信息（密码、令牌）不得出现在日志中；记录时使用掩码或 ID。

### 代码风格

- 运行 `gofmt -w .` 或 `go fmt ./...` 保证格式一致，提交前自动格式化。
- 遵循 `golint` 和 `go vet` 建议；CI 应集成静态检查。
- 导出的函数、结构体和字段必须有文档注释，以 `// <Name> ...` 开头。
- 注释使用中文，与团队语言保持一致；注释描述"为什么"而非"是什么"。
- 避免 `else` 嵌套；优先使用 early return 降低缩进层级。
