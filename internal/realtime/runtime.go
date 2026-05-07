package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	devicemodel "has-smartlock-service/internal/device/model"
	devicerepo "has-smartlock-service/internal/device/repository"
	homerepo "has-smartlock-service/internal/home/repository"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/redisx"
	eventstore "has-smartlock-service/internal/realtime/eventstore"
	mqttpkg "has-smartlock-service/internal/realtime/mqtt"
	"has-smartlock-service/internal/realtime/shadow"
	wspkg "has-smartlock-service/internal/realtime/ws"
	usermodel "has-smartlock-service/internal/user/model"
	userrepo "has-smartlock-service/internal/user/repository"
)

const (
	wsVersion           = "1.0"
	wakeUpTTL           = 30 * time.Second
	commandCacheTTL     = time.Hour
	defaultFunctionCode = 5000
)

type Runtime struct {
	deviceRepo     *devicerepo.Repository
	homeRepo       *homerepo.Repository
	userRepo       *userrepo.Repository
	redisClient    *redisx.Client
	shadowStore    shadow.Store
	eventStore     eventstore.Store
	mqttClient     publisher
	hub            *wspkg.Hub
	deviceUpgrades []config.DeviceUpgradeConfig
	methods        map[string]wsMethodSpec
	webrtcQueue    chan asyncWebrtcSignal
}

type publisher interface {
	Publish(ctx context.Context, topic string, payload any) error
}

type wsMethodSpec struct {
	Name           string
	RequiresMaster bool
	Delayed        bool
	PhoneCodeOnly  bool
	Handler        func(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error)
}

type deviceContext struct {
	User     *usermodel.User
	Device   *devicemodel.Device
	Visible  *devicerepo.VisibleDevice
	Model    string
	IsMaster bool
}

type mqttRequest struct {
	MsgID string         `json:"msg_id"`
	Time  int64          `json:"time"`
	Data  map[string]any `json:"data"`
}

