# MQTT WebSocket 后端服务对接技术实现文档（单服务版）

> 本文档基于 SL100 平台 `deviceShadow` / `gateway` / `AdminService` 三个 Go 服务的真实实现整理，**已合并为单服务架构**，便于在一个 Go / Java / Node 单体项目中直接复刻。
> 所有 topic、字段名、状态值、表结构均来自源码，未做编造。

---

## 0. 单服务 vs 原多服务的差异

| 项 | 原多服务实现 | 单服务复刻建议 |
|---|---|---|
| HTTP API | `gateway` 入口 + `AdminService` 业务 | 同进程 HTTP 路由（Gin / Spring / Express 任一） |
| MQTT 客户端 | `deviceShadow` 持有 super-user 连接，订阅设备消息 | 同进程内启动一个 MQTT client goroutine / worker，挂全局单例 |
| 设备绑定 / 凭证生成 | `gateway` 调 `deviceShadow` 的 RPC 写 Redis | 同进程函数调用，直接写 Redis |
| WebSocket | `deviceShadow:801/ws` | 同进程 `/ws` endpoint，复用 HTTP server |
| 服务间 RPC | gRPC / 自研 RPC | **删除**，全部改为内部函数调用 |
| MQTT 服务账号 | 每个服务一个 super-user | **只需 1 个**，例 `backend_service` |
| Push 队列消费者 | 独立 `pushService` 进程消费 Redis list | 单服务可拆 goroutine 内消费，或保留独立 worker（推荐） |
| 云存储签名 URL | 独立 `cloudStorage` 服务 RPC（`rpc.GetSignUrl`） | 内部函数：直接调 OSS/S3 SDK 生成签名 URL |
| 视频信令 / Turn 服务 | 独立 `turnService` RPC（`rpc.NotifySDP/AppStartPush/...`） | 不做对讲可整段删除；做就接 coturn / mediasoup |

**保持不变**：MQTT broker、topic 命名、消息 JSON 结构、ACL 规则、设备凭证规则、数据库 schema、WebSocket 协议。这些是协议层的事，跟服务拆分无关。

### 0.1 单服务复刻 6 个最容易踩的坑（先看这个）

1. **WebSocket method 命中时不要立即 ack**——`CreateUser` / `AddPwd` / `SetBackgroundImage` / `ChangeCam` 等命令必须等设备的 `func/<x>/resp` 才能 `ServerFunc.<x>` 推回前端。前端的 UI 进度态依赖这条逻辑。**详见 §A.4 "立即响应 vs 等设备 resp" 列**。
2. **`SetBackgroundImage` / `ChangeCam` 必须按 phone_code 单端推送**——不要广播 uid，否则同一账号的另一台手机会收到莫名其妙的"切换镜头"通知。Cache 里要存 `phone_code`，应答用 `NotifyUserFunctionResponseByPhoneCode()`。
3. **`WebrtcSignal` 一定要异步 channel 处理**——同步阻塞 MQTT 回调会拖慢全部设备消息。详见 §5.7。
4. **`RingStop` 不入库、`SetUsers/DelUser/AddPwd/DelPwd`（事件方向）不入库，只更新影子**——事件入库矩阵见 §3.4 第二张表。
5. **解绑时 Redis 三个 key 必须一起删**：`mqtt_user:<u>`、`mqtt_acl:<u>`、`device_bind:<u>`，并向设备 publish `func/UnBind {clean_data: 0/1}`，否则设备还能连进来或保留旧数据。
6. **`$SYS/.../disconnected` 收到 reason=`tcp_closed`/`discarded` 时不要推前端**——这是 EMQX 闪断瞬时上报，紧接着会有 `connected` 重连，推了前端会闪烁。

---

## 1. 系统架构（单服务版）

### 1.1 组件关系

```
                              ┌──────────────────────────┐
                              │  Frontend (App / Web)    │
                              └────────────┬─────────────┘
                                           │  WebSocket (ws / wss)
                                           │  ws://<host>:<ws_port>/ws
                                           │  Header: access_token, phone_code
                                           ▼
   ┌─────────────┐    MQTT TCP 1883    ┌──────────────────────────────────┐
   │  IoT Device │ ◄──────────────►   │      Backend (单服务)            │
   │  (Lock)     │                     │  ┌────────────────────────────┐  │
   └─────────────┘                     │  │ HTTP API (Gin / Spring)    │  │
        ▲                              │  │  - 用户登录、设备绑定/解绑  │  │
        │ MQTT TCP 1883                │  │  - 设备列表、影子查询       │  │
        │ user=<uuid>                  │  ├────────────────────────────┤  │
        │ pwd=<rand16>                 │  │ MQTT Client (Paho/eclipse) │  │
        ▼                              │  │  - super-user 接入 broker   │  │
   ┌────────────────────────────┐      │  │  - 订阅设备 topic + $SYS    │  │
   │  MQTT Broker (EMQX 4.x/5)  │      │  │  - 反射分发到 handler        │  │
   │  - TCP :1883               │ ◄──► │  ├────────────────────────────┤  │
   │  - $SYS/.../{connected,    │      │  │ WebSocket Hub               │  │
   │     disconnected}          │      │  │  - JWT 鉴权                 │  │
   │  - 鉴权/ACL → Redis        │      │  │  - uid+phone_code 注册连接   │  │
   └────────────────────────────┘      │  │  - 双向桥接 MQTT ↔ 前端     │  │
                                       │  └────────────────────────────┘  │
                                       │     │           │           │   │
                                       └─────┼───────────┼───────────┼───┘
                                             ▼           ▼           ▼
                                          MySQL      MongoDB       Redis
```

### 1.2 模块职责（同进程，逻辑切分）

| 模块 | 职责 |
|------|------|
| **HTTP API 层** | 用户/设备 CRUD、绑定流程（生成 MQTT 凭证写 Redis）、影子/事件查询 |
| **MQTT 客户端模块** | 进程启动时连 broker，订阅 5+2 条 topic，按 topic 反射分发到业务 handler |
| **WebSocket Hub** | 维护 `<uid, phone_code> → conn` map，提供 `Notify*` 推送 API；接收前端 method 反射到 publish 函数 |
| **业务 Handler** | 属性上报、事件落库、func 应答处理（与 Redis 命令缓存配合做幂等） |
| **数据访问** | MySQL（device、user_device）、MongoDB（device_shadow、device_event）、Redis |

### 1.3 Broker 类型与版本

- **类型**：EMQX（依赖 `$SYS/brokers/+/clients/+/{connected,disconnected}` 系统主题，payload 字段如 `clean_start`、`proto_ver`、`sockport` 与 EMQX 完全一致）
- **MQTT 协议版本**：3.1.1（设备 `proto_ver=4`）
- **后端 MQTT 库**：`github.com/eclipse/paho.mqtt.golang`（Java 选 `org.eclipse.paho.client.mqttv3`，Node 选 `mqtt`）
- **建议版本**：EMQX 5.x，启用 Redis 鉴权与 ACL 模块

---

## 2. 连接与鉴权

### 2.1 连接地址

| 角色 | 协议 | 地址（占位） | 备注 |
|------|------|-------------|------|
| 设备 | `tcp://` | `tcp://<broker-host>:1883` | 生产建议 `mqtts://:8883` |
| 后端 MQTT client | `tcp://` | `tcp://<broker-host>:1883` | 同进程一个全局 client |
| 前端 WebSocket | `ws://` / `wss://` | `ws://<backend-host>:<ws_port>/ws` | **不直连 MQTT** |

> `<ws_port>` 在原项目用 `801`，单服务可与 HTTP API 共用同一端口（推荐），用 `/ws` path 区分。

### 2.2 凭证规则

#### (1) 设备凭证（绑定接口下发）

| 字段 | 规则 |
|------|------|
| `clientId` | 全局唯一即可。推荐 `<model>_<uuid>` 或仿设备的 `uuid_v1` 去 `-` |
| `username` | 设备 UUID（32 hex） |
| `password` | 16 字符随机串（绑定时一次性生成，明文返回设备一次） |
| Redis 写入 | `HMSET mqtt_user:<uuid> password_hash <sha256(password+salt)> salt <8字符随机>` |

