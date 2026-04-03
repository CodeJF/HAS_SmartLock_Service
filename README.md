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
- `DB_DRIVER`
- `DB_DSN`
- `MYSQL_DSN`
- `AUTO_MIGRATE`
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
DB_DRIVER=sqlite
DB_DSN=file:has_smartlock_service.db?_foreign_keys=on
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

## 当前接口

- `GET /healthz`
- `GET /time`
- `POST /v1/user/registerSend`
- `POST /v1/user/loginSend`
- `POST /v1/user/register`
- `POST /v1/user/login`
- `GET /v1/user/info`

## 后续方向

- 补齐 MySQL 连接与 migration 方案
- 继续完成用户模块剩余接口
- 接入 refresh token、修改资料、验证码校验等能力