type mqttResponse struct {
	MsgID  string         `json:"msg_id"`
	Time   int64          `json:"time"`
	Result int            `json:"result"`
	Msg    string         `json:"msg,omitempty"`
	Code   int            `json:"code,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

type funcResponse struct {
	MsgID   string         `json:"msg_id"`
	Time    int64          `json:"time"`
	Result  int            `json:"result"`
	ErrCode int64          `json:"err_code"`
	Status  int            `json:"status"`
	Data    map[string]any `json:"data"`
}

type commandContext struct {
	UID       string         `json:"uid"`
	PhoneCode string         `json:"phone_code"`
	UUID      string         `json:"uuid"`
	Method    string         `json:"method"`
	Data      map[string]any `json:"data"`
}

type asyncWebrtcSignal struct {
	Session wspkg.Session
	Request wspkg.Request
}

func NewRuntime(deviceRepo *devicerepo.Repository, homeRepo *homerepo.Repository, userRepo *userrepo.Repository, redisClient *redisx.Client, shadowStore shadow.Store, eventStore eventstore.Store, cfg ...config.Config) *Runtime {
	r := &Runtime{
		deviceRepo:  deviceRepo,
		homeRepo:    homeRepo,
		userRepo:    userRepo,
		redisClient: redisClient,
		shadowStore: shadowStore,
		eventStore:  eventStore,
		webrtcQueue: make(chan asyncWebrtcSignal, 128),
	}
	if len(cfg) > 0 {
		r.deviceUpgrades = cfg[0].DeviceUpgrades
	}
	r.methods = r.buildMethodRegistry()
	go r.consumeWebrtcSignals()
	return r
}

func (r *Runtime) SetMQTTClient(client publisher) {
	r.mqttClient = client
}

func (r *Runtime) SetHub(hub *wspkg.Hub) {
	r.hub = hub
}

func (r *Runtime) HandleWS(ctx context.Context, session wspkg.Session, request wspkg.Request) *wspkg.Response {
	spec, ok := r.methods[strings.TrimSpace(request.Method)]
	if !ok {
		return r.errorResponse(request, 4000, "unsupported websocket method")
	}
	deviceCtx, err := r.loadDeviceContext(session, request.UUID, spec.RequiresMaster)
	if err != nil {
		switch {
		case errors.Is(err, errMasterOnly):
			return r.errorResponse(request, 4003, "device master required")
		case errors.Is(err, errVisibleDeviceNotFound):
			return r.errorResponse(request, 4001, "device not found")
		default:
			return r.errorResponse(request, 5000, err.Error())
		}
	}
	resp, err := spec.Handler(ctx, session, request, deviceCtx, spec)
	if err != nil {
		return r.errorResponse(request, 5000, err.Error())
	}
	return resp
}

func (r *Runtime) HandleAttrSetResp(ctx context.Context, topic string, payload []byte) {
	_ = ctx
	_ = topic
	_ = payload
}

func (r *Runtime) HandleAttrPost(ctx context.Context, topic string, payload []byte) {
	if r.shadowStore == nil {
		return
	}
	model, uuid, _ := parseThingTopic(topic)
	var req mqttRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}
	device, err := r.deviceRepo.FindDeviceByUUID(uuid)
	if err != nil {
		return
	}
	reported := normalizeReportedValues(req.Data, req.Time)
	doc, err := r.shadowStore.UpdateReported(ctx, uuid, device.UID, req.Time, reported)
	if err != nil {
		return
	}
	r.NotifyAttributeChange(ctx, uuid, model, doc.State.Reported)
	_ = r.publishAck(ctx, fmt.Sprintf("/thing/%s/%s/attr/post/resp", model, uuid), req.MsgID, true, 0, nil)
}

func (r *Runtime) HandleAttrGet(ctx context.Context, topic string, payload []byte) {
	if r.shadowStore == nil || r.mqttClient == nil {
		return
	}
	model, uuid, _ := parseThingTopic(topic)
	var req mqttRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}
	doc, err := r.shadowStore.Get(ctx, uuid)
	if err != nil {
		return
	}
	_ = r.mqttClient.Publish(ctx, fmt.Sprintf("/thing/%s/%s/attr/get/resp", model, uuid), mqttResponse{
		MsgID:  req.MsgID,
		Time:   time.Now().Unix(),
		Result: 1,
		Msg:    "ok",
		Data: map[string]any{
			"desired":  doc.State.Desired,
			"reported": doc.State.Reported,
			"metadata": doc.Metadata,
			"version":  doc.Version,
		},
	})
}

func (r *Runtime) HandleEvent(ctx context.Context, topic string, payload []byte) {
	model, uuid, eventName := parseThingTopic(topic)
	var req mqttRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}
	device, err := r.deviceRepo.FindDeviceByUUID(uuid)
	if err != nil {
		return
	}

	switch eventName {
	case "Ring":
		r.NotifyRing(ctx, uuid, device.Name, "Ring", req.Data)
	case "RingStop":
		r.NotifyRing(ctx, uuid, device.Name, "RingStop", req.Data)
	case "ChangeCamResp":
		r.notifyFuncResponseFromEvent(ctx, uuid, "ChangeCam", req)
	case "WebrtcSignal":
		r.notifyWebrtcEvent(ctx, uuid, req)
	}

	if isShadowOnlyEvent(eventName) && r.shadowStore != nil {
		values := map[string]map[string]any{
			eventName: {
				"timestamp": req.Time,
				"value":     req.Data,
			},
		}
		if doc, err := r.shadowStore.UpdateReported(ctx, uuid, device.UID, req.Time, values); err == nil {
			r.NotifyAttributeChange(ctx, uuid, model, doc.State.Reported)
		}
		_ = r.publishAck(ctx, fmt.Sprintf("/thing/%s/%s/event/%s/resp", model, uuid, eventName), req.MsgID, true, 200, nil)
		return
	}

	if shouldPersistEvent(eventName) && r.eventStore != nil {
		homeID := ""
		belong := r.collectEventBelongTo(device)
		if link, err := r.homeRepo.FindActiveHomeDeviceByInternalDeviceID(device.ID); err == nil {
			homeID = strconv.FormatUint(uint64(link.HomeID), 10)
		}
		_ = r.eventStore.Create(ctx, eventstore.Event{
			ID:         req.MsgID,
			UUID:       uuid,
			HomeID:     homeID,
			DeviceName: device.Name,
			Type:       mapEventType(eventName),
			Time:       extractEventTime(req.Data, req.Time),
			DeviceTime: extractEventTime(req.Data, req.Time),
			Thumbnail:  stringValue(req.Data["thumbnail"]),
			Payload:    req.Data,
			BelongTo:   belong,
			ExpireAt:   time.Now().Add(7 * 24 * time.Hour),
		})
	}
	if eventName != "ChangeCamResp" && eventName != "WebrtcSignal" {
		r.broadcast(uuid, eventName, map[string]any{
			"uuid":        uuid,
			"type":        eventName,
			"device_name": device.Name,
			"data":        req.Data,
		})
	}
	_ = r.publishAck(ctx, fmt.Sprintf("/thing/%s/%s/event/%s/resp", model, uuid, eventName), req.MsgID, true, 200, nil)
}

func (r *Runtime) HandleFuncResp(ctx context.Context, topic string, payload []byte) {
	if r.redisClient == nil || r.hub == nil {
		return
	}
	_, uuid, funcName := parseThingTopic(topic)
	var resp funcResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return
	}
	var cache commandContext
	ok, err := r.redisClient.LoadJSON(ctx, commandCacheKey(funcName, resp.MsgID), &cache)
	if err != nil || !ok {
		return
	}
	_ = r.redisClient.DeleteKey(ctx, commandCacheKey(funcName, resp.MsgID))

	data := decodeFuncResponse(funcName, resp)
	methodName := "ServerFunc." + funcName
	code := statusCode(resp.Result, resp.ErrCode)
	if isPhoneCodeMethod(funcName) {
		_ = r.hub.Notify(cache.UID, cache.PhoneCode, wspkg.Response{
			MsgID:   resp.MsgID,
			Method:  methodName,
			UUID:    uuid,
			Code:    code,
			Msg:     responseMsg(code),
			Time:    time.Now().Unix(),
			Version: wsVersion,
			Data:    data,
		})
		return
	}
	r.hub.Broadcast(cache.UID, wspkg.Response{
		MsgID:   resp.MsgID,
		Method:  methodName,
		UUID:    uuid,
		Code:    code,
		Msg:     responseMsg(code),
		Time:    time.Now().Unix(),
		Version: wsVersion,
		Data:    data,
	})
}

func (r *Runtime) HandleBrokerConnected(ctx context.Context, topic string, payload []byte) {
	_ = topic
	r.handleBrokerStatus(ctx, payload, true)
}

func (r *Runtime) HandleBrokerDisconnected(ctx context.Context, topic string, payload []byte) {
	_ = topic
	r.handleBrokerStatus(ctx, payload, false)
}

func (r *Runtime) NotifyDeviceBind(ctx context.Context, uid, uuid string) {
	r.pushUser(uid, "", true, wspkg.Response{
		Method:  "DeviceBind",
		UUID:    uuid,
		Code:    0,
		Msg:     "ok",
		Time:    time.Now().Unix(),
		Version: wsVersion,
		Data: map[string]any{
			"uuid":   uuid,
			"result": 1,
		},
	})
	r.NotifyDevicesChanged(ctx, uid)
}

func (r *Runtime) NotifyDeviceUnBind(ctx context.Context, uid, uuid string) {
	r.pushUser(uid, "", true, wspkg.Response{
		Method:  "DeviceUnBind",
		UUID:    uuid,
		Code:    0,
		Msg:     "ok",
		Time:    time.Now().Unix(),
		Version: wsVersion,
		Data: map[string]any{
			"uuid":   uuid,
			"result": 1,
		},
	})
	r.NotifyDevicesChanged(ctx, uid)
}

func (r *Runtime) NotifyDevicesChanged(_ context.Context, uid string) {
	r.pushUser(uid, "", true, wspkg.Response{
		Method:  "DevicesChanged",
		Code:    0,
		Msg:     "ok",
		Time:    time.Now().Unix(),
		Version: wsVersion,
	})
}

func (r *Runtime) NotifyAttributeChange(_ context.Context, uuid, model string, attrs map[string]any) {
	data := map[string]any{
		"uuid":  uuid,
		"model": model,
	}
	for key, value := range attrs {
		data[key] = value
	}
	r.broadcast(uuid, "AttributeChange", data)
}

func (r *Runtime) NotifyRing(_ context.Context, uuid, deviceName, eventName string, payload map[string]any) {
	data := map[string]any{
		"uuid":        uuid,
		"type":        eventName,
		"device_name": deviceName,
	}
	for key, value := range payload {
		data[key] = value
	}
	r.broadcast(uuid, eventName, data)
}

func (r *Runtime) buildMethodRegistry() map[string]wsMethodSpec {
	immediateFunc := func(name string, requiresMaster bool) wsMethodSpec {
		return wsMethodSpec{Name: name, RequiresMaster: requiresMaster, Handler: r.handleImmediateFunction}
	}
	delayedFunc := func(name string, requiresMaster, phoneCodeOnly bool) wsMethodSpec {
		return wsMethodSpec{Name: name, RequiresMaster: requiresMaster, Delayed: true, PhoneCodeOnly: phoneCodeOnly, Handler: r.handleDelayedFunction}
	}
	return map[string]wsMethodSpec{
		"SetAttribute":       {Name: "SetAttribute", Handler: r.handleSetAttribute},
		"GetAttribute":       {Name: "GetAttribute", Handler: r.handleGetAttribute},
		"SetNotifyStatus":    {Name: "SetNotifyStatus", Handler: r.handleSetNotifyStatus},
		"GetNotifyStatus":    {Name: "GetNotifyStatus", Handler: r.handleGetNotifyStatus},
		"SleepState":         {Name: "SleepState", Handler: r.handleSleepState},
		"Awake":              {Name: "Awake", Handler: r.handleAwake},
		"CreateUser":         delayedFunc("CreateUser", true, false),
		"UpdateUser":         delayedFunc("UpdateUser", true, false),
		"DelUser":            delayedFunc("DelUser", true, false),
		"AddPwd":             delayedFunc("AddPwd", true, false),
		"UpdatePwd":          delayedFunc("UpdatePwd", true, false),
		"DelPwd":             delayedFunc("DelPwd", true, false),
		"Lock":               immediateFunc("Lock", false),
		"Unlock":             immediateFunc("Unlock", false),
		"SetBluetoothSwitch": immediateFunc("SetBluetoothSwitch", false),
		"DelBluetoothSwitch": immediateFunc("DelBluetoothSwitch", false),
		"OtaUpgrade":         {Name: "OtaUpgrade", Handler: r.handleOtaUpgrade},
		"Reboot":             immediateFunc("Reboot", true),
		"ResetSetting":       immediateFunc("ResetSetting", true),
		"SetBackgroundImage": delayedFunc("SetBackgroundImage", false, true),
		"Call":               immediateFunc("Call", false),
		"ChangeCam":          delayedFunc("ChangeCam", false, true),
		"OthersAnswer":       {Name: "OthersAnswer", Handler: r.handleOthersAnswer},
		"LeaveWord":          {Name: "LeaveWord", Handler: r.handleStubMethod},
		"WebrtcSignal":       {Name: "WebrtcSignal", Handler: r.handleWebrtcSignal},
		"SetP2pState":        {Name: "SetP2pState", Handler: r.handleStubMethod},
		"QuickReplyFile":     {Name: "QuickReplyFile", Handler: r.handleStubMethod},
	}
}

func (r *Runtime) handleSetAttribute(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	if r.shadowStore != nil {
		if _, err := r.shadowStore.UpdateDesired(ctx, request.UUID, deviceCtx.Device.UID, request.Time, request.Data); err != nil {
			return nil, err
		}
	}
	if err := r.publishAttribute(ctx, deviceCtx.Model, request, request.Data); err != nil {
		return nil, err
	}
	r.NotifyAttributeChange(ctx, request.UUID, deviceCtx.Model, request.Data)
	return r.okResponse(request, request.Data), nil
}

func (r *Runtime) handleGetAttribute(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	if r.shadowStore == nil {
		return r.okResponse(request, map[string]any{
			"desired":  map[string]any{},
			"reported": map[string]any{},
			"metadata": map[string]any{},
			"version":  0,
		}), nil
	}
	doc, err := r.shadowStore.Get(ctx, request.UUID)
	if err != nil {
		return nil, err
	}
	keys := extractAttributeKeys(request.Data)
	desired, reported, desiredMeta, reportedMeta := filterShadowDocument(doc, keys)
	return r.okResponse(request, map[string]any{
		"desired":  desired,
		"reported": reported,
		"metadata": map[string]any{
			"desired":  desiredMeta,
			"reported": reportedMeta,
		},
		"version": doc.Version,
	}), nil
}

func (r *Runtime) handleSetNotifyStatus(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	payload := map[string]any{"NotifyStatus": request.Data}
	if r.shadowStore != nil {
		if _, err := r.shadowStore.UpdateDesired(ctx, request.UUID, deviceCtx.Device.UID, request.Time, payload); err != nil {
			return nil, err
		}
	}
	if err := r.publishAttribute(ctx, deviceCtx.Model, request, payload); err != nil {
		return nil, err
	}
	return r.okResponse(request, request.Data), nil
}

func (r *Runtime) handleGetNotifyStatus(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	if r.shadowStore == nil {
		return r.okResponse(request, map[string]any{"NotifyStatus": map[string]any{}}), nil
	}
	doc, err := r.shadowStore.Get(ctx, request.UUID)
	if err != nil {
		return nil, err
	}
	return r.okResponse(request, map[string]any{
		"NotifyStatus": mapValue(doc.State.Desired["NotifyStatus"]),
	}), nil
}

func (r *Runtime) handleSleepState(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	value := request.Data["SleepState"]
	if value == nil {
		value = request.Data["status"]
	}
	payload := map[string]any{"SleepState": value}
	if r.shadowStore != nil {
		if _, err := r.shadowStore.UpdateDesired(ctx, request.UUID, deviceCtx.Device.UID, request.Time, payload); err != nil {
			return nil, err
		}
	}
	if err := r.publishAttribute(ctx, deviceCtx.Model, request, payload); err != nil {
		return nil, err
	}
	return r.okResponse(request, payload), nil
}

func (r *Runtime) handleAwake(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	if r.redisClient != nil {
		_ = r.redisClient.StoreJSON(ctx, "wake_up:"+request.UUID, map[string]any{
			"uid":  session.UID,
			"time": time.Now().Unix(),
		}, wakeUpTTL)
	}
	if err := r.publishFunction(ctx, deviceCtx.Model, request, "Awake", request.Data); err != nil {
		return nil, err
	}
	return r.okResponse(request, map[string]any{"accepted": 1}), nil
}

func (r *Runtime) handleImmediateFunction(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	payload := cloneAnyMap(request.Data)
	if spec.Name == "Lock" || spec.Name == "Unlock" {
		payload["uid"] = deviceCtx.User.UID
		payload["username"] = deviceCtx.User.Username
	}
	if err := r.publishFunction(ctx, deviceCtx.Model, request, spec.Name, payload); err != nil {
		return nil, err
	}
	return r.okResponse(request, payload), nil
}

func (r *Runtime) handleDelayedFunction(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	if r.redisClient != nil {
		_ = r.redisClient.StoreJSON(ctx, commandCacheKey(spec.Name, request.MsgID), commandContext{
			UID:       session.UID,
			PhoneCode: session.PhoneCode,
			UUID:      request.UUID,
			Method:    spec.Name,
			Data:      request.Data,
		}, commandCacheTTL)
	}
	if err := r.publishFunction(ctx, deviceCtx.Model, request, spec.Name, request.Data); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *Runtime) handleOtaUpgrade(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	payload := cloneAnyMap(request.Data)
	if stringValue(payload["version"]) == "" {
		payload["version"] = r.matchUpgradeVersion(deviceCtx.Device, stringValue(payload["flag"]))
	}
	if stringValue(payload["version"]) == "" {
		payload["version"] = deviceCtx.Device.CurrentVersion
	}
	if stringValue(payload["url"]) == "" {
		payload["url"] = "controlled://ota/" + deviceCtx.Model + "/" + stringValue(payload["version"])
	}
	if err := r.publishFunction(ctx, deviceCtx.Model, request, spec.Name, payload); err != nil {
		return nil, err
	}
	return r.okResponse(request, payload), nil
}

func (r *Runtime) handleOthersAnswer(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	if err := r.publishFunction(ctx, deviceCtx.Model, request, spec.Name, request.Data); err != nil {
		return nil, err
	}
	r.NotifyRing(ctx, request.UUID, deviceCtx.Device.Name, "RingStop", map[string]any{
		"uuid": request.UUID,
	})
	return r.okResponse(request, map[string]any{"accepted": 1}), nil
}

func (r *Runtime) handleWebrtcSignal(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	select {
	case r.webrtcQueue <- asyncWebrtcSignal{Session: session, Request: request}:
	default:
		go r.notifyWebrtcSignal(session, request)
	}
	return r.okResponse(request, map[string]any{"queued": 1, "stub": 1}), nil
}

func (r *Runtime) handleStubMethod(ctx context.Context, session wspkg.Session, request wspkg.Request, deviceCtx deviceContext, spec wsMethodSpec) (*wspkg.Response, error) {
	return r.okResponse(request, map[string]any{
		"stub":     1,
		"accepted": 1,
		"method":   spec.Name,
	}), nil
}

func (r *Runtime) publishAttribute(ctx context.Context, model string, request wspkg.Request, payload map[string]any) error {
	if r.mqttClient == nil {
		return fmt.Errorf("mqtt not configured")
	}
	return r.mqttClient.Publish(ctx, fmt.Sprintf("/thing/%s/%s/attr/set", model, request.UUID), mqttRequest{
		MsgID: request.MsgID,
		Time:  time.Now().Unix(),
		Data:  payload,
	})
}

func (r *Runtime) publishFunction(ctx context.Context, model string, request wspkg.Request, method string, payload map[string]any) error {
	if r.mqttClient == nil {
		return fmt.Errorf("mqtt not configured")
	}
	return r.mqttClient.Publish(ctx, fmt.Sprintf("/thing/%s/%s/func/%s", model, request.UUID, method), mqttRequest{
		MsgID: request.MsgID,
		Time:  time.Now().Unix(),
		Data:  payload,
	})
}

func (r *Runtime) loadDeviceContext(session wspkg.Session, uuid string, requiresMaster bool) (deviceContext, error) {
	user, err := r.userRepo.FindUserByUID(session.UID)
	if err != nil {
		return deviceContext{}, errVisibleDeviceNotFound
	}
	visible, err := r.deviceRepo.FindVisibleDeviceByUUID(user.ID, user.UID, strings.TrimSpace(uuid))
	if err != nil {
		return deviceContext{}, errVisibleDeviceNotFound
	}
	device, err := r.deviceRepo.FindDeviceByUUID(strings.TrimSpace(uuid))
	if err != nil {
		return deviceContext{}, errVisibleDeviceNotFound
	}
	ctx := deviceContext{
		User:    user,
		Device:  device,
		Visible: visible,
		Model:   strings.TrimSpace(device.ModelCode),
	}
	if ctx.Model == "" {
		ctx.Model = "unknown"
	}
	ctx.IsMaster = device.UID == user.UID
	if !ctx.IsMaster && r.redisClient != nil {
		if bindings, err := r.redisClient.DeviceBindings(context.Background(), device.UUID); err == nil {
			if bindings[user.UID] == "1" {
				ctx.IsMaster = true
			}
		}
	}
	if requiresMaster && !ctx.IsMaster {
		return deviceContext{}, errMasterOnly
	}
	return ctx, nil
}

func (r *Runtime) handleBrokerStatus(ctx context.Context, payload []byte, online bool) {
	var message struct {
		Username string `json:"username"`
		IP       string `json:"ipaddress"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return
	}
	if message.Username == "" || message.Username == "backend_service" {
		return
	}
	device, err := r.deviceRepo.FindDeviceByUUID(message.Username)
	if err != nil {
		return
	}
	ipValue := int64(0)
	if online {
		ipValue = ipToInt64(message.IP)
	}
	now := time.Now().Unix()
	_ = r.deviceRepo.UpdateDeviceByID(device.ID, map[string]any{
		"online":      boolToInt(online),
		"online_ip":   ipValue,
		"update_time": now,
		"active_time": now,
	})
	if r.shadowStore != nil {
		if doc, err := r.shadowStore.UpdateReported(ctx, device.UUID, device.UID, now*1000, map[string]map[string]any{
			"Online": {
				"timestamp": now * 1000,
				"value":     boolToInt(online),
			},
		}); err == nil {
			if !online && (message.Reason == "discarded" || message.Reason == "tcp_closed") {
				return
			}
			r.NotifyAttributeChange(ctx, device.UUID, strings.TrimSpace(device.ModelCode), map[string]any{
				"Online": doc.State.Reported["Online"],
			})
		}
	}
}

