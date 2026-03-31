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
go run ./cmd/api
```

默认监听地址：

- `http://localhost:8080`

可用环境变量：

- `APP_NAME`
- `APP_ENV`
- `HTTP_ADDR`
- `DB_DRIVER`
- `DB_DSN`
- `MYSQL_DSN`
- `AUTO_MIGRATE`
- `JWT_SECRET`

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
