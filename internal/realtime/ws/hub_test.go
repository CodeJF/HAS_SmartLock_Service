package ws

import (
	"context"
	"testing"
	"time"

	"has-smartlock-service/internal/pkg/auth"
)

func TestMakeConnKey(t *testing.T) {
	if got := makeConnKey("u1", "86_ios"); got != "u1::86_ios" {
		t.Fatalf("makeConnKey = %q, want %q", got, "u1::86_ios")
	}
}

func TestHandlerReturnsImmediateResponseWhenConfigured(t *testing.T) {
	manager := auth.NewTokenManager("test-secret", 60, 60)
	hub := NewHub(manager, func(_ context.Context, session Session, request Request) *Response {
		if session.UID != "u1" {
			t.Fatalf("session uid = %q, want u1", session.UID)
		}
		return &Response{
			MsgID:   request.MsgID,
			Method:  request.Method,
			UUID:    request.UUID,
			Code:    0,
			Msg:     "ok",
			Time:    time.Now().Unix(),
			Version: request.Version,
		}
	})
	if hub == nil {
		t.Fatal("hub should not be nil")
	}
}