func (r *Runtime) publishAck(ctx context.Context, topic, msgID string, success bool, code int, data map[string]any) error {
	if r.mqttClient == nil {
		return nil
	}
	result := 0
	if success {
		result = 1
	}
	return r.mqttClient.Publish(ctx, topic, mqttResponse{
		MsgID:  msgID,
		Time:   time.Now().Unix(),
		Result: result,
		Code:   code,
		Msg:    "ok",
		Data:   data,
	})
}

func (r *Runtime) broadcast(uuid, method string, data map[string]any) {
	if r.hub == nil {
		return
	}
	device, err := r.deviceRepo.FindDeviceByUUID(uuid)
	if err != nil {
		return
	}
	sent := map[string]struct{}{}
	push := func(uid string) {
		if uid == "" {
			return
		}
		if _, ok := sent[uid]; ok {
			return
		}
		sent[uid] = struct{}{}
		r.hub.Broadcast(uid, wspkg.Response{
			Method:  method,
			UUID:    uuid,
			Code:    0,
			Msg:     "ok",
			Time:    time.Now().Unix(),
			Version: wsVersion,
			Data:    data,
		})
	}
	push(device.UID)
	if link, err := r.homeRepo.FindActiveHomeDeviceByInternalDeviceID(device.ID); err == nil {
		if members, err := r.homeRepo.ListHomeMembersByInternalHomeID(link.HomeID); err == nil {
			for _, member := range members {
				push(member.User.UID)
			}
		}
	}
	if shared, err := r.deviceRepo.ListActiveDeviceShareMembersByInternalDeviceID(device.ID); err == nil {
		for _, member := range shared {
			push(member.User.UID)
		}
	}
}

