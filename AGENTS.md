# AGENTS

本仓库用于 HAS SmartLock API 服务重构与 Go 后端学习实践。

## 仓库目标

- 以 Go 重构 App 当前实际使用的 API 服务。
- 按“先表设计，再服务实现，再接口联调”的方式逐步推进。
- 优先保证接口合同与 App 现有调用保持一致，再考虑内部实现优化。

## 主合同文档

- `docs/api/http_api_index.md` 是本仓库 API 服务重构的主合同文档。
- `docs/api/openapi.yaml` 是本仓库面向 Apifox、Postman 等 API 平台的导入源文档。
- 所有 API 设计、实现、测试、联调、回归，默认都以 `docs/api/http_api_index.md` 为准。
- 对外导入到 Apifox、Postman、其他接口测试平台时，优先使用 `docs/api/openapi.yaml`，避免在平台内手工逐个维护接口。
- 若其他文档、旧 Postman collection、历史实现与 `docs/api/http_api_index.md` 冲突，默认以 `docs/api/http_api_index.md` 为准。
- 若 `docs/api/http_api_index.md` 中字段、参数或返回值仍为 `TODO`，允许在实施时回查 App 实际请求、旧文档或其他历史资料补充细节。
- 回查得到的新事实应优先回写到 `docs/api/http_api_index.md`，保持该文档持续成为单一真值来源。

## 协作与输出规则

- 本仓库内所有 agent 的用户可见回答统一使用简体中文。
- 本项目以“新手学习指导导向”推进；回答和实现说明应优先帮助用户理解 Go 后端、数据库、鉴权、测试和接口联调流程。
- 默认采用教学式协作：先解释当前修改解决了什么，再解释代码位于哪里、请求如何流转、后续应该如何验证。
- 面向用户的技术解释要优先保证准确性，再追求通俗；先给精确结论和适用前提，避免用过度简化但可能误导的说法。
- 当问题涉及框架行为、执行时机、作用域或语言细节时，应明确说明“什么时候生效、影响哪些对象、不影响哪些对象”。
- 变更应保持聚焦，避免一次性引入不必要的重构。
- 未经明确要求，不要回退用户已有修改。
- 任何影响接口行为、字段定义、鉴权规则或响应结构的变更，必须同步更新主合同文档。

## 实施原则

- 先对齐 API 合同，再扩展实现细节。
- Handler 层负责协议转换与参数校验，业务逻辑放在 service 层，数据访问放在 repository 层。
- 优先做小步可验证的改动，并为核心行为补充测试。
- 新增模块或文档时，应优先放在清晰、稳定的目录结构中，例如 `docs/api/`、`cmd/`、`internal/`、`migrations/`。

## 文档维护

- 接口新增、删除或行为变化后，首先更新 `docs/api/http_api_index.md`。
- 当前已实现且需要导入测试平台的接口，应同步更新 `docs/api/openapi.yaml`。
- 若新增补充说明文档，其内容必须与 `docs/api/http_api_index.md` 保持一致，不得形成冲突的第二份接口真值。
- README 应记录关键的运行、调试和联调方式。

## 当前记忆（2026-03-31）

- 今天已完成仓库初始化、Go API 服务骨架、配置与统一响应封装、SQLite/MySQL 双模式数据库接入、用户模块基础表设计与 migration 草稿。
- 今天已完成用户认证主链路：`/v1/user/registerSend`、`/v1/user/loginSend`、`/v1/user/register`、`/v1/user/login`、`/v1/user/info`。
- 今天已完成用户登录态闭环：`/v1/user/validateCode`、`/v1/user/refresh`、`/v1/user/logout`。
- 当前 OpenAPI 导入源 `docs/api/openapi.yaml` 已覆盖已实现接口，可直接用于 Apifox / Postman 导入测试。
- 当前验证码默认生成固定值 `123456`，这是开发学习阶段的临时实现，后续应替换为真实随机码与发送能力。
- 当前 `logout` 会撤销该用户全部活跃 `refresh_token`；`access_token` 不做黑名单，注销后会继续有效直到自然过期。
- 协作经验：用户希望回答先准确、再解释；尤其在 Gin、GORM、Go 语法、执行时机这类问题上，要避免“方便理解但不精确”的表述。

## 当前记忆（2026-04-01）

- 今天已完成用户模块剩余核心接口：`/v1/user/resetSend`、`/v1/user/reset`、`/v1/user/updatePwd`、`/v1/user/updateInfo`、`/v1/user/putClient`、`/v1/user/deleteSend`、`/v1/user/delete`。
- 今天已完成删除账号链路，第一版采用逻辑删除；删除后用户无法再登录，活跃 `refresh_token` 会被撤销。
- 今天已确认 `GET /v1/user/putAvatar` 不再作为独立接口实现；头像地址统一从 `/v1/user/info` 的 `avatar` 字段获取。
- 截至目前，用户模块主合同中可实现的核心接口已基本完成，OpenAPI 与测试均已同步。

## 后续待推进

- 用户模块合同中已不再保留 `/v1/user/putAvatar`；头像地址由 `/v1/user/info` 中的 `avatar` 字段提供，不再作为独立接口实现。
- 在用户模块稳定后，再推进数据库管理规范：明确 `AutoMigrate` 与正式 migration 工具的最终方案，避免长期双轨。
- 后续进入设备、家庭、事件等模块时，继续保持“先补合同文档/OpenAPI，再实现，再补测试”的顺序。
- 下一阶段优先级：先做家庭模块，再做设备模块。
