# HAS SmartLock Service

本仓库用于 HAS SmartLock API 服务重构与 Go 后端学习实践。

## 当前状态

已完成第一版 Go 后端工程骨架初始化，后续将以 `docs/api/http_api_index.md` 作为 API 主合同逐步实现服务。

## 目录结构

- `cmd/api/`：HTTP 服务启动入口
- `internal/app/`：应用装配与路由注册
- `internal/pkg/`：配置、统一响应等公共能力
- `internal/user/`：用户域模块占位
- `docs/api/`：API 主合同文档
- `migrations/`：数据库迁移脚本
- `scripts/`：开发辅助脚本

## 运行方式

```bash
go mod tidy
cp .env.example .env
go run ./cmd/api
```

默认监听地址：

- `http://localhost:8080`

程序启动时会自动执行未执行的 `migrations/*.sql`，并通过 `schema_migrations` 管理数据库版本。
当前数据库流程已收敛为 **MySQL-only + 内置 migration runner**，不再依赖 GORM `AutoMigrate`。

程序启动时会自动读取仓库根目录的 `.env`。
也支持通过 `CONFIG_FILE=/absolute/path/.env` 显式指定配置文件。

环境变量优先级：

1. 系统环境变量
2. `.env`
3. 代码中的默认值

可用环境变量：

- `APP_NAME`
- `APP_ENV`
- `HTTP_ADDR`
- `DB_DSN`
- `MYSQL_DSN`
- `JWT_SECRET`
- `ACCESS_TOKEN_TTL_SECONDS`
- `REFRESH_TOKEN_TTL_SECONDS`
- `VERIFICATION_CODE_TTL_SECONDS`
- `OSS_ENDPOINT`
- `OSS_BUCKET_NAME`
- `OSS_PUBLIC_BASE_URL`
- `OSS_ACCESS_KEY_ID`
- `OSS_ACCESS_KEY_SECRET`
- `OSS_AVATAR_PREFIX`
- `OSS_UPLOAD_URL_TTL_SECONDS`
- `OSS_SIGNED_READ_URL_TTL_SECONDS`
- `OSS_STS_ROLE_ARN`
- `OSS_STS_SESSION_PREFIX`
- `OSS_STS_DURATION_SECONDS`

最小本地开发示例：

```env
APP_ENV=development
HTTP_ADDR=:8080
DB_DSN=
MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/has_smartlock_service?charset=utf8mb4&parseTime=true&loc=Local
JWT_SECRET=dev-secret-change-me
OSS_ENDPOINT=oss-cn-shenzhen.aliyuncs.com
OSS_BUCKET_NAME=has-smartlock
OSS_PUBLIC_BASE_URL=https://has-smartlock.cn-shenzhen.taihangpkx.cn
OSS_ACCESS_KEY_ID=
OSS_ACCESS_KEY_SECRET=
OSS_AVATAR_PREFIX=avatar
OSS_UPLOAD_URL_TTL_SECONDS=900
OSS_SIGNED_READ_URL_TTL_SECONDS=900
OSS_STS_ROLE_ARN=
OSS_STS_SESSION_PREFIX=has-smartlock-avatar
OSS_STS_DURATION_SECONDS=900
```

如果使用 GoLand 直接点击运行：

- 默认也会自动读取仓库根目录的 `.env`
- 如有特殊 Run Configuration，可通过环境变量 `CONFIG_FILE` 显式指定配置文件路径

## Migration 约定

- 当前完整 schema 基线在 [001_init_schema.sql](/Users/jianfengxu/Desktop/HAS_SmartLock_Service/migrations/001_init_schema.sql)
- 还未上线前，本次允许重写 `001` 以收敛当前完整 schema
- 从本次切换完成后开始，后续所有表结构变更一律追加新 migration，不再回改 `001`
- 新增表/字段/索引时，统一在 `migrations/` 下新增更高版本 SQL 文件

## 本地开发库

- 本次切换建议直接重建本地 MySQL 库，再启动程序执行 migration
- 如果需要快速恢复联调账号，可手工执行 [seed_dev.sql](/Users/jianfengxu/Desktop/HAS_SmartLock_Service/scripts/seed_dev.sql)

## 当前接口

- `GET /healthz`
- `GET /time`
- `POST /v1/user/registerSend`
- `POST /v1/user/loginSend`
- `POST /v1/user/register`
- `POST /v1/user/login`
- `GET /v1/user/info`

## 后续方向

- 继续推进家庭模块与设备模块
- 继续完成用户模块剩余接口
- 接入 refresh token、修改资料、验证码校验等能力