func (r *Runtime) pushUser(uid, phoneCode string, broadcast bool, response wspkg.Response) {
	if r.hub == nil || uid == "" {
		return
	}
	if broadcast {
		r.hub.Broadcast(uid, response)
		return
	}
	_ = r.hub.Notify(uid, phoneCode, response)
}

func (r *Runtime) okResponse(request wspkg.Request, data map[string]any) *wspkg.Response {
	return &wspkg.Response{
		MsgID:   request.MsgID,
		Method:  request.Method,
		UUID:    request.UUID,
		Time:    time.Now().Unix(),
		Code:    0,
		Msg:     "ok",
		Version: request.Version,
		Data:    data,
	}
}

func (r *Runtime) errorResponse(request wspkg.Request, code int, msg string) *wspkg.Response {
	return &wspkg.Response{
		MsgID:   request.MsgID,
		Method:  request.Method,
		UUID:    request.UUID,
		Time:    time.Now().Unix(),
		Code:    code,
		Msg:     msg,
		Version: request.Version,
	}
}

func (r *Runtime) collectEventBelongTo(device *devicemodel.Device) []eventstore.BelongTo {
	belong := []eventstore.BelongTo{{UID: device.UID, IsRead: 0}}
	seen := map[string]struct{}{device.UID: {}}
	if link, err := r.homeRepo.FindActiveHomeDeviceByInternalDeviceID(device.ID); err == nil {
		if members, err := r.homeRepo.ListHomeMembersByInternalHomeID(link.HomeID); err == nil {
			for _, member := range members {
				if _, ok := seen[member.User.UID]; ok {
					continue
				}
				seen[member.User.UID] = struct{}{}
				belong = append(belong, eventstore.BelongTo{UID: member.User.UID, IsRead: 0})
			}
		}
	}
	if shared, err := r.deviceRepo.ListActiveDeviceShareMembersByInternalDeviceID(device.ID); err == nil {
		for _, member := range shared {
			if _, ok := seen[member.User.UID]; ok {
				continue
			}
			seen[member.User.UID] = struct{}{}
			belong = append(belong, eventstore.BelongTo{UID: member.User.UID, IsRead: 0})
		}
	}
	return belong
}

