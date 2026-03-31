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
