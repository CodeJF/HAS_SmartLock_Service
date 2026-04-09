# HTTP API 服务索引（整合版 · 供后端开发参考）

> 整合自 `api_index.md`、`auth_rules.md`、`response_envelope.md` 及实际测试集合。
> 这是面向后端开发的完整版本，包含了请求壳、响应壳和签名规则。
> 最后更新：2026-04-08

---

## 零、全局规则（必须阅读）

### 1. 统一响应格式 (Response Envelope)
针对所有的 HTTP 接口请求，除特殊情况外，后端均必须返回统一结构的 JSON：

```json
{
  "code": 1000,
  "msg": "ok",
  "data": { ... }
}
```
* **`code`**：`1000` 表示成功；`1004` 表示客户端需重新同步时间（触发 /time 修正）；`1005` 表示 access_token 失效，需走 refresh 刷新；其他值纯业务失败。
* **`msg`**：提示信息。
* **`data`**：可空，按接口文档声明返回具体数据。

### 2. Header 参数与接口鉴权签名
绝大多数用户端接口无论是（`Auth: ❌` 还是 `Auth: ✅`）都需要携带防篡改签名。除特例，以下是常规全局 Headers：

| 字段 | 类型 | 必填 (针对常规请求) | 说明 |
|------|------|------|------|
| `appid` | `string` | ✅ | 应用 ID |
| `app_version` | `string` | ✅ | 应用版本号 |
| `phone_code` | `string` | ✅ | 手机国家区号 |
| `timestamp` | `string` | ✅ | Unix 秒级时间戳（考虑到偏差纠正） |
| `request_id` | `string` | ✅ | 请求唯一 ID (UUID v4) |
| `sign` | `string` | ✅ | 签名（见下文计算算法） |
| `access_token` | `string` | 按文档标识 `Auth` 决定 | **当 `Auth: ✅` 时，必须携带。** |

#### 用户端 API 签名算法 (`sign`)：
```javascript
// 1. 生成基础字符串
baseString = HTTP_METHOD + "&" +
             access_token +
             app_version +
             appid +
             phone_code +
             request_id +
             timestamp +
             "&" + canonicalQueryOrBodyParams;
             
// 注意：
// - 仅当请求实际携带 access_token Header 时，access_token 才参与签名；未携带时按空字符串处理。
// - 如果存在 Query 或 Body Params，先对它们进行 Key 的 ASCII 升序排列，并拼装成 Key=Value&Key2=Val2 形式作为 canonicalQueryOrBodyParams。没有则留空字符串。

// 2. HMAC-SHA256(baseString, secret_key)
// 3. 将 HMAC 原始结果转成十六进制字符串
// 4. 再将该十六进制字符串按 UTF-8 字节做 Base64，赋值给 Header "sign"
```

---

## 一、用户模块 (`/v1/user/*`)

### 1. 用户登录
- **Method**: `POST`
- **Path**: `/v1/user/login`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 登录账号（中国手机号或邮箱） |
  | `type` | `string` | ✅ | 登录方式：`"password"` / `"code"` |
  | `password` | `string` | ✅ | 密码（type=password 时必填） |
  | `phone_brand` | `string` | ✅ | 手机品牌 |
  | `code` | `string` | ❌ | 验证码（type=code 时必填） |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `uid` | `string` | 用户 ID |
  | `access_token` | `string` | 访问令牌 |
  | `refresh_token` | `string` | 刷新令牌 |
  | `is_debug` | `int` | 是否调试账号 |
  | `expiration` | `int` | 过期时间（Unix 秒） |

---

### 2. 发送登录验证码
- **Method**: `POST`
- **Path**: `/v1/user/loginSend`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 登录账号 |
  | `country` | `string` | ✅ | 国家区号（如 `"86"`） |
- **返回值**: `null`

---

### 3. 发送注册验证码
- **Method**: `POST`
- **Path**: `/v1/user/registerSend`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 注册账号 |
  | `country` | `string` | ✅ | 国家区号 |
- **返回值**: `null`

---

### 4. 注册
- **Method**: `POST`
- **Path**: `/v1/user/register`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 注册账号 |
  | `country` | `string` | ✅ | 国家区号 |
  | `code` | `string` | ✅ | 验证码 |
  | `password` | `string` | ❌ | 密码（md5 后） |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `uid` | `string` | 用户 ID |
  | `access_token` | `string` | 访问令牌 |
  | `refresh_token` | `string` | 刷新令牌 |
  | `is_debug` | `int` | 是否调试账号 |
  | `expiration` | `int` | 过期时间（Unix 秒） |