func (r *Runtime) notifyFuncResponseFromEvent(ctx context.Context, uuid, method string, req mqttRequest) {
	if r.redisClient == nil || r.hub == nil {
		return
	}
	var cache commandContext
	ok, err := r.redisClient.LoadJSON(ctx, commandCacheKey(method, req.MsgID), &cache)
	if err != nil || !ok {
		return
	}
	_ = r.redisClient.DeleteKey(ctx, commandCacheKey(method, req.MsgID))
	_ = r.hub.Notify(cache.UID, cache.PhoneCode, wspkg.Response{
		MsgID:   req.MsgID,
		Method:  "ServerFunc." + method,
		UUID:    uuid,
		Code:    0,
		Msg:     "ok",
		Time:    time.Now().Unix(),
		Version: wsVersion,
		Data:    req.Data,
	})
}

func (r *Runtime) notifyWebrtcEvent(ctx context.Context, uuid string, req mqttRequest) {
	_ = ctx
	if r.hub == nil {
		return
	}
	sendTo := stringValue(req.Data["send_to"])
	phoneCode := stringValue(req.Data["phone_code"])
	response := wspkg.Response{
		MsgID:   req.MsgID,
		Method:  "ServerFunc.WebrtcSignal",
		UUID:    uuid,
		Code:    0,
		Msg:     "ok",
		Time:    time.Now().Unix(),
		Version: wsVersion,
		Data:    req.Data,
	}
	if sendTo != "" && phoneCode != "" {
		_ = r.hub.Notify(sendTo, phoneCode, response)
		return
	}
	if sendTo != "" {
		r.hub.Broadcast(sendTo, response)
	}
}