代码片段（参考实现）：
```go
salt := RandString(8)
passwordHash := sha256Hex(password + salt)
redis.HMSet("mqtt_user:"+uuid, map[string]string{
    "password_hash": passwordHash,
    "salt":          salt,
})
```

#### (2) 后端 super-user 账号

进程启动前在 Redis 预置一行（部署脚本完成）：
```
HMSET mqtt_user:backend_service password_hash <sha256(...)> salt <...> is_superuser 1
```
配置文件中保存明文 username/password，启动时 MQTT client 用它连接。**示例值 `123456` 必须替换**。

#### (3) 前端 WebSocket 鉴权

```
GET /ws HTTP/1.1
Host: <backend-host>:<ws_port>
access_token: <JWT>
phone_code:   <端标识，区分多端登录>
Upgrade: websocket
```

服务端校验 JWT，把 `<uid, phone_code>` 作为连接的全局唯一键，登录新连接时**主动关闭旧连接**（`GetUserPhoneConn(uid, pco).Close()`）。

### 2.3 Client ID

| 角色 | 规则 |
|------|------|
| 后端连 broker | UUID v4 字符串 |
| 设备 | 全局唯一；不同实现选用 UUID v1（去 `-`）或 `<model>_<uuid>` |

### 2.4 KeepAlive / CleanSession / QoS / retain

| 参数 | 值 |
|------|----|
| KeepAlive | **60s** |
| PingTimeout | 2s |
| CleanSession | **false**（设备与后端均建议 false） |
| AutoReconnect | true（MaxReconnectInterval=10s，ConnectRetryInterval=2s） |
| ResumeSubs | true |
| **QoS（所有 publish）** | **0** |
| **retain** | **false** |
| Will（LWT） | **未配置**，离线由 `$SYS/.../disconnected` 兜底 |

> 建议：升级时 `attr/post` 与 `func/<x>` 改 QoS=1 + 持久会话，能扛后端短时重启不丢。

### 2.5 ACL 规则

EMQX Redis ACL：`HSET mqtt_acl:<uuid> <topic> {publish|subscribe}`。绑定时一次性写入：

| Topic | 设备权限 |
|-------|---------|
| `/thing/<model>/<uuid>/attr/set` | subscribe |
| `/thing/<model>/<uuid>/attr/set/resp` | publish |
| `/thing/<model>/<uuid>/attr/get` | publish |
| `/thing/<model>/<uuid>/attr/get/resp` | subscribe |
| `/thing/<model>/<uuid>/attr/post` | publish |
| `/thing/<model>/<uuid>/attr/post/resp` | subscribe |
| `/thing/<model>/<uuid>/event/+` | publish |
| `/thing/<model>/<uuid>/event/+/resp` | subscribe |
| `/thing/<model>/<uuid>/func/+` | subscribe |
| `/thing/<model>/<uuid>/func/+/resp` | publish |

设备只能操作自己 UUID 命名空间，无法跨设备订阅。后端 super-user 不受 ACL 限制。

---

## 3. Topic 设计

### 3.1 命名规则

```
/thing/<product_model>/<device_uuid>/<resource>/<action>[/<sub>]
```

- 以 **`/` 开头**（不是 `thing/...`）
- `<resource>` ∈ `attr` / `event` / `func`
- 通配符：仅服务端订阅时用 `+`；**业务不使用 `#`**
- 系统主题：`$SYS/brokers/+/clients/+/connected`、`$SYS/brokers/+/clients/+/disconnected`

### 3.2 全量 Topic 列表

> D=Device，B=Backend，F=Frontend(经 WS)。`<m>` = product_model，`<u>` = uuid。

| # | Topic | 方向 | 用途 | QoS | retain |
|---|-------|------|------|-----|--------|
| 1 | `/thing/<m>/<u>/attr/set` | B→D | 云端属性下发 | 0 | false |
| 2 | `/thing/<m>/<u>/attr/set/resp` | D→B | 设备 ACK | 0 | false |
| 3 | `/thing/<m>/<u>/attr/get` | D→B | 设备主动拉 desired | 0 | false |
| 4 | `/thing/<m>/<u>/attr/get/resp` | B→D | 后端回 desired | 0 | false |
| 5 | `/thing/<m>/<u>/attr/post` | D→B | 属性上报 | 0 | false |
| 6 | `/thing/<m>/<u>/attr/post/resp` | B→D | 上报 ACK | 0 | false |
| 7 | `/thing/<m>/<u>/event/<eventName>` | D→B | 事件上报 | 0 | false |
| 8 | `/thing/<m>/<u>/event/<eventName>/resp` | B→D | 事件 ACK | 0 | false |
| 9 | `/thing/<m>/<u>/func/<funcName>` | B→D | 功能下发 | 0 | false |
| 10 | `/thing/<m>/<u>/func/<funcName>/resp` | D→B | 功能应答 | 0 | false |
| 11 | `$SYS/brokers/+/clients/+/connected` | Broker→B | 客户端上线事件 | 0 | – |
| 12 | `$SYS/brokers/+/clients/+/disconnected` | Broker→B | 客户端下线事件 | 0 | – |

### 3.3 后端订阅模式（带通配，启动时一次性订阅）

```
/thing/+/+/attr/set/resp
/thing/+/+/attr/post
/thing/+/+/attr/get
/thing/+/+/event/+
/thing/+/+/func/+/resp
$SYS/brokers/+/clients/+/connected
$SYS/brokers/+/clients/+/disconnected
```

### 3.4 funcName / eventName 完整枚举

> 下面两张表来自代码 `subscribe/event/core.go: GetEventType()` 与 `socket/websocket/*.go` 反射的 `Request.<X>()` 方法的完整逐行盘点，单服务复刻时一个都不能少。

#### funcName（B→D 控制命令，topic = `/thing/<m>/<u>/func/<funcName>`）

| funcName | 触发来源（前端 Method） | 后端是否要写 Redis 缓存 | 备注 |
|----------|-----------------------|----------------------|------|
| `CreateUser` | WS `CreateUser` | 是（`device_user_create:<msg_id>`） | 仅 master 可调 |
| `UpdateUser` | WS `UpdateUser` | 是（`device_user_update:<msg_id>`） | 仅 master |
| `DelUser` | WS `DelUser` | 是（`device_user_del:<msg_id>`） | 仅 master |
| `AddPwd` | WS `AddPwd` | 是（`device_user_pwd_create:<msg_id>`） | 仅 master |
| `UpdatePwd` | WS `UpdatePwd` | 是（`device_user_pwd_update:<msg_id>`） | 仅 master |
| `DelPwd` | WS `DelPwd` | 是（`device_user_pwd_del:<msg_id>`） | 仅 master |
| `Lock` | WS `Lock` | 否 | data 自动注入 `{uid, username}` |
| `Unlock` | WS `Unlock` | 否 | 同上 |
| `SetBluetoothSwitch` | WS `SetBluetoothSwitch` | 否 | 仅 master |
| `DelBluetoothSwitch` | WS `DelBluetoothSwitch` | 否 | 仅 master |
| `OtaUpgrade` | WS `OtaUpgrade` | 否 | 仅 master，需查固件版本表 + 云存储签名 URL |
| `Reboot` | WS `Reboot` | 否 | 仅 master |
| `ResetSetting` | WS `ResetSetting` | 否 | 仅 master |
| `SetBackgroundImage` | WS `SetBackgroundImage` | 是（`device_background_image:<msg_id>`） | **关联 phone_code** |
| `Call` | WS `Call` | 否 | 仅 master，门铃远程呼叫 |
| `ChangeCam` | WS `ChangeCam` | 是（`device_change_cam:<msg_id>`） | **关联 phone_code** |
| `WebrtcSignal` | WS `WebrtcSignal` | 否 | 走 channel 异步，下文详述 |
| `Awake` | WS `Awake` | 否，但写 `MarkWakeUpStatus(uuid)` | 用 Awake 简化结构（无 time 字段） |
| `UnBind` | 解绑接口触发 | 否 | data: `{clean_data: 0/1}` |

