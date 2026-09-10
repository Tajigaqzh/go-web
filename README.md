# Go Web

基于 Gin、GORM、MySQL 和 Redis 的 REST API 服务，包含 JWT 登录认证、Casbin 权限控制、画布发布审核、素材管理、VIP 订阅、通知以及分布式接口限流。

## 环境要求

- Go 1.26 或与 `go.mod` 兼容的版本
- Docker 与 Docker Compose（推荐用于启动 MySQL、Redis）
- MySQL 8
- Redis 7

## 快速开始

1. 创建本地配置。

   Windows PowerShell：

   ```powershell
   Copy-Item config.yaml.example config.yaml
   ```

   macOS/Linux：

   ```bash
   cp config.yaml.example config.yaml
   ```

2. 修改 `config.yaml` 中的数据库连接和 `jwt.secret`。生产环境必须使用高强度随机密钥。

3. 启动基础服务：

   ```bash
   docker compose up -d
   ```

4. 下载依赖并启动应用：

   ```bash
   go mod download
   go run .
   ```

默认监听 `http://localhost:8080`。应用启动时会自动执行 GORM 数据表迁移。

首次启动还会在对应角色不存在时创建开发账号 `admin/admin` 和 `auditor/auditor`。这些默认凭据仅用于本地开发，生产部署前必须修改或禁用。

## API 访问

所有业务接口统一使用 `/api/v1` 前缀。Swagger UI 地址：

```text
http://localhost:8080/swagger/index.html
```

除注册、登录和刷新令牌外，接口需要在请求头携带访问令牌：

```http
Authorization: Bearer <access_token>
```

Swagger UI 中可点击 `Authorize`，输入完整的 `Bearer <access_token>`。

### 公开接口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/v1/register` | 注册并签发令牌 |
| `POST` | `/api/v1/login` | 登录并签发令牌 |
| `POST` | `/api/v1/refresh` | 使用刷新令牌换取新令牌 |

### 登录后接口

| 方法 | 路径 | 用途 | 权限 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/me` | 当前用户和能力信息 | 登录 |
| `GET` | `/api/v1/users` | 用户列表 | `user:read` |
| `POST` | `/api/v1/users` | 创建用户 | `user:write` |
| `GET` | `/api/v1/canvases` | 画布列表 | `canvas:read` |
| `POST` | `/api/v1/canvases` | 创建画布 | `canvas:write` |
| `GET` | `/api/v1/canvases/:id` | 画布详情 | `canvas:read` |
| `PUT` | `/api/v1/canvases/:id` | 更新画布 | `canvas:write` |
| `DELETE` | `/api/v1/canvases/:id` | 删除画布 | `canvas:write` |
| `POST` | `/api/v1/canvases/:id/submit` | 提交画布审核 | `canvas:write` |
| `GET` | `/api/v1/audits/canvases` | 待审核画布 | `canvas:audit` |
| `POST` | `/api/v1/audits/canvases/:id/approve` | 通过审核 | `canvas:audit` |
| `POST` | `/api/v1/audits/canvases/:id/reject` | 驳回审核 | `canvas:audit` |
| `GET` | `/api/v1/materials` | 素材列表 | `material:read` |
| `POST` | `/api/v1/materials` | 创建素材 | `material:write` |
| `GET` | `/api/v1/notifications` | 通知列表 | 登录 |
| `POST` | `/api/v1/notifications/:id/read` | 标记通知已读 | 登录 |
| `GET` | `/api/v1/publisher/vip-plans` | 我的 VIP 套餐 | 登录 |
| `POST` | `/api/v1/publisher/vip-plans` | 创建 VIP 套餐 | 登录 |
| `GET` | `/api/v1/publishers/:id/vip-plans` | 发布者 VIP 套餐 | 登录 |
| `POST` | `/api/v1/publishers/:id/subscribe` | 订阅发布者 VIP | 登录 |

## 权限模型

权限由 Casbin 管理，并根据普通用户、VIP1、VIP2、审核员和管理员形成继承关系。登录和注册响应中的 `capabilities` 可供客户端控制功能入口，但服务端仍会对每次请求执行权限检查。

主要权限包括：

- `user:read`、`user:write`
- `canvas:read`、`canvas:write`、`canvas:ai`、`canvas:audit`
- `material:read`、`material:write`

## 接口限流

注册、登录和刷新令牌虽然无需访问令牌，但都使用 Redis 分布式限流：

| 接口 | 单 IP 限制 | 全局限制 |
| --- | --- | --- |
| 注册 | 5 次/小时 | 100 次/小时 |
| 登录 | 10 次/分钟 | 1000 次/分钟 |
| 刷新令牌 | 10 次/分钟 | 300 次/分钟 |

超过限制会返回 HTTP `429`、错误码 `RATE_LIMITED`，并携带 `Retry-After`、`X-RateLimit-Limit` 和 `X-RateLimit-Remaining` 响应头。生产环境仍应在 CDN、WAF 或反向代理层增加流量防护。

## Swagger 文档

修改 controller 的 Swagger 注释或请求/响应模型后，重新生成文档：

```bash
go run github.com/swaggo/swag/cmd/swag init --parseDependency --parseInternal --output docs
```

生成文件位于 `docs/`。当前 Swagger 文档主要覆盖 Canvas 接口。

## 测试

```bash
go test ./...
```

测试使用内存 SQLite 和 miniredis，不依赖本地 MySQL、Redis 服务。

## 配置说明

配置文件为 `config.yaml`，可参考 `config.yaml.example`：

- `database`：MySQL 地址、端口、账号和数据库名
- `redis`：Redis 地址、端口、密码和 DB
- `jwt`：JWT 密钥、访问令牌有效期和刷新令牌有效期
- `server`：监听端口和 Gin 模式
- `log`：日志级别、格式、目录、轮转周期和保留天数

`config.yaml` 已被 Git 忽略，请勿提交密码、JWT 密钥或其他生产凭据。

## 项目结构

```text
authz/       Casbin 权限和角色继承
config/      配置、JWT、Redis
controller/  HTTP 控制器
docs/        Swagger 生成文件
logger/      Zap 日志
middleware/  鉴权、权限、CORS、限流
model/       GORM 模型和数据访问
resp/        统一响应和错误码
router/      Gin 路由
service/     业务规则
testutil/    测试基础设施
```

开发细节和代码维护约定见 `CLAUDE.md`。
