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

## 当前记忆（2026-04-03）

- 今天已完成头像链路从预签名 URL 方案切换为 STS 临时凭证方案。
- 今天已新增并启用 `GET /v1/cloud/getToken`，由后端调用 STS `AssumeRole` 下发临时访问凭证给 App。
- 当前头像对象 key 规则已固定为：`avatar/{uid}`。
- 当前 `/v1/user/info` 的 `avatar` 语义已改为乐观 object key，直接返回 `avatar/{uid}`，不再表示“后端确认已上传成功”。
- 当前头像上传与读取不再依赖后端生成预签名上传/下载 URL，也不再依赖 `avatarUploadPrepare` / `avatarUploadConfirm`。
- 当前 STS 临时凭证权限已收敛到当前用户自己的头像对象：`avatar/{uid}`，只允许 `oss:PutObject` 与 `oss:GetObject`。
- 今天已完成阿里云 RAM / STS 实际联调：`/v1/cloud/getToken` 可正常返回临时凭证，且已验证能对 `avatar/{uid}` 成功执行 `PutObject` 与 `GetObject`。
- 协作经验：用户在学习 STS、RAM、AssumeRole、对象存储签名链路时，倾向先搞清楚“权限分层”和“请求到底发给谁”；解释时应明确区分 RAM 用户、RAM 角色、AssumeRole 权限、角色基础权限、会话策略这几层。

## 当前待推进（2026-04-03）

- 家庭模块第一阶段已完成方案对齐，当前目标接口为：
  - `POST /v1/device/homeCreate`
  - `GET /v1/device/homes`
  - `GET /v1/device/homeUsers`
  - `POST /v1/device/homeUpdate`
  - `DELETE /v1/device/homeDelete`
- 家庭模块第一阶段按“单人家庭、仅拥有者可修改/删除、成员可查看”的规则设计。
- `homeUsers` / `homes` 这类查看接口应校验“当前用户是否属于该家庭”，不是校验“是否为拥有者”。
- `homeUpdate` / `homeDelete` 这类修改接口才需要校验“当前用户是否为拥有者”。
- 家庭成员头像语义与用户模块保持一致，统一返回 object key：`avatar/{uid}`，不是 URL。
- 家庭模块第一阶段落地后，下一阶段仍按既定优先级继续：先做设备模块，再考虑事件等模块。
- 数据库管理下一步要正式切换到 migration 流程：不再长期依赖 `cfg.AutoMigrate`，而是由程序启动时执行 `migrations/*.sql`，通过 `schema_migrations` 管理版本。
- 当前已确定数据库管理方向：MySQL-only + 仓内自研 migration runner。

## 当前记忆（2026-04-07）

- 今天已决定数据库管理正式切到 migration 流程，不再把 `AutoMigrate` 当长期方案使用。
- 当前数据库方案已收敛为 MySQL-only；SQLite 仅作为历史阶段存在，不再作为正式运行与测试基线。
- 本次切换允许直接重写 `001`，将其收敛为当前完整 schema 基线；切换完成后，后续 schema 变更一律通过新增 migration 文件推进。
- 程序启动流程目标已明确：连接 MySQL 后自动执行未执行的 `migrations/*.sql`，并通过 `schema_migrations` 记录版本。
- 用户当前希望尽早养成正式习惯，即使尚未上线也优先采用 migration，而不是继续长期依赖 GORM 自动建表。

## 当前记忆（2026-04-08）

- 今天已明确协议层要直接切新合同，不再兼容 `Authorization: Bearer`；登录态统一改为 Header `access_token`。
- 当前用户端已实现接口需要统一校验 Header：`appid`、`app_version`、`phone_code`、`timestamp`、`request_id`、`sign`。
- 用户端签名规则已固定为：`METHOD + "&" + app_version + appid + phone_code + request_id + timestamp + "&" + canonicalQueryOrBodyParams`，再用 `APP_SECRET_KEY` 做 HMAC-SHA256 并 Base64。
- 时间戳校验当前固定允许偏差 5 分钟；超出时返回 `1004`。
- 设备端 `POST /v1/device/bind`、`POST /v1/device/login` 也必须真实实现，且签名规则与用户端不同：`sign` 使用 `model_secret`，配置来源固定为 JSON 环境变量 `DEVICE_MODEL_SECRETS`。
- `docs/api/openapi.yaml` 已同步补充 Header 参数与设备端 `bind/login` 合同壳，但仅改 OpenAPI 不足以联调成功，代码侧必须同步协议层。