---

### 5. 检查验证码
- **Method**: `POST`
- **Path**: `/v1/user/validateCode`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 账号 |
  | `code` | `string` | ✅ | 验证码 |
  | `type` | `string` | ✅ | 验证类型：`"login"` / `"register"` / `"reset"` |
- **返回值**: `null`

---

### 6. 发送忘记密码验证码
- **Method**: `POST`
- **Path**: `/v1/user/resetSend`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 登录账号 |
  | `country` | `string` | ✅ | 国家区号 |
- **返回值**: `null`

---

### 7. 忘记密码（重置密码）
- **Method**: `POST`
- **Path**: `/v1/user/reset`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 登录账号 |
  | `code` | `string` | ✅ | 验证码 |
  | `password` | `string` | ✅ | 新密码 |
- **返回值**: `null`

---

### 8. 刷新 Token
- **Method**: `POST`
- **Path**: `/v1/user/refresh`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `refresh_token` | `string` | ✅ | 刷新令牌 |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `uid` | `string` | 用户 ID |
  | `access_token` | `string` | 新访问令牌 |
  | `refresh_token` | `string` | 新刷新令牌 |
  | `is_debug` | `int` | 是否调试账号 |
  | `expiration` | `int` | 过期时间（Unix 秒） |

---

### 9. 获取用户信息
- **Method**: `GET`
- **Path**: `/v1/user/info`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `username` | `string` | 登录账号 |
  | `nickname` | `string?` | 昵称 |
  | `avatar` | `string?` | 头像 URL |
  | `is_debug` | `int?` | 是否调试账号 |
  | `register_time` | `int?` | 注册时间（Unix 秒） |

---

### 10. 获取上传用户头像地址
- **Method**: `GET`
- **Path**: `/v1/user/putAvatar`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值**: TODO（无示例）

---

### 11. 修改密码
- **Method**: `POST`
- **Path**: `/v1/user/updatePwd`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `new_password` | `string` | ✅ | 新密码 |
- **返回值**: `null`

---

### 12. 修改昵称
- **Method**: `POST`
- **Path**: `/v1/user/updateInfo`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `nickname` | `string` | ✅ | 新昵称 |
- **返回值**: `null`

---

### 13. 上报登录设备信息
- **Method**: `POST`
- **Path**: `/v1/user/putClient`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `push_type` | `int` | ✅ | 推送类型 |
  | `push_token` | `string` | ✅ | 推送凭证 |
  | `brand` | `string` | ❌ | 设备品牌（如 iPhone） |
  | `version` | `string` | ❌ | 设备系统版本 |
  | `language` | `string` | ❌ | 语言（如 `zh_CN`） |
  | `zone` | `string` | ❌ | 时区 |
- **返回值**: `null`

---

### 14. 退出登录
- **Method**: `POST`
- **Path**: `/v1/user/logout`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值**: `null`

---

### 15. 发送删除帐号验证码
- **Method**: `POST`
- **Path**: `/v1/user/deleteSend`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 登录帐号 |
- **返回值**: `null`

---

### 16. 删除帐号
- **Method**: `POST`
- **Path**: `/v1/user/delete`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 登录帐号 |
  | `code` | `string` | ✅ | 验证码 |
- **返回值**: `null`

---

## 二、设备模块 (`/v1/device/*`)

> **[❗] 设备端专属 API 注意事项**：
> 下方 `设备绑定` 与 `设备登录` 也是由设备端通过 HTTP 调用后端的接口，但它们的鉴权头与签名规则与用户端不同。使用 `model_secret` 而不是用户的 `secret_key`。
> **设备端签名算法**:
> `baseString = METHOD + "&" + model + request_id + timestamp + uuid + "&" + canonicalQueryOrBodyParams`
> `sign = Base64(HmacSHA256(baseString, model_secret))`