func (r *Runtime) consumeWebrtcSignals() {
	for signal := range r.webrtcQueue {
		r.notifyWebrtcSignal(signal.Session, signal.Request)
	}
}

func (r *Runtime) notifyWebrtcSignal(session wspkg.Session, request wspkg.Request) {
	if r.hub == nil {
		return
	}
	data := cloneAnyMap(request.Data)
	data["stub"] = 1
	response := wspkg.Response{
		MsgID:   request.MsgID,
		Method:  "ServerFunc.WebrtcSignal",
		UUID:    request.UUID,
		Code:    0,
		Msg:     "ok",
		Time:    time.Now().Unix(),
		Version: request.Version,
		Data:    data,
	}
	sendTo := stringValue(request.Data["send_to"])
	if sendTo != "" {
		r.hub.Broadcast(sendTo, response)
		return
	}
	_ = r.hub.Notify(session.UID, session.PhoneCode, response)
}

func parseThingTopic(topic string) (model, uuid, name string) {
	parts := strings.Split(topic, "/")
	if len(parts) < 6 {
		return "", "", ""
	}
	return parts[2], parts[3], parts[5]
}

func commandCacheKey(method, msgID string) string {
	switch method {
	case "CreateUser":
		return "device_user_create:" + msgID
	case "UpdateUser":
		return "device_user_update:" + msgID
	case "DelUser":
		return "device_user_del:" + msgID
	case "AddPwd":
		return "device_user_pwd_create:" + msgID
	case "UpdatePwd":
		return "device_user_pwd_update:" + msgID
	case "DelPwd":
		return "device_user_pwd_del:" + msgID
	case "SetBackgroundImage":
		return "device_background_image:" + msgID
	case "ChangeCam":
		return "device_change_cam:" + msgID
	default:
		return "device_func:" + method + ":" + msgID
	}
}

