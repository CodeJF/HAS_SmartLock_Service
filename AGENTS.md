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

## 当前统一记忆（2026-05-07）

- 当前仓库已完成用户、家庭、设备基础、家庭分享、设备分享、消息、事件等主 HTTP 链路，相关接口已进入稳定回归阶段。
- 当前协议层已正式切到新合同：
  - 用户端统一校验 `appid`、`app_version`、`phone_code`、`timestamp`、`request_id`、`sign`
  - 登录态统一从 Header `access_token` 读取
  - 不再以 `Authorization: Bearer` 作为正式协议入口
- 当前数据库方案已收敛为：
  - MySQL-only 业务主库
  - 程序启动自动执行仓内 SQL migrations
  - `AutoMigrate` 不再作为正式长期方案
- 当前头像/云存储链路已切到 STS 临时凭证方案：
  - `GET /v1/cloud/getToken` 已可用
  - 头像 object key 语义固定为 `avatar/{uid}`
- 当前设备与家庭主链路已经闭环：
  - `POST /v1/device/bind`
  - `POST /v1/device/login`
  - `POST /v1/device/homeAddDevice`
  - `POST /v1/device/homeChange`
  - `GET /v1/device/homeDevices`
  - `GET /v1/device/list`
  - `GET /v1/device/newList`
  - `POST /v1/device/upName`
- 当前家庭分享链路已完成：
  - `POST /v1/device/homeShare`
  - `POST /v1/device/homeShareFeedback`
  - `POST /v1/device/homeShareRemove`
- 当前设备分享链路已完成：
  - `POST /v1/device/share`
  - `GET /v1/device/shareRecords`
  - `POST /v1/device/shareDelete`
  - `POST /v1/device/shareFeedback`
- 当前消息模块已完成并覆盖家庭分享与设备分享消息：
  - `GET /v1/message/list`
  - `GET /v1/message/unreadNum`
  - `POST /v1/message/read`
  - `DELETE /v1/message/delete`
- 当前事件模块已完成并已切到 realtime event store 语义：
  - `GET /v1/event/list`
  - `GET /v1/event/existDay`
  - `GET /v1/event/unreadNum`
  - `POST /v1/event/read`
  - `DELETE /v1/event/delete`
- 当前设备型号与升级接口已完成：
  - `GET /v1/device/models`
  - `GET /v1/device/upgradedVersion`

## 当前实时链路记忆（2026-05-07）

- 基于 `docs/MQTT_WebSocket_对接技术文档.md`，仓库已经完成 MQTT + WebSocket 实时主干接入：
  - Redis、MongoDB、MQTT client、WebSocket Hub、`/ws`、`/getUrl`
  - shadow store
  - Mongo event store
- 当前启动层要求 Mongo 可用；测试环境支持 `MongoURI=memory://local` 的内存版 shadow/event store，用于维持 Mongo-only 语义下的测试基线。
- MQTT 订阅主链路已接入：
  - `attr/set/resp`
  - `attr/post`
  - `attr/get`
  - `event/+`
  - `func/+/resp`
  - `$SYS connected`
  - `$SYS disconnected`
- 当前 WebSocket method 已接入主集合：
  - `SetAttribute`、`GetAttribute`、`SetNotifyStatus`、`GetNotifyStatus`
  - `SleepState`、`Awake`
  - `CreateUser`、`UpdateUser`、`DelUser`
  - `AddPwd`、`UpdatePwd`、`DelPwd`
  - `Lock`、`Unlock`
  - `SetBluetoothSwitch`、`DelBluetoothSwitch`
  - `OtaUpgrade`、`Reboot`、`ResetSetting`
  - `SetBackgroundImage`、`Call`、`ChangeCam`、`OthersAnswer`
- 延迟响应类 method 已按协议实现“先发 MQTT，等设备 `func/<x>/resp`，再推 `ServerFunc.<x>`”，其中 `SetBackgroundImage`、`ChangeCam` 按 `phone_code` 单端回推。
- 当前服务端主动推送已实现：
  - `DeviceBind`
  - `DeviceUnBind`
  - `DevicesChanged`
  - `AttributeChange`
  - `Ring`
  - `RingStop`
  - `ServerFunc.<funcName>`
  - `ServerFunc.SetBackgroundImage`
  - `ServerFunc.ChangeCam`
  - `ServerFunc.WebrtcSignal`
- 当前设备绑定/解绑已接入实时语义：
  - `bind` 会写 `mqtt_user:<uuid>`、`mqtt_acl:<uuid>`、`device_bind:<uuid>` 并初始化 shadow
  - `remove` 会先发 `func/UnBind`，再删 Redis 凭证/ACL/绑定缓存，并删除 shadow
- 当前 `go test ./...` 为通过状态，说明现有 HTTP 主链路、设备链路、事件链路和实时基础装配测试基线是通的。

## 当前明确未完成/仅为桩的部分

- 当前仍是“协议桩/受控桩”的 WebSocket method 有 4 个：
  - `LeaveWord`
  - `WebrtcSignal`
  - `SetP2pState`
  - `QuickReplyFile`
- `WebrtcSignal` 当前虽然已按文档要求做成异步 channel，不会阻塞主线程，但还没有接真实 WebRTC / turn / p2p 服务。
- `LeaveWord` 当前还没有落真实存储和 HTTP 留言模块，`/v1/leaveword/*` 仍未真正实现。
- 合同中 `feedback`、`leaveword`、`afterSales` 等外围业务模块仍未完整进入实现阶段。

## 当前待推进

- 下一阶段若继续沿实时文档推进，优先级应理解为：
  1. 先把 `LeaveWord` 从协议桩推进到真实入库 + APP 侧可查询/已读闭环
  2. 再决定是否推进真实 `WebrtcSignal / SetP2pState`
  3. 最后再进入 `feedback`、`afterSales` 等尚未开工的外围业务模块