#### eventName（D→B 上报，topic = `/thing/<m>/<u>/event/<eventName>`）

| eventName | 是否入 `device_event` | 是否进 push 队列 | 处理特征 |
|-----------|---------------------|---------------|---------|
| `OpenDoor` | ✅ | ✅ QueueDeviceEvent | data 含 user_id/user_name/type |
| `Tamper` | ✅ | ✅ QueueDeviceEvent | |
| `Ring` | ✅ | ✅ QueueDeviceEvent | data 含 flag |
| `RingStop` | **❌ 不入库** | ✅ QueueDeviceEvent | 仅停铃信号 |
| `Pir` | ✅ | ✅ QueueDeviceEvent | 红外感应 |
| `Radar` | ✅ | ✅ QueueDeviceEvent | 雷达感应 |
| `Stay` | ✅ | ✅ QueueDeviceEvent | 逗留 |
| `OpenDoorFailed` | ✅ | ✅ QueueDeviceEvent | |
| `OpenDoorFailedMax` | ✅ | ✅ QueueDeviceEvent | |
| `WaitingLockTimeout` | ✅ | ✅ QueueDeviceEvent | |
| `LowBattery` | ✅ | ✅ QueueDeviceEvent | 无 thumbnail |
| `Conflagration` | ✅ | ✅ QueueDeviceEvent | 火灾报警 |
| `LeaveWord` | 落 LeaveWord 集合 | ✅ QueueDeviceEvent | 留言 |
| `UpgradeStart` | ✅ | ✅ **QueueDeviceUpgrade** | 走升级专用队列 |
| `UpgradeResult` | ✅ | ✅ **QueueDeviceUpgrade** | result=1 成功 / 0 失败 |
| `SetUsers` | **❌ 直接写影子** | ❌ | 调 `BatchSetUser()` |
| `DelUser`（事件方向） | **❌ 直接写影子** | ❌ | 调 `BatchDelUser()`，注意与 func 同名 |
| `AddPwd`（事件方向） | **❌ 直接写影子** | ❌ | 调 `BatchAddPwd()` |
| `DelPwd`（事件方向） | **❌ 直接写影子** | ❌ | 调 `BatchDelPwd()` |
| `WebrtcSignal` | **❌ 走异步 channel** | ❌ | `PushWebrtcSignalChannel()`，下文详述 |
| `ChangeCamResp` | **❌ 不入库** | ❌ | 直接 `NotifyUserFunctionResponse()` 推前端 |
| `ReportAbnormal` | **❌ 空实现** | ❌ | no-op，仅返 OK |
| `GetWeather` | ❌ | ❌ | 设备拉天气，后端调外部 API 后回 resp |
| `LeaveWordUnRead` | ❌ | ❌ | 返未读留言数 |
| `GetLastLeaveWordIsRead` | ❌ | ❌ | 返最后一条留言的已读状态 |

> ⚠️ 注意：**`SetUsers` / `DelUser` / `AddPwd` / `DelPwd` 既是 event 也是 func**——event 方向是设备主动通知后端"我侧用户/密码状态变了，更新影子"；func 方向是 APP 主动下发"创建/删除"。两条链路独立，单服务复刻必须都实现。

---

## 4. 消息协议

所有 MQTT payload 均为 UTF-8 JSON。

### 4.1 通用字段规则

| 字段 | 类型 | 规则 |
|------|------|------|
| `msg_id` | string | UUID（v1/v4 均可），由发起方生成；ACK 必须原样回填；同时是 Redis 命令缓存 Key 的一部分 |
| `time` | int64 | Unix **秒**（`attr/post` 顶层 `time` 通常为毫秒，每个属性的 `timestamp` 也是毫秒；其余统一秒。**实现时务必保持发送方与接收方一致**） |
| `result` | int64 | `1`=成功，`0`=失败 |
| `code` | int64 | 业务错误码，0 / 200 = OK |
| `msg` | string | 错误描述；成功为 `"ok"` |
| `traceId` | – | **未使用**。链路追踪靠 `msg_id` |

### 4.2 三个骨架结构

```go
// 4.2.1 Request（B→D）
type Request struct {
    MsgId string      `json:"msg_id"`
    Time  int64       `json:"time"`
    Data  interface{} `json:"data"`
}

// 4.2.2 Response（attr ACK / 通用回执）
type Response struct {
    MsgId  string                 `json:"msg_id"`
    Time   int64                  `json:"time"`
    Result int64                  `json:"result"`
    Msg    string                 `json:"msg"`
    Data   map[string]interface{} `json:"data"`
}

// 4.2.3 ResponseEvent（事件上报 ACK，比 Response 多一个 code）
type ResponseEvent struct {
    MsgId  string                 `json:"msg_id"`
    Time   int64                  `json:"time"`
    Result int64                  `json:"result"`
    Msg    string                 `json:"msg"`
    Code   int64                  `json:"code"`
    Data   map[string]interface{} `json:"data"`
}
```

### 4.3 Func 设备应答（特殊：含 err_code、status、加密载荷）

```go
type FuncResponse struct {
    MsgId   string `json:"msg_id"`
    Time    int    `json:"time"`
    Result  int    `json:"result"`     // 1=成功 0=失败
    ErrCode int64  `json:"err_code"`   // 设备端业务错误码
    Status  int    `json:"status"`
    Data    struct {
        Id   string `json:"id"`        // 例：新建用户 / 密码 ID
        Enc  string `json:"enc"`
        Data string `json:"data"`
        Salt string `json:"salt"`
    } `json:"data"`
}
```

### 4.4 各类消息示例

#### (1) 属性下发 `/thing/SL100/<uuid>/attr/set`（B→D）
```json
{
  "msg_id": "8b1d2c3e-4f50-11ee-8c99-0242ac120002",
  "time": 1714435200,
  "data": { "SleepState": 1, "Volume": 80 }
}
```

#### (2) 设备 ACK `/thing/SL100/<uuid>/attr/set/resp`（D→B）
```json
{ "msg_id": "8b1d2c3e-4f50-11ee-8c99-0242ac120002", "time": 1714435201, "result": 1 }
```

#### (3) 属性上报 `/thing/SL100/<uuid>/attr/post`（D→B）

每个属性带独立时间戳（**毫秒**），后端按 `metadata.reported.<attr>.timestamp` 与入参 `value.timestamp` 比较，旧时间戳直接丢弃（天然抗乱序与重复）。

```json
{
  "msg_id": "f3a4b5c6-d7e8-49f0-a1b2-c3d4e5f60718",
  "time": 1714435210000,
  "data": {
    "Online":  { "timestamp": 1714435210000, "value": 1  },
    "Battery": { "timestamp": 1714435210000, "value": 85 }
  }
}
```

#### (4) 上报 ACK `/thing/SL100/<uuid>/attr/post/resp`（B→D）
```json
{ "msg_id": "f3a4b5c6-d7e8-49f0-a1b2-c3d4e5f60718", "time": 1714435211, "result": 1, "msg": "ok", "data": null }
```

#### (5) 事件上报 `/thing/SL100/<uuid>/event/OpenDoor`（D→B）
```json
{
  "msg_id": "01HC0K8...",
  "data": {
    "time": 1714435300,
    "thumbnail": "oss://hichs/snap/2025-04-30/xxx.jpg",
    "user_id": "u_001",
    "user_name": "Alice",
    "type": 1
  }
}
```

> `time / user_id / user_name / type` 缺一即返 `RequestParameterError`。`type` 取值（开门方式）：`1`=指纹、`2`=密码、`3`=刷卡、`4`=人脸、`5`=钥匙、`6`=APP远程，需结合产品自定义。

#### (6) 事件 ACK `/thing/SL100/<uuid>/event/OpenDoor/resp`（B→D）
```json
{ "msg_id": "01HC0K8...", "time": 1714435301, "result": 1, "code": 200, "msg": "ok", "data": null }
```