### 16.1 设备绑定 (仅限设备端调用)
- **Method**: `POST`
- **Path**: `/v1/device/bind`
- **Auth**: 设备专属认证
- **参数方式**: Body (JSON)
- **请求头强制验证项**: `model`, `uuid`, `appid`, `timestamp`, `request_id`, `sign`
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uid` | `string` | ✅ | 用户 ID |
  | `mac` | `string` | ✅ | MAC 地址 |
  | `zone` | `string` | ✅ | 时区 (如 `"8.00"`) |
  | `version` | `string` | ✅ | 固件版本号 |
- **返回值**: `null` (返回标准通用基础响应, 下同)

---

### 16.2 设备登录 (仅限设备端调用)
- **Method**: `POST`
- **Path**: `/v1/device/login`
- **Auth**: 设备专属认证
- **参数方式**: Body (JSON)
- **请求头强制验证项**: `model`, `uuid`, `uid`, `timestamp`, `request_id`, `sign`
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `zone` | `string` | ✅ | 时区 (如 `"8.00"`) |
  | `version` | `string` | ✅ | 固件版本号（如 `"SL100_BP_1.01.10"`） |
- **返回值**: `null`

---

### 17. 设备列表
- **Method**: `GET`
- **Path**: `/v1/device/list`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ❌ | 家庭 ID（筛选） |
- **返回值** (`data`): `Array<Device>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `id` | `int` | 绑定记录 ID |
  | `uuid` | `string` | 设备 UUID |
  | `device_id` | `string` | 设备唯一 ID |
  | `uid` | `string` | 用户 ID |
  | `bind_type` | `int` | 绑定类型（1=主用户, 2=共享用户） |
  | `secret` | `string` | 设备密钥 |
  | `name` | `string` | 设备名称 |
  | `first_bind_time` | `int` | 首次绑定时间（Unix 秒） |
  | `bind_time` | `int` | 最近绑定时间（Unix 秒） |
  | `State` | `object` | 设备状态（影子） |
  | `State.desired` | `object` | 期望状态 |
  | `State.reported` | `object` | 设备上报状态（见下表） |

  **`State.reported` 字段**:
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `Charging` | `number` | 是否充电中（0/1） |
  | `CurrentBattery` | `number` | 当前电量百分比 |
  | `EmergencySupplyStatus` | `number` | 应急供电状态 |
  | `Language` | `number` | 语言设置 |
  | `LeaveWordNotify` | `number` | 留言提醒开关（0/1） |
  | `LockStatus` | `number` | 锁状态（0=关, 1=开） |
  | `LowBattery` | `number` | 低电量阈值 |
  | `Mac` | `string` | MAC 地址 |
  | `MaxBattery` | `number` | 最大电量（通常 100） |
  | `Model` | `string` | 设备型号（如 SL100） |
  | `MotionDetectionNotify` | `number` | 移动侦测提醒（0/1） |
  | `NightVisionStatus` | `number` | 夜视状态（0/1） |
  | `Online` | `number` | 是否在线（0=离线, 1=在线） |
  | `OpenDoorNotify` | `number` | 开门提醒（0/1） |
  | `OpeningDirection` | `number` | 开门方向 |
  | `RSSI` | `number` | 信号强度 |
  | `RadarDetectionSensitivity` | `number` | 雷达灵敏度 |
  | `RadarDetectionSw` | `number` | 雷达开关（0/1） |
  | `SceneMode` | `number` | 场景模式 |
  | `SleepState` | `number` | 休眠状态 |
  | `Sn` | `string` | 设备序列号 |
  | `Uuid` | `string` | 设备 UUID（冗余） |
  | `Volume` | `number` | 音量（0–100） |
  | `Version_Front` | `string` | 前面板版本 |
  | `Version_MCU` | `string` | MCU 版本 |
  | `Version_Rear` | `string` | 后面板版本 |
  | `Version_XR806` | `string` | XR806 模块版本 |
  | `UpgradeStatus` | `object` | 升级状态 |
  | `UpgradeStatus.flag` | `string` | 当前升级模块 |
  | `UpgradeStatus.schedule` | `number` | 升级进度 |
  | `UpgradeStatus.step` | `number` | 升级步骤 |
  | `WIFI` | `object` | WiFi 信息 |
  | `WIFI.ssid` | `string` | WiFi 名称 |
  | `WIFI.pwd` | `string` | WiFi 密码（脱敏） |

---