## 当前记忆（2026-04-08 更新）

- 今天已完成协议层真实改造：用户端已实现接口统一读取并校验 Header `appid`、`app_version`、`phone_code`、`timestamp`、`request_id`、`sign`，登录态统一从 Header `access_token` 读取。
- 当前服务端已不再使用 `Authorization: Bearer` 作为正式登录态入口；相关测试与联调方式都应改为 `access_token` Header。
- 用户端签名规则已根据旧版本后端实际行为回退兼容，当前真实规则为：`METHOD + "&" + access_token + app_version + appid + phone_code + request_id + timestamp + "&" + canonicalParams`，其中 `access_token` 仅在请求头实际携带时参与。
- 当前用户端签名结果不是“原始 HMAC 字节直接 Base64”，而是兼容旧版的：先做 `HMAC-SHA256`，转为小写 hex 字符串，再对该 hex 字符串做 Base64。
- 当前设备端签名规则与用户端不同，仍保持：`METHOD + "&" + model + request_id + timestamp + uuid + "&" + canonicalParams`，并直接对原始 `HMAC-SHA256` 结果做 Base64。
- 今天已真实实现设备端接口：
  - `POST /v1/device/bind`
  - `POST /v1/device/login`
- 当前设备端最小链路已可用：`bind` 会创建或刷新最小设备记录；`login` 会校验设备存在且归属该 `uid`。
- 当前设备模块第一批基础接口也已完成：
  - `GET /v1/device/homeDevices`
  - `GET /v1/device/list`
  - `GET /v1/device/newList`
  - `POST /v1/device/upName`
- 当前 `device/login` 的请求体已与主合同重新对齐，正确 body 为：
  - `zone`
  - `version`
  不再使用旧的 `a` 占位字段。
- 当前 OpenAPI 已同步显式建模用户端公共 Header，并已补入设备端 `bind/login`；Apifox 导入后，Header 的默认值展示不完全可靠，联调时应依赖环境变量和前置脚本，而不是指望界面自动回填。
- 当前 Apifox 联调经验已明确：
  - 前置脚本执行时，`pm.request.body.raw` 仍可能是 `{{变量}}` 模板，签名前必须先用 `pm.variables.replaceIn(...)` 展开真实值。
  - `pm` 是 Apifox/Postman 提供的脚本运行时上下文；`pm.environment` 用于读写环境变量；`pm.variables.replaceIn(...)` 用于把 `{{变量名}}` 模板替换成当前环境里的实际值。
- 当前数据库流程、协议层、设备端基础链路和测试均已跑通；最近相关提交包括：
  - `68f8224 feat: add home module and migrate to sql migrations`
  - `d8503ad feat: add home device list skeleton`
  - `ce69e5a feat: align protocol signing and device auth flow`
  - `93601e5 fix: align device login payload with contract`

## 当前待推进（2026-04-08 更新）

- 下一步优先实现 `POST /v1/device/homeAddDevice`，把当前已有的 `bind -> homeDevices/list/newList/upName` 串成完整设备挂家庭闭环。
- `homeAddDevice` 第一版规则已经定下：
  - 只有家庭 owner 可以添加设备
  - 设备必须属于当前登录用户本人
  - 设备已在当前家庭时按幂等成功处理
  - 设备已挂到其他家庭时返回业务失败
- 当前 `homeAddDevice` 暂不顺带实现 `homeChange`；本轮不做跨家庭自动迁移、设备分享、复杂权限扩展。
- `homeAddDevice` 当前预计不新增 migration，优先复用现有 `devices`、`home_devices` 表；去重和冲突先由 service/repository 显式处理。
- `homeAddDevice` 完成后，下一阶段再考虑：
  - `POST /v1/device/homeChange`
  - `GET /v1/device/models`
  - `GET /v1/device/upgradedVersion`
  或其他设备域真实能力扩展。