#### (7) 功能下发 `/thing/SL100/<uuid>/func/CreateUser`（B→D）
```json
{
  "msg_id": "create-user-7c1f...",
  "time": 1714435400,
  "data": { "name": "Alice", "role": 1 }
}
```

#### (8) 功能应答 `/thing/SL100/<uuid>/func/CreateUser/resp`（D→B）
```json
{
  "msg_id": "create-user-7c1f...",
  "time": 1714435401,
  "result": 1,
  "err_code": 0,
  "status": 0,
  "data": { "id": "user_001", "enc": "", "data": "", "salt": "" }
}
```

#### (9) EMQX 上线事件 `$SYS/brokers/<broker_id>/clients/<clientid>/connected`
```json
{
  "clean_start": false,
  "username": "239c926067b841a28a0912283e8a0b98",
  "ts": 1714435000123,
  "sockport": 1883,
  "reason": "",
  "protocol": "mqtt",
  "proto_ver": 4,
  "proto_name": "MQTT",
  "ipaddress": "203.0.113.5",
  "disconnected_at": 0,
  "connected_at": 1714435000120,
  "clientid": "239c926067b841a28a0912283e8a0b98"
}
```

#### (10) EMQX 下线事件 `$SYS/.../disconnected`
```json
{
  "username": "239c926067b841a28a0912283e8a0b98",
  "reason": "tcp_closed",
  "disconnected_at": 1714436000000,
  "clientid": "239c926067b841a28a0912283e8a0b98"
}
```

> `reason ∈ {"discarded","tcp_closed"}` 时**不通知前端**（瞬时断连，立即重连）。

### 4.5 ack / 重试 / 幂等 / 去重

| 维度 | 规则 |
|------|------|
| **ack** | 设备每次 `attr/post`、`event/<x>` 后必须收到对应 `*/resp`，但应用层不强等待；QoS=0 不保证 broker 重发 |
| **重试** | 后端 `func` 下发**无内置重试**，由前端层再次发起。MQTT 库自带 `AutoReconnect=true`、`ResumeSubs=true` |
| **幂等** | 关键操作以 `msg_id` 作幂等键。下发前把请求体写入 Redis Key `device_user_create:<msg_id>` 等（TTL 1h），收到 resp 时凭 `msg_id` 取出原始上下文 |
| **去重** | 属性级用 `MetaData.Reported[attr].Timestamp` 与入参 `value.timestamp` 比较，旧/等于丢弃 |

---

## 5. 业务流程

### 5.1 设备上线 / 离线（基于 `$SYS`）

```
Device          EMQX                 Backend                  MySQL/Mongo/Redis    Frontend
  │   CONNECT    │                      │                          │                  │
  │ user=<uuid>  │                      │                          │                  │
  │ pwd=<rand16> │                      │                          │                  │
  ├─────────────►│ AUTH 查 Redis         │                          │                  │
  │              │ mqtt_user:<uuid>     │                          │                  │
  │   CONNACK    │                      │                          │                  │
  │◄─────────────┤                      │                          │                  │
  │              │ pub $SYS/.../connected ───────────────────────► │                  │
  │              │                      │ MqttUserIsSuper(false)?  │                  │
  │              │                      │ GetDeviceMaster(uuid)    │                  │
  │              │                      │ MySQL UPDATE device      │                  │
  │              │                      │   SET online=1, online_ip=<int>             │
  │              │                      │ Mongo device_shadow      │                  │
  │              │                      │   state.reported.Online=1                   │
  │              │                      │ NotifyUserDeviceAttributeChange ───────────►│
  │              │                      │                          │   Method:"AttributeChange"
  │              │                      │                          │   data:{Online:1}
```

下线流程相同，区别：`online=0`、`online_ip=0`，且 `reason ∈ {"discarded","tcp_closed"}` 跳过前端推送。

### 5.2 设备数据上报

```
Device ─pub /thing/SL100/<u>/attr/post─► EMQX ─► Backend.AttrReport()
                                                  │ Mongo DeviceReport(): 时间戳比较, 写 state.reported.*
                                                  │ websocket.NotifyUserDeviceAttributeChange (推所有绑定 uid)
                                                  │ pub /thing/SL100/<u>/attr/post/resp ─► Device
```

### 5.3 前端订阅实时消息

```
Frontend ─HTTP Upgrade /ws (Header access_token + phone_code)─► Backend
            ◄─ 101 Switching Protocols ─
Frontend ─{"method":"...","msg_id":"...","uuid":"<u>","time":...,"version":"1.0","data":{}}─►
Backend ─Reflect call → publish.Attribute / FunctionDistribute ─► EMQX ─► Device
                                                                     │
Backend ◄─ event/attr 上报推送 ◄────────────────── Device ────────┘
Backend ─WebSocket TextMessage Response ─► Frontend
```

WebSocket 心跳：客户端每 N 秒发送字符串 `ping`，服务端回 `pong`。

### 5.4 前端下发命令（以创建设备用户为例）

```
1) Frontend WS  →  Backend
   {"msg_id":"m1","method":"CreateUser","uuid":"<u>","time":...,"version":"1.0",
    "data":{"name":"Alice","role":1}}

2) Backend:
   - ValidateJWT, Device.GetByUuid, CheckBindAndGetMaster
   - redis.Set device_user_create:m1 = {"name":"Alice","role":1} (TTL 1h)
   - pub /thing/SL100/<u>/func/CreateUser  Request{m1, time, data}

3) Device → /thing/SL100/<u>/func/CreateUser/resp  {result:1, data:{id:"user_001"}}

4) Backend.FunctionDistributeResp:
   - 反射定位 funcResp.<funcName> 处理函数
   - redis.GetDeviceUserCache(m1, DeviceUserCreate) 取出 name/role
   - mongodb.NewDeviceShadow().CreateUser(uid, uuid, "user_001", "Alice", 1)
   - websocket.NotifyUserFunctionResponse(OK, "m1", uid, uuid, "CreateUser",
       {"user_id":"user_001"})

5) Frontend ← WS:
   {"msg_id":"m1","method":"ServerFunc.CreateUser","uuid":"<u>","code":0,
    "data":{"user_id":"user_001"}}
```

### 5.5 失败 / 超时 / 离线下发

| 情况 | 处理 |
|------|------|
| 设备返回 `result=0` | 后端用 `r.ErrCode`（缺省取 `MethodExecutionFailed`）经 WebSocket 回前端，data=null |
| 设备 1h 内不回复 | Redis Key `device_user_*:<msg_id>` TTL 过期；前端层加业务超时（建议 30s）+ 重发 |
| 离线设备下发 | broker 接受 publish，但设备未连接、QoS0 ⇒ 消息直接丢弃。前端应先查 `device.online` 提示 |
| broker 失联 | Paho 自动重连（2~10s 退避），`OnConnect` 回调重新订阅 |

### 5.6 WebrtcSignal 必须走异步 channel（重要！）

WebrtcSignal 是 SDP 协商信号，会高频双向（Offer/Answer/ICE/StartPush/ClosePush/Close），同步处理会**阻塞 MQTT message handler 线程**，导致其他设备消息延迟。代码用 channel 隔离：

```go
// subscribe/core.go
if event == "WebrtcSignal" {
    subEvent.PushWebrtcSignalChannel(subEvent.Async{
        ProductId: product, Uuid: uuid, Event: stc,
    })
    return // 立即返回，不阻塞
}

// 后台 goroutine 消费
func AsyncWebrtcSignal(c chan event.Async) {
    for stc := range c {
        // 处理 SDP / RPC 调用 turnService / 推送回设备
    }
}
```

**单服务复刻关键点**：
- 每个 uuid 维护一个独立 channel（buffer 推荐 1024），避免一个慢设备拖累其他设备
- WebSocket 连接的 `WebrtcSignal` method 也走 channel（`read.go: i.signal <- r`），独立 goroutine `consumptionWebrtcSignal()` 消费
- 如果你不做 WebRTC（不接对讲/视频通话），可以**整块删掉** WebrtcSignal 相关代码

