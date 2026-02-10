package contactsruntime

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/quailyquaily/mistermorph/contacts"
)

func TestBuildMAEPDataPushRequest_EnvelopeTopicFromPlainText(t *testing.T) {
	now := time.Date(2026, 2, 7, 4, 31, 30, 0, time.UTC)
	decision := contacts.ShareDecision{
		ContentType:    "text/plain",
		PayloadBase64:  base64.RawURLEncoding.EncodeToString([]byte("hello")),
		IdempotencyKey: "manual:1",
		ItemID:         "cand_1",
	}

	_, err := buildMAEPDataPushRequest(decision, now)
	if err == nil {
		t.Fatalf("expected error when session_id is missing for dialogue topic")
	}
}

func TestBuildMAEPDataPushRequest_EnvelopeTopicFromJSON(t *testing.T) {
	now := time.Date(2026, 2, 7, 4, 32, 0, 0, time.UTC)
	payload := map[string]any{
		"text":       "pong",
		"session_id": "0194f5c0-8f6e-7d9d-a4d7-6d8d4f35f456",
		"reply_to":   "msg_prev",
	}
	payloadRaw, _ := json.Marshal(payload)
	decision := contacts.ShareDecision{
		ContentType:    "application/json",
		PayloadBase64:  base64.RawURLEncoding.EncodeToString(payloadRaw),
		IdempotencyKey: "manual:2",
		ItemID:         "",
	}

	req, err := buildMAEPDataPushRequest(decision, now)
	if err != nil {
		t.Fatalf("buildMAEPDataPushRequest() error = %v", err)
	}
	if req.Topic != contacts.ShareTopic {
		t.Fatalf("Topic mismatch: got %q want %q", req.Topic, contacts.ShareTopic)
	}
	if req.ContentType != "application/json" {
		t.Fatalf("ContentType mismatch: got %q want %q", req.ContentType, "application/json")
	}

	raw, err := base64.RawURLEncoding.DecodeString(req.PayloadBase64)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got := envelope["text"]; got != "pong" {
		t.Fatalf("text mismatch: got %v want pong", got)
	}
	if got := envelope["session_id"]; got != "0194f5c0-8f6e-7d9d-a4d7-6d8d4f35f456" {
		t.Fatalf("session_id mismatch: got %v want 0194f5c0-8f6e-7d9d-a4d7-6d8d4f35f456", got)
	}
	if got := envelope["reply_to"]; got != "msg_prev" {
		t.Fatalf("reply_to mismatch: got %v want msg_prev", got)
	}
}

func TestBuildMAEPDataPushRequest_EnvelopeTopicFromPlainTextWithSession(t *testing.T) {
	now := time.Date(2026, 2, 7, 4, 33, 0, 0, time.UTC)
	payload := map[string]any{
		"text":       "x",
		"session_id": "0194f5c0-8f6e-7d9d-a4d7-6d8d4f35f456",
	}
	payloadRaw, _ := json.Marshal(payload)
	decision := contacts.ShareDecision{
		ContentType:   "application/json",
		PayloadBase64: base64.RawURLEncoding.EncodeToString(payloadRaw),
	}

	req, err := buildMAEPDataPushRequest(decision, now)
	if err != nil {
		t.Fatalf("buildMAEPDataPushRequest() error = %v", err)
	}
	if req.Topic != contacts.ShareTopic {
		t.Fatalf("Topic mismatch: got %q want %q", req.Topic, contacts.ShareTopic)
	}
}

func TestBuildMAEPDataPushRequest_InvalidPayload(t *testing.T) {
	now := time.Date(2026, 2, 7, 4, 34, 0, 0, time.UTC)
	decision := contacts.ShareDecision{
		PayloadBase64: "***invalid***",
	}

	if _, err := buildMAEPDataPushRequest(decision, now); err == nil {
		t.Fatalf("expected error for invalid payload_base64")
	}
}