### 18. 新设备列表
- **Method**: `GET`
- **Path**: `/v1/device/newList`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值** (`data`): `Array<NewDevice>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `uuid` | `string` | 设备 UUID |
  | `device_id` | `string` | 设备唯一 ID |
  | `uid` | `string` | 用户 ID |
  | `bind_type` | `int` | 绑定类型 |
  | `secret` | `string` | 设备密钥 |
  | `name` | `string` | 设备名称 |
  | `bind_status` | `int` | 绑定状态 |
  | `first_bind_time` | `int` | 首次绑定时间（Unix 秒） |
  | `bind_time` | `int` | 最近绑定时间（Unix 秒） |
  | `delete_time` | `int` | 删除时间（Unix 秒） |

---

### 19. 设备型号列表
- **Method**: `GET`
- **Path**: `/v1/device/models`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值** (`data`): `Array<DeviceModel>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `model_code` | `string` | 型号代码 |
  | `status` | `int` | 状态 |
  | `model_name` | `string` | 型号名称 |
  | `category` | `string` | 分类 |
  | `show_name` | `string` | 显示名称 |
  | `default_name` | `string` | 默认名称 |
  | `thumbnail` | `string` | 缩略图 URL |

---

### 20. 修改设备名称
- **Method**: `POST`
- **Path**: `/v1/device/upName`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
  | `name` | `string` | ✅ | 新设备名称 |
- **返回值**: `null`

---

### 21. 获取设备可更新版本
- **Method**: `GET`
- **Path**: `/v1/device/upgradedVersion`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
  | `flag` | `string` | ✅ | 升级模块标识 |
- **返回值** (`data`): `Array<DeviceUpgrade>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `has` | `bool` | 是否有可更新版本 |
  | `version` | `object` | 版本详情 |
  | `version.flag` | `string` | 升级模块标识 |

---

### 22. 移除设备
- **Method**: `DELETE`
- **Path**: `/v1/device/remove`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
  | `clean_data` | `bool` | ❌ | 是否清除数据（仅创建者 role=1 可传） |
- **返回值**: `null`

---

### 23. 重置设备
- **Method**: `POST`
- **Path**: `/v1/device/reset`
- **Auth**: ❌
- **参数方式**: 无参数
- **返回值**: TODO（无示例）

---

## 三、设备分享 (`/v1/device/share*`)

### 24. 分享设备
- **Method**: `POST`
- **Path**: `/v1/device/share`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 被分享者账号 |
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值**: `null`

---

### 25. 分享记录
- **Method**: `GET`
- **Path**: `/v1/device/shareRecords`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值** (`data`): `Array<ShareRecord>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `username` | `string?` | 被分享者账号 |
  | `uuid` | `string?` | 设备 UUID |
  | `uid` | `string?` | 被分享者 UID |
  | `status` | `int?` | 分享状态 |
  | `role` | `int?` | 角色 |

---

### 26. 设备拥有者移除分享者
- **Method**: `DELETE`
- **Path**: `/v1/device/shareDelete`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uid` | `string` | ✅ | 被分享者 UID |
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值**: `null`

---

### 27. 分享反馈
- **Method**: `POST`
- **Path**: `/v1/device/shareFeedback`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `msg_id` | `string` | ✅ | 消息 ID |
  | `status` | `int` | ✅ | 反馈状态（1=同意, 2=拒绝） |
- **返回值**: `null`

---

## 四、家庭模块 (`/v1/device/home*`)

### 28. 创建家庭
- **Method**: `POST`
- **Path**: `/v1/device/homeCreate`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `name` | `string` | ✅ | 家庭名称 |
- **返回值**: `null`

---

### 29. 家庭列表
- **Method**: `GET`
- **Path**: `/v1/device/homes`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值** (`data`): `Array<Home>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `id` | `string` | 家庭 ID |
  | `name` | `string` | 家庭名称 |
  | `location` | `string?` | 位置 |
  | `count` | `int?` | 设备数量 |
  | `role` | `int?` | 用户角色 |
  | `create_time` | `int?` | 创建时间（Unix 秒） |

---

### 30. 家庭设备列表
- **Method**: `GET`
- **Path**: `/v1/device/homeDevices`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
- **返回值** (`data`): `Array<Device>`（结构同 #17 设备列表）

---