### 5.7 OTA 升级流程（文档原版缺）

```
APP ──WS OtaUpgrade {flag: ["MCU","Main"]}──► Backend
                                                │
                                                │ 1. 仅 master 可调
                                                │ 2. 查 mysql.firmware_version 找该 model+flag 的最新版本
                                                │ 3. 查 mysql.firmware_version_rule 校验依赖版本
                                                │ 4. 调云存储签名 URL（rpc.GetSignUrl）
                                                ▼
                                          /thing/<m>/<u>/func/OtaUpgrade
                                          {
                                            "msg_id": "...",
                                            "time":   ...,
                                            "data": {
                                              "MCU":  {"version":"1.2.3","md5":"...","uri":"https://...","expire_time":...,"flag":"MCU"},
                                              "Main": {"version":"2.0.0","md5":"...","uri":"https://...","expire_time":...,"flag":"Main"}
                                            }
                                          }
                                                │
Device ─pub event/UpgradeStart ─► Backend ─入库 + 推 QueueDeviceUpgrade
Device ─pub event/UpgradeResult{result:1/0,msg} ─► Backend ─入库 + 推队列
```

**OtaUpgrade data 字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | string | 目标版本号字符串（如 `1.2.3`） |
| `md5` | string | 固件文件 MD5，设备下载后校验 |
| `expire_time` | int64 | 签名 URL 过期 Unix 秒，设备超时需重新申请 |
| `uri` | string | 签名后的下载 URL |
| `flag` | string | 升级目标分区（MCU / Main / 其他自定义） |

> 单服务可以用静态 OSS / S3 公开 URL 替代 `rpc.GetSignUrl`，但生产环境**强烈建议预签名**，避免固件被盗。

### 5.8 重连与订阅恢复

```go
opts := mqtt.NewClientOptions().
    SetClientID(uuid.New().String()).
    SetUsername(cfg.MqttUser).SetPassword(cfg.MqttPwd).
    SetKeepAlive(60 * time.Second).
    SetPingTimeout(2 * time.Second).
    SetCleanSession(false).
    SetAutoReconnect(true).
    SetConnectRetry(true).
    SetMaxReconnectInterval(10 * time.Second).
    SetConnectRetryInterval(2 * time.Second).
    SetResumeSubs(true).
    SetOnConnectHandler(func(c mqtt.Client) { StartSubscribe() }) // 兜底再订阅
opts.AddBroker(fmt.Sprintf("tcp://%s:%s", cfg.BrokerHost, cfg.BrokerPort))
```

设备端：建议同样 `cleanSession=false` + AutoReconnect；本地缓存关键事件，连上后择机重发。

---

## 6. 后端职责

### 6.1 消费的 MQTT 消息与处理函数

| 订阅 topic | Handler | 简述 |
|----------|---------|------|
| `/thing/+/+/attr/set/resp` | `AttributeDistributeResp` | 仅日志 |
| `/thing/+/+/attr/post` | `AttrReport` | 写影子，推 WS |
| `/thing/+/+/attr/get` | `DeviceGetAttributes` | 回设备 desired 属性 |
| `/thing/+/+/event/+` | `EventReport` → `event.GetEventType(<eventName>).Exec()` | 落 `device_event`，按需推送 |
| `/thing/+/+/func/+/resp` | `FunctionDistributeResp` → 反射 `funcResp.<funcName>()` | 与 Redis 上下文配对，回前端 |
| `$SYS/.../connected` | `Connected` | 维护在线状态 + 推送 |
| `$SYS/.../disconnected` | `Disconnected` | 同上，反向 |

### 6.2 路由 / 转发

- **设备 → 前端**
  - 属性变化：`AttrReport` → `NotifyUserDeviceAttributeChange`，按 `redis.GetUsersByUuid(uuid)` 拿全部绑定用户广播
  - 事件：`event.<X>.Exec` 入库 + `redis.PushMessage(QueueDeviceEvent, ...)` 进推送队列；`Ring`/`RingStop` 走 `NotifyUsersRing`
  - func 应答：每个 `funcResp.Response.<X>()` 自决通知哪个 uid（一般是 master）
- **前端 → 设备**：WebSocket method 反射调用 `Request.<Method>()` → 调 `publish.Attribute / FunctionDistribute / Awake`
- **设备 ↔ 设备**：仅 `WebrtcSignal` 信号转发，由后端解析后定向 publish 给目标 uuid 的 func topic

### 6.3 在线状态维护

- **数据源**：EMQX `$SYS` 系统 topic（不依赖应用层心跳）
- **存储**：MySQL `device.online`、`device.online_ip`（API 查询）；MongoDB `device_shadow.state.reported.Online` + 时间戳（影子）
- **跳过 super-user**：服务端账号上线不触发业务逻辑

### 6.4 入库 / 不入库

| 数据 | 存储 |
|------|------|
| 设备静态信息（mac、uuid、model、version、zone） | MySQL `device` |
| 用户-设备绑定关系 | MySQL `user_device` |
| 设备影子（desired/reported + metadata + version） | MongoDB `device_shadow` |
| 事件流水（开门、按铃、报警…） | MongoDB `device_event`，TTL 7 天 |
| 通知偏好 | MongoDB `device_notify` |
| MQTT 凭证 | Redis `mqtt_user:<u>` |
| MQTT ACL | Redis `mqtt_acl:<u>` |
| 绑定关系缓存 | Redis `device_bind:<u>` |
| 命令上下文（msg_id 缓存） | Redis `device_user_create:<m>` 等，TTL 1h |
| Push 队列 | Redis list（`QueueDeviceEvent`） |

### 6.5 HTTP API（建议在单服务里要做的接口）

| Method | Path | 用途 |
|--------|------|------|
| POST | `/v1/auth/login` | 登录，签发 JWT，写 Redis `user_login:<uid>:<phone_code>:<tid>` 存签名密钥 |
| POST | `/v1/device/bind` | 绑定设备（详见 §6.6 步骤） |
| POST | `/v1/device/unbind` | 解绑设备（详见 §6.7 步骤） |
| GET | `/v1/device/list` | 用户的设备列表（含 `online`） |
| GET | `/v1/device/shadow/:uuid` | 取影子（desired+reported） |
| GET | `/v1/device/event/:uuid` | 事件分页（按 belong_to.uid 过滤） |
| POST | `/v1/device/share` | 共享设备给其他用户 |
| POST | `/v1/device/unshare` | 取消共享 |
| GET | `/v1/firmware/version` | 查询设备可升级版本 |

### 6.6 设备绑定流程（单服务展开版）

原项目的绑定走 `gateway` HTTP → `deviceShadow` RPC，单服务里**必须把 RPC 调用展开为同进程函数**：

```
1. APP POST /v1/device/bind  {uuid, mac, model, version, secret, ...}
2. 后端校验：
   - JWT 合法
   - 设备未被其他 master 绑定（查 device_bind:<uuid>）
   - 校验 device_secret（与 model 配置的 ModelSecret 比对，可选）
3. 写 MySQL device 表（如不存在则 INSERT，存在则 UPDATE mac/version/model）
4. 写 MySQL user_device 表 INSERT { uuid, uid, is_master=1, name=<默认名>, appid }
5. 创建 MongoDB device_shadow 文档（uuid+uid 复合键，初始 desired/reported 为空）
6. 生成 MQTT 凭证：
   - password = RandString(16)
   - salt     = RandString(8)
   - HMSET mqtt_user:<uuid>  password_hash <sha256(password+salt)> salt <salt>
7. 写 ACL：HMSET mqtt_acl:<uuid> <按 §2.5 的 10 条 topic→permission>
8. 写绑定缓存：HSET device_bind:<uuid> <uid> 1
9. 返回 {username:<uuid>, password, secret} 给 APP
10. APP 通过蓝牙/二维码把凭证烧录到设备
11. 设备拿到凭证后用 username/password 连 broker
12. 后端订阅的 $SYS/connected 触发 → online=1 → WS 推 AttributeChange{Online:1}
13. 通过 WS 推 DeviceBind{result:1} 给 APP（uid 全端广播）
```

