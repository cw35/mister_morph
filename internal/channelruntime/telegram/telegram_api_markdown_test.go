package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessageMarkdownV2ReplyFallbackToV1(t *testing.T) {
	var calls []telegramSendMessageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottoken/sendMessage" {
			http.NotFound(w, r)
			return
		}
		var req telegramSendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		calls = append(calls, req)
		if req.ParseMode == "MarkdownV2" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: can't parse entities"}`))
			return
		}
		if req.ParseMode == "Markdown" {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"unexpected parse mode"}`))
	}))
	defer srv.Close()

	api := newTelegramAPI(srv.Client(), srv.URL, "token")
	err := api.sendMessageMarkdownV2Reply(context.Background(), 42, "*bad*", true, 0)
	if err != nil {
		t.Fatalf("sendMessageMarkdownV2Reply() error = %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("len(calls) = %d, want 3", len(calls))
	}
	if calls[0].ParseMode != "MarkdownV2" || calls[1].ParseMode != "MarkdownV2" || calls[2].ParseMode != "Markdown" {
		t.Fatalf("unexpected parse mode sequence: %#v", []string{calls[0].ParseMode, calls[1].ParseMode, calls[2].ParseMode})
	}
	if calls[0].Text == calls[1].Text {
		t.Fatalf("expected escaped markdown retry text to differ, got %q", calls[1].Text)
	}
	if !strings.Contains(calls[1].Text, `\*`) {
		t.Fatalf("expected escaped markdown retry text, got %q", calls[1].Text)
	}
}

func TestSendMessageMarkdownV2ReplyFallbackToPlainWhenV1Fails(t *testing.T) {
	var calls []telegramSendMessageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottoken/sendMessage" {
			http.NotFound(w, r)
			return
		}
		var req telegramSendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		calls = append(calls, req)
		switch req.ParseMode {
		case "MarkdownV2", "Markdown":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: can't parse entities"}`))
		case "":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"description":"unexpected parse mode"}`))
		}
	}))
	defer srv.Close()

	api := newTelegramAPI(srv.Client(), srv.URL, "token")
	err := api.sendMessageMarkdownV2Reply(context.Background(), 42, "*bad*", true, 0)
	if err != nil {
		t.Fatalf("sendMessageMarkdownV2Reply() error = %v", err)
	}
	if len(calls) != 4 {
		t.Fatalf("len(calls) = %d, want 4", len(calls))
	}
	got := []string{calls[0].ParseMode, calls[1].ParseMode, calls[2].ParseMode, calls[3].ParseMode}
	want := []string{"MarkdownV2", "MarkdownV2", "Markdown", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parse mode at %d = %q, want %q (full=%#v)", i, got[i], want[i], got)
		}
	}
}

func TestSendMessageMarkdownV1ReplyUsesMarkdownParseMode(t *testing.T) {
	var calls []telegramSendMessageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottoken/sendMessage" {
			http.NotFound(w, r)
			return
		}
		var req telegramSendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		calls = append(calls, req)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	api := newTelegramAPI(srv.Client(), srv.URL, "token")
	err := api.sendMessageMarkdownV1Reply(context.Background(), 42, "*hello*", true, 99)
	if err != nil {
		t.Fatalf("sendMessageMarkdownV1Reply() error = %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].ParseMode != "Markdown" {
		t.Fatalf("parse_mode = %q, want Markdown", calls[0].ParseMode)
	}
	if calls[0].ReplyToMessageID != 99 {
		t.Fatalf("reply_to_message_id = %d, want 99", calls[0].ReplyToMessageID)
	}
}