### 31. 家庭用户列表
- **Method**: `GET`
- **Path**: `/v1/device/homeUsers`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
- **返回值** (`data`): `Array<HomeUser>`
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `uid` | `string` | 用户 ID |
  | `username` | `string?` | 用户名 |
  | `avatar` | `string?` | 头像 URL |
  | `role` | `int?` | 角色 |
  | `accept` | `int?` | 是否接受邀请 |

---

### 32. 更新家庭信息
- **Method**: `POST`
- **Path**: `/v1/device/homeUpdate`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
  | `name` | `string` | ✅ | 新名称 |
  | `location` | `string` | ✅ | 位置 |
- **返回值**: `null`

---

### 33. 添加设备到家庭
- **Method**: `POST`
- **Path**: `/v1/device/homeAddDevice`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **行为说明**:
  - 第一版仅家庭 owner 可添加设备
  - 设备必须归属当前登录用户本人
  - 设备已在当前家庭时按成功处理，不重复插入
  - 设备已挂到其他家庭时返回失败
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值**: `null`

---

### 34. 切换设备家庭
- **Method**: `POST`
- **Path**: `/v1/device/homeChange`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **行为说明**:
  - 第一版仅支持本人设备切换家庭
  - 当前用户必须能看到设备当前所在家庭
  - 当前用户必须是目标家庭 owner
  - 目标家庭等于当前家庭时按成功处理
  - 设备未挂任何家庭时返回失败，不自动兼容为添加设备
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 目标家庭 ID |
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值**: `null`

---

### 35. 邀请分享家庭
- **Method**: `POST`
- **Path**: `/v1/device/homeShare`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **行为说明**:
  - 第一版仅家庭 owner 可邀请
  - 不允许邀请自己
  - 目标用户已在家庭中时返回失败
  - 若已存在待处理邀请，返回失败
  - 成功后会创建一条待处理邀请记录，供后续 `homeShareFeedback` 使用
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
  | `username` | `string` | ✅ | 被邀请者账号 |
- **返回值**: `null`

---

### 36. 家庭分享反馈
- **Method**: `POST`
- **Path**: `/v1/device/homeShareFeedback`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **行为说明**:
  - `msg_id` 来源于 `GET /v1/message/list`
  - 仅消息接收方本人可反馈
  - 仅待处理消息可反馈
  - `accept=1` 表示同意并加入家庭；`accept=2` 表示拒绝
  - 反馈后，邀请发起人的消息列表会新增一条 `type=2` 家庭分享结果消息
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `msg_id` | `string` | ✅ | 消息 ID |
  | `accept` | `int` | ✅ | 反馈（1=同意, 2=拒绝） |
- **返回值**: `null`

---

### 37. 移除家庭成员
- **Method**: `POST`
- **Path**: `/v1/device/homeShareRemove`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **行为说明**:
  - 第一版仅家庭 owner 可移除成员
  - 不允许移除自己
  - 目标用户必须当前属于该家庭
  - 移除成功后，被移除成员会收到一条 `type=3` 的家庭移除通知消息
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
  | `uid` | `string` | ✅ | 被移除用户 UID |
- **返回值**: `null`

---

### 38. 删除家庭
- **Method**: `DELETE`
- **Path**: `/v1/device/homeDelete`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `home_id` | `string` | ✅ | 家庭 ID |
- **返回值**: `null`

---

## 五、设备事件模块 (`/v1/event/*`)

### 39. 事件列表
- **Method**: `GET`
- **Path**: `/v1/event/list`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `date` | `string` | ❌ | 日期筛选（yyyy-MM-dd） |
  | `uuid` | `string` | ❌ | 设备 UUID |
  | `home_id` | `string` | ❌ | 家庭 ID |
  | `type` | `string` | ❌ | 事件类型（多类型用 `,` 分隔） |
  | `start_time` | `int` | ❌ | 起始时间（分页用） |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `has` | `bool` | 是否还有更多 |
  | `list` | `Array<Event>` | 事件列表 |

  **Event 对象**:
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `id` | `string` | 事件 ID |
  | `uuid` | `string` | 设备 UUID |
  | `device_name` | `string?` | 设备名称 |
  | `type` | `int?` | 事件类型 |
  | `is_read` | `int?` | 是否已读（0=未读, 1=已读） |
  | `time` | `int?` | 时间（Unix 秒） |
  | `device_time` | `int?` | 设备时间（Unix 秒） |
  | `thumbnail` | `string?` | 缩略图 URL |
  | `payload` | `object?` | 事件载荷 |
  | `payload.result` | `int?` | 结果 |