### 6.7 设备解绑流程（单服务展开版）

原项目解绑走多个 RPC（清 Redis、清 MongoDB、向设备 publish UnBind），单服务里要全展开：

```
1. APP POST /v1/device/unbind  {uuid, clean_data}
2. 校验是 master 操作（user_device.is_master=1）
3. 向设备 publish /thing/<m>/<uuid>/func/UnBind  {data: {clean_data: 0/1}}
   - clean_data=1：设备会清空本地用户/密码
   - 不等设备应答（设备可能已离线）
4. MySQL: DELETE FROM user_device WHERE uuid=?  （删除所有共享用户）
5. MongoDB: device_shadow.deleteOne({uuid: ?})
6. MongoDB: device_event.deleteMany({uuid: ?})  （可选，看保留策略）
7. Redis 一次性清理：
   - DEL mqtt_user:<uuid>
   - DEL mqtt_acl:<uuid>
   - DEL device_bind:<uuid>
8. 主动 `EMQX kick`（可选，确保设备立即被踢下线）
   - 通过 EMQX HTTP API: DELETE /api/v5/clients/<clientid>
   - 不做也行，下次设备重连会因找不到 mqtt_user 而被拒
9. WS 推 DeviceUnBind{uuid} 给所有原绑定 uid
10. WS 推 DevicesChanged 给 master uid，让 APP 重新拉列表
```

> ⚠️ **顺序很重要**：先 publish UnBind，后删 ACL/凭证。否则先删凭证设备会立即断线，UnBind 命令永远到不了设备。

---

## 7. 数据存储

### 7.1 MySQL `device`

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int 自增 | 主键 |
| `mac` | varchar(32) | 设备 MAC |
| `uuid` | varchar(64) | 设备 UUID（唯一索引） |
| `version` | varchar(32) | 固件版本 |
| `model` | varchar(32) | 型号，例 `SL100` |
| `online_ip` | bigint | IPv4 转 int 存储 |
| `zone` | double | 时区 |
| `online` | tinyint | 1=在线 0=离线 |
| `update_time` | bigint | 状态最近更新 Unix 秒 |
| `active_time` | bigint | 最近活跃 Unix 秒 |

写入：上线/下线事件触发 `UPDATE device SET online=?, online_ip=?, update_time=? WHERE uuid=?`。

### 7.2 MySQL `user_device`

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int 自增 | 主键 |
| `uuid` | varchar(64) | 设备 UUID |
| `uid` | varchar(64) | 用户 ID |
| `is_master` | tinyint | 1=主人 0=共享 |
| `name` | varchar(64) | 设备别名 |
| `appid` | varchar(64) | 客户端 appid |

权限校验：`SELECT ... FROM user_device WHERE uuid=? AND uid=?`。

### 7.3 MongoDB

#### 集合 `device_shadow`

```go
type Shadow struct {
    Uuid      string   `bson:"uuid"`
    Uid       string   `bson:"uid"`
    State     struct {
        Desired  map[string]interface{} `bson:"desired"`
        Reported map[string]interface{} `bson:"reported"`
    } `bson:"state"`
    MetaData  struct {
        Desired  map[string]struct{ Timestamp int64 `bson:"timestamp"` } `bson:"desired"`
        Reported map[string]struct{ Timestamp int64 `bson:"timestamp"` } `bson:"reported"`
    } `bson:"metadata"`
    Timestamp int64    `bson:"timestamp"` // 整文档最近更新 (秒)
    Version   int64    `bson:"version"`   // 每次写入 +1
}
```

固定 attribute：`Online`、`Users`、`SleepState`、`Model`。其余按产品自定义。

索引：`uuid + uid`（unique）。

#### 集合 `device_event`

```go
type DeviceEvent struct {
    Id          string
    Uuid        string
    DeviceName  string
    Type        int64        // 见下表枚举
    Time        int64        // 设备上报时间（秒）
    DeviceTime  int64
    Thumbnail   string       // OSS / S3 路径
    Payload     interface{}  // 各事件特有字段
    BelongTo    []struct {
        Uid     string
        IsRead  int64
    }
    ExpireAt    time.Time    // = now + 7d，TTL 索引
}
```

事件 Type 枚举：

| Type | 名称 |
|------|------|
| 1 | OpenDoor |
| 2 | Tamper |
| 3 | Ring |
| 4 | Pir |
| 5 | Radar |
| 6 | Stay |
| 7 | OpenDoorFailedMax |
| 8 | WaitingLockTimeout |
| 9 | UpgradeResult |
| 10 | UpgradeStart |
| 11 | LowBattery |
| 12 | OpenDoorFailed |
| 13 | Conflagration |
| 14 | LockStatusNotChangedInADay |

索引：
- `uuid + time`（用户事件分页）
- `belong_to.uid + belong_to.is_read`（未读统计）
- `expire_at`（TTL 索引，自动清理 7 天前数据）

### 7.4 Redis Key 总览

| Key | 类型 | 用途 | TTL |
|-----|------|------|-----|
| `mqtt_user:<uuid>` | hash | EMQX 鉴权（password_hash、salt、is_superuser） | 永久 |
| `mqtt_acl:<uuid>` | hash | EMQX ACL（topic→permission） | 永久 |
| `device_bind:<uuid>` | hash | 设备绑定的所有 uid（field=uid，value=is_master） | 永久 |
| `user_login:<uid>:<phone_code>:<tid>` | string | JWT 签名密钥（登录时设，验签时取） | 与 token TTL 一致 |
| `device_user_create:<msg_id>` | string | CreateUser 命令上下文 | 1h |
| `device_user_update:<msg_id>` | string | UpdateUser 命令上下文 | 1h |
| `device_user_del:<msg_id>` | string | DelUser 命令上下文 | 1h |
| `device_user_pwd_create:<msg_id>` | string | AddPwd 命令上下文 | 1h |
| `device_user_pwd_update:<msg_id>` | string | UpdatePwd 命令上下文 | 1h |
| `device_user_pwd_del:<msg_id>` | string | DelPwd 命令上下文 | 1h |
| `device_background_image:<msg_id>` | string | SetBackgroundImage 上下文（含 uid+phone_code+image） | 1h |
| `device_change_cam:<msg_id>` | string | ChangeCam 上下文（含 uid+phone_code+flag） | 1h |
| `wake_up:<uuid>` | string | Awake 标记，避免重复唤醒 | 短 TTL |
| `queue:device_event` | list | 通用事件 push 队列（铃声/开门/低电报警等）| – |
| `queue:device_upgrade` | list | OTA 升级事件队列（UpgradeStart/Result）| – |
| `queue:device_share` | list | 设备分享通知队列 | – |
| `queue:device_family` | list | 家庭成员通知队列 | – |
| `queue:remote_login` | list | 远程登录通知队列 | – |
| `queue:verification_code` | list | 验证码下发队列 | – |
| `queue:lock_status_not_changed_last` | list | 门锁长时间未操作告警队列 | – |

> ⚠️ **`device_background_image` 与 `device_change_cam` 的 Key 维度是 `msg_id` 而非 uuid**——因为这两个命令的应答需要**精确推回发起的那个端**（同一 uid 多端登录时区分）。Cache 里要存 `phone_code`，func 应答处理用 `NotifyUserFunctionResponseByPhoneCode()` 而不是广播。

### 7.5 心跳 / 命令记录 / 数据保留策略

| 类型 | 策略 |
|------|------|
| 心跳 | 不存数据库；MQTT keepalive=60s 即心跳；EMQX 自行判断离线 |
| 命令记录 | 仅 Redis 短期上下文（1h），命令本身不入库；最终结果反映在 `device_shadow` 与 `device_event` |
| 事件 | MongoDB TTL 7 天 |
| 影子 | 永久，绑定/解绑触发清理 |
| 在线状态 | MySQL 永久最新值；MongoDB 影子内含历史时间戳但只保留最新值 |

