package contactsruntime

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/quailyquaily/mistermorph/contacts"
)

func TestBuildEnvelopePayload_RequiresSessionID(t *testing.T) {
	now := time.Date(2026, 2, 7, 4, 31, 30, 0, time.UTC)
	decision := contacts.ShareDecision{
		ContentType:    "text/plain",
		PayloadBase64:  base64.RawURLEncoding.EncodeToString([]byte("hello")),
		IdempotencyKey: "manual:1",
		ItemID:         "cand_1",
	}

	_, err := buildEnvelopePayload(decision, decision.ContentType, decision.PayloadBase64, now)
	if err == nil {
		t.Fatalf("expected error when session_id is missing")
	}
}

func TestBuildEnvelopePayload_FromJSON(t *testing.T) {
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

	raw, err := buildEnvelopePayload(decision, decision.ContentType, decision.PayloadBase64, now)
	if err != nil {
		t.Fatalf("buildEnvelopePayload() error = %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if got := envelope["text"]; got != "pong" {
		t.Fatalf("text mismatch: got %v want pong", got)
	}
	if got := envelope["session_id"]; got != "0194f5c0-8f6e-7d9d-a4d7-6d8d4f35f456" {
		t.Fatalf("session_id mismatch: got %v", got)
	}
	if got := envelope["reply_to"]; got != "msg_prev" {
		t.Fatalf("reply_to mismatch: got %v want msg_prev", got)
	}
}

func TestBuildEnvelopePayload_InvalidPayload(t *testing.T) {
	now := time.Date(2026, 2, 7, 4, 34, 0, 0, time.UTC)
	decision := contacts.ShareDecision{
		PayloadBase64: "***invalid***",
	}

	if _, err := buildEnvelopePayload(decision, "application/json", decision.PayloadBase64, now); err == nil {
		t.Fatalf("expected error for invalid payload_base64")
	}
}