---

### 40. 按月查询有事件的日期
- **Method**: `GET`
- **Path**: `/v1/event/existDay`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `month` | `string` | ✅ | 月份（yyyy-MM） |
  | `uuid` | `string` | ❌ | 设备 UUID |
  | `home_id` | `string` | ❌ | 家庭 ID |
- **返回值** (`data`): `Map<string, int>`（key 为日期，value 为事件数量）

---

### 41. 未读事件数量
- **Method**: `GET`
- **Path**: `/v1/event/unreadNum`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值**: TODO（无示例）

---

### 42. 标记事件已读
- **Method**: `POST`
- **Path**: `/v1/event/read`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `msg_id` | `string` | ✅ | 事件消息 ID |
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值**: `null`

---

### 43. 删除事件
- **Method**: `DELETE`
- **Path**: `/v1/event/delete`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `msg_id` | `string` | ✅ | 事件消息 ID |
  | `uuid` | `string` | ❌ | 设备 UUID |
- **返回值**: `null`

---

## 六、系统消息模块 (`/v1/message/*`)

### 44. 消息列表
- **Method**: `GET`
- **Path**: `/v1/message/list`
- **Auth**: ✅
- **参数方式**: Query
- **行为说明**:
  - 第一版返回家庭分享邀请消息（`type=1`）、家庭分享结果消息（`type=2`）与家庭移除通知消息（`type=3`）
  - 当前 `id` 可直接作为 `homeShareFeedback.msg_id`
  - 外层 `uid` 表示消息归属用户；`payload.uid` / `payload.username` 表示消息中的另一方用户
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `start_id` | `string` | ❌ | 分页起始 ID |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `has` | `bool` | 是否还有更多 |
  | `list` | `Array<Message>` | 消息列表 |

  **Message 对象**:
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `id` | `string` | 消息 ID |
  | `uid` | `string?` | 用户 ID |
  | `type` | `int` | 消息类型（1=家庭分享, 2=家庭分享结果, 3=移除家庭分享, 4=设备分享, 5=设备分享结果） |
  | `time` | `int` | 时间（Unix 秒） |
  | `is_read` | `int` | 是否已读（0=未读, 1=已读） |
  | `payload` | `object` | 消息载荷 |
  | `payload.home_id` | `string?` | 家庭 ID |
  | `payload.home_name` | `string?` | 家庭名称 |
  | `payload.status` | `int` | 状态（0=待处理, 1=同意, 2=拒绝） |
  | `payload.uid` | `string?` | 相关用户 ID |
  | `payload.username` | `string?` | 相关用户名 |
  | `payload.device_name` | `string?` | 设备名称 |
  | `payload.uuid` | `string?` | 设备 UUID |

---

### 45. 未读消息数量
- **Method**: `GET`
- **Path**: `/v1/message/unreadNum`
- **Auth**: ✅
- **参数方式**: 无参数
- **行为说明**:
  - 当前统计的是当前登录用户消息盒中的未读数量
  - 第一版统计 `type=1` 家庭分享邀请消息、`type=2` 家庭分享结果消息与 `type=3` 家庭移除通知消息
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `number` | `int` | 未读消息数量 |

---

### 46. 标记消息已读
- **Method**: `POST`
- **Path**: `/v1/message/read`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **行为说明**:
  - `message_id` 不传时，标记当前登录用户全部消息已读
  - `message_id` 传入时，仅标记该条消息已读
  - 只允许操作当前登录用户自己的消息
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `message_id` | `string` | ❌ | 消息 ID（不传则标记全部已读） |
- **返回值**: `null`

---

### 47. 删除消息
- **Method**: `DELETE`
- **Path**: `/v1/message/delete`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `message_id` | `string` | ✅ | 消息 ID |
- **返回值**: `null`

---

## 七、云存储模块 (`/v1/cloud/*`)

### 48. 获取 OSS Token
- **Method**: `GET`
- **Path**: `/v1/cloud/getToken`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `access_token_id` | `string` | AccessKey ID |
  | `access_key_secret` | `string` | AccessKey Secret |
  | `security_token` | `string` | 安全令牌 |
  | `expiration` | `int` | 过期时间（Unix 秒） |
  | `region_id` | `string?` | 区域 ID |
  | `endpoint` | `string` | OSS Endpoint |
  | `bucket` | `string` | Bucket 名称 |