---

## 8. 异常与边界条件

| 场景 | 处理 |
|------|------|
| **重复消息**（同 `msg_id` 二次到达） | 1) 属性级以 `metadata.reported.<attr>.timestamp` 比较，旧/等于丢弃；2) func 命令以 Redis Key 单次读+消费实现幂等 |
| **乱序消息** | 同上，时间戳比较保证后到的旧时间戳不会覆盖新值 |
| **设备断线重连** | 设备 `cleanSession=false` + AutoReconnect；上线时 EMQX 再次发 `connected`，后端重置 `online=1` |
| **broker 重启** | 后端 Paho 自动重连（2~10s 退避），`OnConnect` 回调里再次 `StartSubscribe()`；`ResumeSubs=true` 协助恢复 |
| **后端单服务重启** | 重连后重新订阅；期间 `attr/post` QoS0 消息丢失。改进建议：QoS1 + 持久会话 + broker 端订阅持久化 |
| **命令超时** | 设备 1h 未应答 → Redis 命令上下文过期，前端无回调；前端层加业务超时（建议 30s）+ 重发 |
| **离线设备下发** | publish 成功但设备未连接，QoS0 直接丢弃。前端应先查 `device.online` 提示"设备离线" |
| **WS 重复登录** | 升级前 `GetUserPhoneConn(uid, pco).Close()`，显式踢掉旧连接（同 uid+phone_code） |
| **同 uid 多端在线** | 用 phone_code 区分；推送消息按 uid 全量广播，多端同时收到 |
| **discarded / tcp_closed** | EMQX 闪断瞬时上报，后端跳过通知；避免 App 频闪 |

### 8.1 安全风险与防护

| 风险 | 当前实现 / 建议 |
|------|----------------|
| 明文传输 | 当前**未启 TLS**。生产必须启用 `mqtts://:8883` + 双向证书或至少服务端证书 |
| 弱密码 | 服务账号示例 `123456`，**部署前必须替换**；设备密码 16 字节随机已足够 |
| ACL 遗漏 | 绑定时由后端写入 `mqtt_acl:<u>`；解绑时**必须**`DEL mqtt_user:<u>`、`DEL mqtt_acl:<u>` |
| 重放攻击 | `msg_id` 一次性 + 时间戳校验；建议增加 nonce + HMAC 签名 |
| 越权订阅 | EMQX Redis ACL 限定 topic 范围；不要让设备拿到 `#` 权限 |
| WebSocket 鉴权 | JWT + phone_code，过期靠 `ValidateToken`；建议加 IP / 频次限流 |
| Topic 注入 | `funcName` / `eventName` 来自 topic 路径，分割后做白名单（`GetEventType` 不在枚举返回 nil；func 用反射，不存在的方法直接拒绝） |
| 敏感字段 | 设备 `enc/data/salt` 是业务加密块，不要打日志；生产环境 Debug 日志必须脱敏或关闭 |

---

## 附录 A：WebSocket 报文（前后端协议）

### A.1 Request

```json
{
  "msg_id": "<uuid>",
  "method": "<MethodName | ServerFunc.FuncName>",
  "uuid":   "<deviceUuid>",
  "time":   1714435400,
  "version": "1.0",
  "data":   {}
}
```

必填校验：`method`、`version`、`msg_id`、`time`、`uuid` 任一为空直接丢弃。

### A.2 Response

```json
{
  "msg_id": "<原 msg_id>",
  "method": "<同请求 Method 或 ServerFunc.<X> 推送>",
  "uuid":   "<deviceUuid>",
  "time":   1714435401,
  "code":   0,
  "msg":    "ok",
  "data":   {}
}
```

### A.3 服务端主动推送 Method

| Method | 触发 | data | 推送函数 / 范围 |
|--------|------|------|----------------|
| `DeviceBind` | 绑定结果 | `{"uuid":"...","result":1}` | `NotifyDeviceBindResult()` 按 uid 广播 |
| `DeviceUnBind` | 解绑 | `{"uuid":"..."}` | `NotifyDeviceUnBind()` 按 uid 广播 |
| `DevicesChanged` | 设备列表变更（让前端拉取） | null | `NotifyDevicesChanged()` 按 uid 广播 |
| `AttributeChange` | 设备属性变化（含上线/离线 Online 字段） | `{ "<attr>": <value>, ... }` | `NotifyUserDeviceAttributeChange()` 按 uid 广播 |
| `Ring` / `RingStop` | 门铃响 / 停 | `{"uuid":"...","type":"Ring","device_name":"..."}` | `NotifyUsersRing()` 按 uid 数组广播；返回离线 uid 列表 |
| `ServerFunc.<funcName>` | func 命令应答（CreateUser、UpdateUser、DelUser、AddPwd、DelPwd、UpdatePwd 等） | 各 func 自定义；失败时 code 取 `r.ErrCode` 或 `MethodExecutionFailed` | `NotifyUserFunctionResponse()` 按 uid 广播 |
| `ServerFunc.SetBackgroundImage` | 主题背景图设置应答 | `{"image":"..."}` | `NotifyUserFunctionResponseByPhoneCode()` **只推发起端** |
| `ServerFunc.ChangeCam` | 切换镜头应答 | `{"flag":"..."}` | `NotifyUserFunctionResponseByPhoneCode()` **只推发起端** |
| `ServerFunc.WebrtcSignal` | WebRTC 信号转发（Offer/Answer/ICE/Candidate） | `{type, sdp, content, ...}` | `NotifyUserEventByPhoneCode()` **只推发起端** |

### A.4 前端可调用的全部 Method（反射识别，单服务复刻必须实现）

> WebSocket 入参的 `method` 字段用反射调用 `Request.<method>()`。下表盘点了 `socket/websocket/*.go` 里全部公开方法，**少实现一个前端就调不通对应功能**。

| Method | 文件 | 权限 | 立即响应 vs 等设备 resp | data 例 |
|--------|------|------|----------------------|--------|
| `SetAttribute` | attribute.go | 绑定用户 | **立即响应**（同步写影子 + publish 后即返回 ok） | `{"SleepState":1,...}` |
| `GetAttribute` | attribute.go | 绑定用户 | **立即响应**，返回影子 desired+reported+metadata | `["Online","Battery"]`（attr 名数组，空则全返） |
| `SetNotifyStatus` | attribute.go | 绑定用户 | 立即响应 | `{"type":"Ring","status":1}` |
| `GetNotifyStatus` | attribute.go | 绑定用户 | 立即响应，返通知偏好 | – |
| `SleepState` | action.go | 绑定用户 | **立即响应**（属性下发，无需等 resp） | `{"status":0/1}` |
| `Awake` | action.go | 绑定用户 | 立即响应，并写 `wake_up:<uuid>` 标记 | `{"type":...,"command":...}` |
| `CreateUser` | user.go | **仅 master** | **不立即响应**——等 `func/CreateUser/resp` 才推 ServerFunc | `{"name":"...","role":1}` |
| `UpdateUser` | user.go | **仅 master** | 不立即响应 | `{"user_id":"...","name":"...","role":1}` |
| `DelUser` | user.go | **仅 master** | 不立即响应 | `{"user_id":"..."}` |
| `AddPwd` | user.go | **仅 master** | 不立即响应 | `{"data":"...","start":...,"exp":...}` |
| `UpdatePwd` | user.go | **仅 master** | 不立即响应 | – |
| `DelPwd` | user.go | **仅 master** | 不立即响应 | `{"id":"..."}` |
| `Lock` | lock.go | 绑定用户 | 立即响应，自动注入 `{uid, username}` | – |
| `Unlock` | lock.go | 绑定用户 | 立即响应，自动注入 `{uid, username}` | – |
| `SetBluetoothSwitch` | lock.go | **仅 master** | 立即响应 | `{"id":...,"mac":...,"secret":...}` |
| `DelBluetoothSwitch` | lock.go | **仅 master** | 立即响应 | `{"id":...}` |
| `OtaUpgrade` | upgrade.go | **仅 master** | 立即响应；后端会查固件库+签 URL | `{"flag":["MCU","Main"]}` |
| `Reboot` | upgrade.go | **仅 master** | 立即响应 | – |
| `ResetSetting` | upgrade.go | **仅 master** | 立即响应 | – |
| `SetBackgroundImage` | upgrade.go | 绑定用户 | **不立即响应**（cache→publish→等 resp→`ByPhoneCode` 推回） | `{"image":"..."}` |
| `Call` | upgrade.go | **仅 master** | 立即响应，远程门铃呼叫 | – |
| `LeaveWord` | leaveword.go | 绑定用户 | – | – |
| `WebrtcSignal` | webrtc.go | 绑定用户 | **走 channel 异步**，立即返回 ok（或 SDP 信号回填） | `{type, sdp, interaction, send_to}` |
| `ChangeCam` | webrtc.go | 绑定用户 | **不立即响应**（cache→publish→等 ChangeCamResp→`ByPhoneCode` 推回） | `{"flag":"..."}` |
| `OthersAnswer` | webrtc.go | 绑定用户 | 立即响应，触发 RingStop 推送给其他成员 | – |
| `SetP2pState` | webrtc.go | 绑定用户 | 立即响应（调 turn 服务 RPC） | `{"is_p2p":true}` |
| `QuickReplyFile` | action.go | – | **空实现**（占位） | – |