func isPhoneCodeMethod(method string) bool {
	return method == "SetBackgroundImage" || method == "ChangeCam"
}

func isShadowOnlyEvent(eventName string) bool {
	switch eventName {
	case "SetUsers", "DelUser", "AddPwd", "DelPwd":
		return true
	default:
		return false
	}
}

func shouldPersistEvent(eventName string) bool {
	switch eventName {
	case "RingStop", "ChangeCamResp", "WebrtcSignal":
		return false
	default:
		return !isShadowOnlyEvent(eventName)
	}
}

func mapEventType(name string) int {
	switch name {
	case "OpenDoor":
		return 1
	case "Tamper":
		return 2
	case "Ring":
		return 3
	case "Pir":
		return 4
	case "Radar":
		return 5
	case "Stay":
		return 6
	case "OpenDoorFailedMax":
		return 7
	case "WaitingLockTimeout":
		return 8
	case "UpgradeResult":
		return 9
	case "UpgradeStart":
		return 10
	case "LowBattery":
		return 11
	case "OpenDoorFailed":
		return 12
	case "Conflagration":
		return 13
	default:
		return 0
	}
}

func normalizeReportedValues(data map[string]any, fallbackTimestamp int64) map[string]map[string]any {
	reported := make(map[string]map[string]any, len(data))
	for key, raw := range data {
		if payload, ok := raw.(map[string]any); ok {
			if _, ok := payload["value"]; ok {
				if _, ok := payload["timestamp"]; !ok {
					payload["timestamp"] = fallbackTimestamp
				}
				reported[key] = payload
				continue
			}
		}
		reported[key] = map[string]any{
			"timestamp": fallbackTimestamp,
			"value":     raw,
		}
	}
	return reported
}

