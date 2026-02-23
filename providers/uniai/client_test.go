package uniai

import (
	"testing"

	"github.com/quailyquaily/mistermorph/llm"
	uniaiapi "github.com/quailyquaily/uniai"
	uniaichat "github.com/quailyquaily/uniai/chat"
)

func TestBuildChatOptionsReplaceMessages(t *testing.T) {
	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: "new"},
		},
	}

	opts := append(
		[]uniaiapi.ChatOption{uniaiapi.WithMessages(uniaiapi.User("old"))},
		buildChatOptions(req, "", false, uniaiapi.ToolsEmulationOff, nil)...,
	)

	built, err := uniaichat.BuildRequest(opts...)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if len(built.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(built.Messages))
	}
	if built.Messages[0].Content != "new" {
		t.Fatalf("expected replaced message content 'new', got %q", built.Messages[0].Content)
	}
}

func TestBuildChatOptionsPreserveToolCallIDAsIs(t *testing.T) {
	rawID := "  call_1|ts:abc  "
	req := llm.Request{
		Messages: []llm.Message{
			{
				Role:       "tool",
				Content:    `{"content":"ok"}`,
				ToolCallID: rawID,
			},
		},
	}

	opts := buildChatOptions(req, "", false, uniaiapi.ToolsEmulationOff, nil)
	built, err := uniaichat.BuildRequest(opts...)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if len(built.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(built.Messages))
	}
	if built.Messages[0].ToolCallID != rawID {
		t.Fatalf("expected tool_call_id preserved as-is %q, got %q", rawID, built.Messages[0].ToolCallID)
	}
}

func TestToolCallThoughtSignatureRoundTrip(t *testing.T) {
	sig := "sig_abc"
	origArgs := `{"path":"/tmp/a.txt","dup":1,"dup":2}`
	orig := []uniaiapi.ToolCall{
		{
			ID:               "call_1",
			Type:             "function",
			ThoughtSignature: sig,
			Function: uniaiapi.ToolCallFunction{
				Name:      "read_file",
				Arguments: origArgs,
			},
		},
	}

	out := toLLMToolCalls(orig)
	if len(out) != 1 {
		t.Fatalf("expected 1 llm tool call, got %d", len(out))
	}
	if out[0].ThoughtSignature != sig {
		t.Fatalf("expected round-tripped thought signature %q, got %q", sig, out[0].ThoughtSignature)
	}
	if out[0].RawArguments != origArgs {
		t.Fatalf("expected raw arguments %q, got %q", origArgs, out[0].RawArguments)
	}

	uniaiCalls := toUniaiToolCallsFromLLM(out)
	if len(uniaiCalls) != 1 {
		t.Fatalf("expected 1 uniai tool call, got %d", len(uniaiCalls))
	}
	if uniaiCalls[0].ThoughtSignature != sig {
		t.Fatalf("expected thought signature %q, got %q", sig, uniaiCalls[0].ThoughtSignature)
	}
	if uniaiCalls[0].Function.Arguments != origArgs {
		t.Fatalf("expected exact raw arguments %q, got %q", origArgs, uniaiCalls[0].Function.Arguments)
	}
}

func TestEnsureGeminiToolCallThoughtSignaturesDecodeFromID(t *testing.T) {
	rawID := "call_1|ts:c2lnX2Zyb21faWQ"
	calls := []llm.ToolCall{{
		ID:           rawID,
		Name:         "read_file",
		RawArguments: `{"path":"/tmp/a.txt"}`,
	}}

	out := ensureGeminiToolCallThoughtSignatures(calls)
	if len(out) != 1 {
		t.Fatalf("expected 1 call, got %d", len(out))
	}
	if out[0].ThoughtSignature != "sig_from_id" {
		t.Fatalf("expected signature decoded from id, got %q", out[0].ThoughtSignature)
	}
}

func TestEnsureGeminiToolCallThoughtSignaturesCarryForward(t *testing.T) {
	calls := []llm.ToolCall{
		{
			ID:               "call_1",
			Name:             "read_file",
			RawArguments:     `{"path":"a"}`,
			ThoughtSignature: "sig_1",
		},
		{
			ID:           "call_2",
			Name:         "read_file",
			RawArguments: `{"path":"b"}`,
		},
	}

	out := ensureGeminiToolCallThoughtSignatures(calls)
	if len(out) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(out))
	}
	if out[1].ThoughtSignature != "sig_1" {
		t.Fatalf("expected second call to inherit prior signature, got %q", out[1].ThoughtSignature)
	}
}

func TestEnsureGeminiToolCallThoughtSignaturesSynthesize(t *testing.T) {
	calls := []llm.ToolCall{{
		ID:           "call_42",
		Name:         "read_file",
		RawArguments: `{"path":"a"}`,
	}}

	out1 := ensureGeminiToolCallThoughtSignatures(calls)
	out2 := ensureGeminiToolCallThoughtSignatures(calls)
	if len(out1) != 1 || len(out2) != 1 {
		t.Fatalf("unexpected output lengths: %d, %d", len(out1), len(out2))
	}
	if out1[0].ThoughtSignature == "" {
		t.Fatalf("expected synthesized signature")
	}
	if out1[0].ThoughtSignature != out2[0].ThoughtSignature {
		t.Fatalf("expected deterministic synthesized signature, got %q vs %q", out1[0].ThoughtSignature, out2[0].ThoughtSignature)
	}
}