> ⚠️ **"不立即响应"的 method 切勿在调用 publish 后再调 `r.response()`**——代码用注释 `//r.response(response.OK,"ok")` 显式标记。原因：前端要等 `ServerFunc.<x>` 推送，如果 publish 后立即返 `code=0 method=CreateUser`，前端会以为已经成功而不等真正的设备应答。**这是单服务复刻最容易踩的坑**。

---

## 附录 B：单服务复刻清单（落地步骤）

按下列顺序实施即可在单体项目中跑起来：

### B.1 基础设施

1. 部署 EMQX（推荐 5.x），开启：
   - Redis 鉴权 / ACL 模块
   - `$SYS` 系统主题
   - TCP 1883（生产追加 8883 TLS）
2. 部署 MySQL、MongoDB、Redis；按 §7.1～7.4 建表 / 集合 / 索引（重点：MongoDB 的 `device_event.expire_at` TTL 索引）。

### B.2 服务账号

3. Redis 预置一个 super-user：
   ```
   HMSET mqtt_user:backend_service \
       password_hash <sha256(<明文>+<salt>)> \
       salt <8字符随机> \
       is_superuser 1
   ```
   把明文 username/password 写入服务配置。

### B.3 后端代码模块（同进程）

4. **MQTT client 模块**：用 §5.6 的 options 启动；订阅 §3.3 列出的 7 条 topic；handler 按 §6.1 注册。
5. **业务 Handler**：
   - `AttrReport`：写 MongoDB `device_shadow`（带时间戳比较）→ 推送 WebSocket → publish ACK
   - `EventReport`：按 `eventName` 分发到具体 Exec → 落 `device_event` → 推队列/推送 → publish ACK
   - `FunctionDistributeResp`：按 `funcName` 反射处理 → 取 Redis 上下文 → 写库 → 推送 WS
   - `Connected/Disconnected`：filter super-user → 更新 MySQL/MongoDB → 推送 WS（注意 reason 过滤）
6. **WebSocket 模块**：
   - `/ws` endpoint，校验 `access_token` + `phone_code`（JWT）
   - 维护 `<uid, phone_code> → conn`，新连接踢旧连接
   - 收 `ping` 回 `pong`
   - 收 Request → 反射调 `Request.<Method>()` → 调 publish 函数
   - 提供 `NotifyUserDeviceAttributeChange / NotifyUserFunctionResponse / NotifyUsersRing` 等
7. **HTTP API**：实现 §6.5 表中接口；绑定接口里：
   - 生成 16 字节随机 password
   - `HMSET mqtt_user:<uuid> ...`
   - `HMSET mqtt_acl:<uuid> ...`（按 §2.5 表）
   - 把明文 password 一次性返给设备

### B.4 配置文件骨架

```yaml
# config.yaml（建议字段）
HttpPort: "8080"          # HTTP + WebSocket 共用
WsPath:   "/ws"

Mqtt:
  Host: "127.0.0.1"
  Port: 1883
  Username: "backend_service"
  Password: "<change-me>"
  KeepLiveTimeout: 60       # 注意：项目里就是这么拼的
  PingTimeout: 2

Mysql:
  Host: "127.0.0.1"
  Port: 3306
  User: "root"
  Password: "<change-me>"
  Db: "iot"
  Charset: "utf8"

MongoDB:
  Url: "mongodb://localhost:27017"
  DB:  "device_shadow"

Redis:
  Addr: "127.0.0.1:6379"

Jwt:
  Secret: "<change-me>"
  TTL: "7d"
```

### B.5 推送队列消费者（pushService 等价物）

原项目用独立 `pushService` 进程消费 Redis list。单服务可以**起一个 goroutine pool 内嵌消费**，逻辑：

```go
// 推荐做法：每个队列起一个 worker pool
queues := []string{
    "queue:device_event",
    "queue:device_upgrade",
    "queue:device_share",
    "queue:device_family",
    "queue:remote_login",
    "queue:verification_code",
    "queue:lock_status_not_changed_last",
}
for _, q := range queues {
    for i := 0; i < workerNum; i++ {
        go func(queue string) {
            for {
                msg, err := redis.BLPop(60*time.Second, queue).Result()
                if err != nil { continue }
                handlePushMessage(queue, msg[1])  // 按 queue 分发到不同 handler
            }
        }(q)
    }
}
```

队列消息 payload（以 `queue:device_event` 为例）：
```json
{
  "appid": "<app_id>",
  "type":  "OpenDoor",
  "device_name": "前门",
  "uuid": "...",
  "time": 1714435300,
  "users": [
    {"uid":"u1","push_type":1,"language":"zh-CN","token":"<APNS/FCM token>"},
    ...
  ]
}
```

`handlePushMessage` 里按 `push_type` 走 APNS / FCM / 厂商通道。如果你的项目不做 push（只用 WebSocket），这部分可以**整段不实现**——所有事件已经通过 WebSocket 实时推到在线用户，离线 push 是锦上添花。

### B.6 上线检查清单

- [ ] 替换所有默认密码（broker super-user / MySQL / MongoDB / Redis / JWT secret）
- [ ] 启用 TLS（broker 与前端 wss）
- [ ] WebSocket 加 origin 白名单 + IP / 频次限流
- [ ] 关闭或脱敏 Debug 级 payload 日志
- [ ] 解绑流程按 §6.7 顺序执行（先 publish UnBind，再删凭证 / ACL / 绑定缓存）
- [ ] MongoDB 的 TTL 索引验证生效（手插一条 `expire_at = now-1h` 看是否被清理）
- [ ] 验证 `$SYS/.../{connected,disconnected}` 后端能收到（EMQX 默认开启，但部分版本需手动 enable）
- [ ] 设备凭证生成必须**只在绑定接口返回一次明文**，不存明文落库
- [ ] **§A.4 表里所有 method 都实现**（少一个前端就有按钮点不动）
- [ ] **"不立即响应"的 method（CreateUser/AddPwd/...）切勿在 publish 后调 r.response()**
- [ ] **`SetBackgroundImage` / `ChangeCam` 用 `NotifyUserFunctionResponseByPhoneCode`** 单端推送
- [ ] **WebrtcSignal 走 channel 异步**，不阻塞 MQTT handler
- [ ] **RingStop 不入库**，其余事件按 §3.4 入库矩阵实现
- [ ] disconnected reason ∈ `{"discarded","tcp_closed"}` 时跳过前端推送

---

文档结束。所有 topic、字段名、状态值、表结构、Redis Key 来自仓库 `deviceShadow/`、`gateway/`、`simulatedDevice/` 的实际实现，可作为单服务复刻的协议契约直接使用。