func extractAttributeKeys(data map[string]any) []string {
	for _, field := range []string{"keys", "attributes"} {
		raw, ok := data[field]
		if !ok {
			continue
		}
		values, ok := raw.([]any)
		if !ok {
			continue
		}
		keys := make([]string, 0, len(values))
		for _, value := range values {
			text := strings.TrimSpace(stringValue(value))
			if text != "" {
				keys = append(keys, text)
			}
		}
		return keys
	}
	return nil
}

func filterShadowDocument(doc *shadow.Document, keys []string) (map[string]any, map[string]any, map[string]shadow.TimestampedValue, map[string]shadow.TimestampedValue) {
	if len(keys) == 0 {
		return cloneAnyMap(doc.State.Desired), cloneAnyMap(doc.State.Reported), cloneTimestampMap(doc.Metadata.Desired), cloneTimestampMap(doc.Metadata.Reported)
	}
	desired := make(map[string]any, len(keys))
	reported := make(map[string]any, len(keys))
	desiredMeta := make(map[string]shadow.TimestampedValue, len(keys))
	reportedMeta := make(map[string]shadow.TimestampedValue, len(keys))
	for _, key := range keys {
		if value, ok := doc.State.Desired[key]; ok {
			desired[key] = value
		}
		if value, ok := doc.State.Reported[key]; ok {
			reported[key] = value
		}
		if value, ok := doc.Metadata.Desired[key]; ok {
			desiredMeta[key] = value
		}
		if value, ok := doc.Metadata.Reported[key]; ok {
			reportedMeta[key] = value
		}
	}
	return desired, reported, desiredMeta, reportedMeta
}

func decodeFuncResponse(funcName string, resp funcResponse) map[string]any {
	data := cloneAnyMap(resp.Data)
	data["status"] = resp.Status
	data["err_code"] = resp.ErrCode
	if _, ok := data["id"]; !ok && (funcName == "CreateUser" || funcName == "UpdateUser" || funcName == "DelUser") {
		data["id"] = stringValue(resp.Data["id"])
	}
	if _, ok := data["enc"]; !ok && strings.Contains(funcName, "Pwd") {
		data["enc"] = stringValue(resp.Data["enc"])
	}
	if _, ok := data["data"]; !ok && strings.Contains(funcName, "Pwd") {
		data["data"] = stringValue(resp.Data["data"])
	}
	if _, ok := data["salt"]; !ok && strings.Contains(funcName, "Pwd") {
		data["salt"] = stringValue(resp.Data["salt"])
	}
	return data
}

func extractEventTime(data map[string]any, fallback int64) int64 {
	if value, ok := data["time"]; ok {
		switch v := value.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		}
	}
	return fallback
}

func statusCode(result int, errCode int64) int {
	if result == 1 && errCode == 0 {
		return 0
	}
	if errCode > 0 {
		return int(errCode)
	}
	return defaultFunctionCode
}

func responseMsg(code int) string {
	if code == 0 {
		return "ok"
	}
	return "failed"
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ipToInt64(ip string) int64 {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}
	var result int64
	for _, part := range parts {
		value, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return 0
		}
		result = result<<8 + value
	}
	return result
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return cloneAnyMap(typed)
	}
	return map[string]any{}
}

func cloneAnyMap(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneAnyMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func cloneTimestampMap(source map[string]shadow.TimestampedValue) map[string]shadow.TimestampedValue {
	if source == nil {
		return map[string]shadow.TimestampedValue{}
	}
	out := make(map[string]shadow.TimestampedValue, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func (r *Runtime) matchUpgradeVersion(device *devicemodel.Device, flag string) string {
	for _, upgrade := range r.deviceUpgrades {
		if strings.TrimSpace(upgrade.ModelCode) != strings.TrimSpace(device.ModelCode) {
			continue
		}
		if strings.TrimSpace(upgrade.Flag) != strings.TrimSpace(flag) {
			continue
		}
		return strings.TrimSpace(upgrade.Version)
	}
	return ""
}

var (
	errVisibleDeviceNotFound = errors.New("visible device not found")
	errMasterOnly            = errors.New("master only")
)

var _ mqttpkg.MessageHandler = (*Runtime)(nil)
