package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/pkg/protocol"
)

type HandlerFunc func(ctx context.Context, session Session, request Request) *Response

type Session struct {
	UID       string
	PhoneCode string
}

type Hub struct {
	upgrader websocket.Upgrader
	auth     *auth.TokenManager
	handler  HandlerFunc

	mu    sync.RWMutex
	conns map[string]*websocket.Conn
}

type Request struct {
	MsgID   string         `json:"msg_id"`
	Method  string         `json:"method"`
	UUID    string         `json:"uuid"`
	Time    int64          `json:"time"`
	Version string         `json:"version"`
	Data    map[string]any `json:"data"`
}

type Response struct {
	MsgID   string         `json:"msg_id"`
	Method  string         `json:"method"`
	UUID    string         `json:"uuid,omitempty"`
	Time    int64          `json:"time"`
	Code    int            `json:"code"`
	Msg     string         `json:"msg"`
	Version string         `json:"version,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func NewHub(tokenManager *auth.TokenManager, handler HandlerFunc) *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		auth:    tokenManager,
		handler: handler,
		conns:   make(map[string]*websocket.Conn),
	}
}

func (h *Hub) RegisterRoute(router *gin.Engine, path string) {
	router.GET(path, h.handle)
}

func (h *Hub) handle(c *gin.Context) {
	accessToken := protocol.AccessTokenFromHeader(c)
	phoneCode := strings.TrimSpace(c.GetHeader("phone_code"))
	if accessToken == "" || phoneCode == "" {
		httpx.Fail(c, 2001, "missing websocket auth headers", nil)
		return
	}
	claims, err := h.auth.ParseAccessToken(accessToken)
	if err != nil {
		httpx.Fail(c, 1005, "access token invalid", nil)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	key := makeConnKey(claims.UID, phoneCode)
	h.replaceConn(key, conn)
	go h.readLoop(Session{UID: claims.UID, PhoneCode: phoneCode}, key, conn)
}

func (h *Hub) Notify(uid, phoneCode string, response Response) error {
	key := makeConnKey(uid, phoneCode)
	h.mu.RLock()
	conn := h.conns[key]
	h.mu.RUnlock()
	if conn == nil {
		return nil
	}
	return conn.WriteJSON(response)
}

func (h *Hub) Broadcast(uid string, response Response) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for key, conn := range h.conns {
		if !strings.HasPrefix(key, uid+"::") {
			continue
		}
		_ = conn.WriteJSON(response)
	}
}

func (h *Hub) replaceConn(key string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.conns[key]; old != nil {
		_ = old.Close()
	}
	h.conns[key] = conn
}

func (h *Hub) readLoop(session Session, key string, conn *websocket.Conn) {
	defer func() {
		h.mu.Lock()
		if h.conns[key] == conn {
			delete(h.conns, key)
		}
		h.mu.Unlock()
		_ = conn.Close()
	}()

	for {
		_, body, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if string(body) == "ping" {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("pong"))
			continue
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			_ = conn.WriteJSON(Response{Code: 4000, Msg: "invalid websocket payload", Time: time.Now().Unix()})
			continue
		}
		if strings.TrimSpace(req.Method) == "" || strings.TrimSpace(req.MsgID) == "" || strings.TrimSpace(req.UUID) == "" || strings.TrimSpace(req.Version) == "" || req.Time == 0 {
			_ = conn.WriteJSON(Response{MsgID: req.MsgID, Method: req.Method, UUID: req.UUID, Code: 4000, Msg: "missing required fields", Time: time.Now().Unix(), Version: req.Version})
			continue
		}

		if h.handler == nil {
			_ = conn.WriteJSON(Response{MsgID: req.MsgID, Method: req.Method, UUID: req.UUID, Code: 0, Msg: "ok", Time: time.Now().Unix(), Version: req.Version})
			continue
		}
		if resp := h.handler(context.Background(), session, req); resp != nil {
			_ = conn.WriteJSON(resp)
		}
	}
}

func makeConnKey(uid, phoneCode string) string {
	return uid + "::" + phoneCode
}