---

### 49. 获取云端文件列表
- **Method**: `GET`
- **Path**: `/v1/cloud/files`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ✅ | 设备 UUID |
  | `date` | `string` | ✅ | 日期（yyyyMMdd） |
  | `zone` | `string` | ❌ | 时区偏移 |
- **返回值** (`data`): `Array<CloudFile>`（DeviceRecord 结构，具体字段见 v1 模型）

---

### 50. 上传文件
- **Method**: `POST`
- **Path**: `/v1/cloud/putFile`
- **Auth**: ❌
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `date` | `int` | ✅ | 日期（Unix 时间戳） |
  | `suffix` | `string` | ✅ | 文件后缀 |
  | `zone` | `double` | ✅ | 时区偏移 |
  | `store` | `int` | ✅ | 存储类型 |
  | `event_tag` | `string` | ✅ | 事件标签 |
  | `files` | `string` | ✅ | 文件数据 |
- **返回值**: TODO（无示例）

---

## 八、反馈模块 (`/v1/feedback/*`)

### 51. 创建反馈
- **Method**: `POST`
- **Path**: `/v1/feedback/create`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `reason` | `string` | ✅ | 反馈原因 |
  | `explain` | `string` | ✅ | 详细说明 |
- **返回值**: TODO（无示例）

---

### 52. 发送反馈消息
- **Method**: `POST`
- **Path**: `/v1/feedback/send`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `feedback_id` | `string` | ✅ | 反馈记录 ID |
  | `message` | `string` | ✅ | 消息内容 |
  | `file` | `string` | ✅ | 附件 |
- **返回值**: TODO（无示例）

---

### 53. 反馈列表
- **Method**: `GET`
- **Path**: `/v1/feedback/list`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `start_id` | `string` | ❌ | 分页起始 ID |
- **返回值**: TODO（无示例）

---

### 54. 反馈详情
- **Method**: `GET`
- **Path**: `/v1/feedback/info`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `feedback_id` | `string` | ✅ | 反馈记录 ID |
- **返回值**: TODO（无示例）

---

## 九、留言模块 (`/v1/leaveword/*`)

### 55. APP 留言列表
- **Method**: `GET`
- **Path**: `/v1/leaveword/list`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `uuid` | `string` | ❌ | 设备 UUID |
  | `rows` | `string` | ❌ | 最大条数 |
  | `start_time` | `int` | ❌ | 起始时间（分页用） |
- **返回值**: TODO（无示例）

---

### 56. APP 获取留言未读数量
- **Method**: `GET`
- **Path**: `/v1/leaveword/unreadNumber`
- **Auth**: ✅
- **参数方式**: 无参数
- **返回值**: TODO（无示例）

---

### 57. APP 留言设置已读
- **Method**: `POST`
- **Path**: `/v1/leaveword/read`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `id` | `string` | ✅ | 留言 ID |
- **返回值**: `null`

---

### 58. 设备获取未读留言数量
- **Method**: `GET`
- **Path**: `/v1/leaveword/deviceUnreadNumber`
- **Auth**: ❌
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `user_id` | `int` | ✅ | 用户 ID |
- **返回值**: TODO（无示例）

> [!NOTE]
> 设备端留言接口（#58–#60）在 APP 代码中**未使用**，仅存在于 Postman Collection。

---

### 59. 设备读取留言
- **Method**: `POST`
- **Path**: `/v1/leaveword/deviceRead`
- **Auth**: ❌
- **参数方式**: Query + Body (JSON)
- **请求参数（Query）**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `12313` | `int` | ✅ | 占位参数 |
  | `adajh` | `int` | ✅ | 占位参数 |
- **请求参数（Body）**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `a` | `int` | ✅ | 占位参数 |
  | `b` | `int` | ✅ | 占位参数 |
- **返回值**: TODO（无示例）

> [!WARNING]
> 此接口参数为 Postman 占位数据，非真实字段定义。

---

### 60. 设备获取留言列表
- **Method**: `GET`
- **Path**: `/v1/leaveword/deviceList`
- **Auth**: ❌
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `user_id` | `int` | ✅ | 用户 ID |
  | `is_read` | `int` | ✅ | 是否已读（0/1） |
- **返回值**: TODO（无示例）

---

## 十、售后服务模块 (`/v1/afterSales/*`)

### 61. 预约安装
- **Method**: `POST`
- **Path**: `/v1/afterSales/installReservation`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `sn` | `string` | ✅ | 设备 SN 码 |
  | `contact` | `string` | ✅ | 联系人 |
  | `phone_number` | `string` | ✅ | 联系电话 |
  | `scheduled_time` | `int` | ✅ | 预约时间（Unix 时间戳） |
  | `province` | `string` | ✅ | 省 |
  | `city` | `string` | ✅ | 市 |
  | `district` | `string` | ✅ | 区 |
  | `detailed_location` | `string` | ✅ | 详细地址 |
- **返回值**: `null`

---

### 62. 取消预约
- **Method**: `POST`
- **Path**: `/v1/afterSales/installCancel`
- **Auth**: ✅
- **参数方式**: Body (JSON)
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `id` | `string` | ✅ | 预约记录 ID |
- **返回值**: `null`

---

### 63. 查询预约历史记录
- **Method**: `GET`
- **Path**: `/v1/afterSales/installList`
- **Auth**: ✅
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `rows` | `string` | ❌ | 最大条数（默认 10） |
  | `direction` | `string` | ❌ | 查询方向：`"previous"` / `"later"`（默认 `"previous"`） |
  | `start_id` | `string` | ❌ | 分页起始 ID |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `has` | `bool` | 是否还有更多 |
  | `list` | `Array<Record>` | 记录列表 |

  **Record 对象**:
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `id` | `string` | 预约记录 ID |
  | `sn` | `string` | 设备 SN 码 |
  | `uuid` | `string` | 设备 UUID |
  | `uid` | `string` | 用户 ID |
  | `appid` | `string` | 应用 ID |
  | `status` | `int` | 状态（0=预约中, 1=已完成, 2=已取消） |
  | `contact` | `string` | 联系人 |
  | `phone_number` | `string` | 联系电话 |
  | `create_time` | `int` | 创建时间（Unix 秒） |
  | `scheduled_time` | `int` | 预约时间（Unix 秒） |
  | `cancel_time` | `int` | 取消时间（0=未取消） |
  | `area` | `object` | 地址信息 |
  | `area.province` | `string` | 省 |
  | `area.city` | `string` | 市 |
  | `area.district` | `string` | 区 |
  | `area.detailed_location` | `string` | 详细地址 |

---

## 十一、系统接口

### 64. 获取服务器时间戳
- **Method**: `GET`
- **Path**: `/time`
- **Auth**: ❌
- **参数方式**: 无参数
- **返回值**: TODO（无示例）

---

### 65. 获取服务器地址
- **Method**: `GET`
- **Path**: `/getUrl`
- **Auth**: ❌
- **参数方式**: Query
- **请求参数**:
  | 字段 | 类型 | 必填 | 说明 |
  |------|------|------|------|
  | `username` | `string` | ✅ | 用户账号 |
- **返回值** (`data`):
  | 字段 | 类型 | 说明 |
  |------|------|------|
  | `exist` | `bool` | 用户是否存在 |
  | `url` | `object` | 服务器地址 |
  | `url.api` | `string` | API 服务地址 |
  | `url.mqtt` | `string` | MQTT 服务地址 |
  | `url.websocket` | `string` | WebSocket 服务地址 |

---

## 统计摘要

| 分类 | 接口数量 | 需要 Auth | 免 Auth |
|------|---------|----------|--------|
| 用户模块 | 16 | 8 | 8 |
| 设备管理 | 7 | 6 | 1 |
| 设备分享 | 4 | 4 | 0 |
| 家庭模块 | 11 | 11 | 0 |
| 事件模块 | 5 | 5 | 0 |
| 消息模块 | 4 | 4 | 0 |
| 云存储 | 3 | 2 | 1 |
| 反馈模块 | 4 | 4 | 0 |
| 留言模块（APP） | 3 | 3 | 0 |
| 留言模块（设备端） | 3 | 0 | 3 |
| 售后服务 | 3 | 3 | 0 |
| 系统接口 | 2 | 0 | 2 |
| **总计** | **65** | **50** | **15** |
