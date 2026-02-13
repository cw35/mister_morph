package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/quailyquaily/mistermorph/tools"
)

type schemaMarkerTool struct{}

func (t schemaMarkerTool) Name() string        { return "schema_marker" }
func (t schemaMarkerTool) Description() string { return "marker tool description" }
func (t schemaMarkerTool) ParameterSchema() string {
	return "SCHEMA_MARKER"
}
func (t schemaMarkerTool) Execute(_ context.Context, _ map[string]any) (string, error) {
	return "ok", nil
}

type planCreateMarkerTool struct{}

func (t planCreateMarkerTool) Name() string            { return "plan_create" }
func (t planCreateMarkerTool) Description() string     { return "plan tool marker" }
func (t planCreateMarkerTool) ParameterSchema() string { return "{}" }
func (t planCreateMarkerTool) Execute(_ context.Context, _ map[string]any) (string, error) {
	return "ok", nil
}

func TestBuildSystemPrompt_UsesToolSummaries(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(schemaMarkerTool{})

	prompt := BuildSystemPrompt(reg, DefaultPromptSpec())
	if !strings.Contains(prompt, "marker tool description") {
		t.Fatalf("expected tool description to be present in prompt")
	}
	if strings.Contains(prompt, "SCHEMA_MARKER") {
		t.Fatalf("expected tool schema to be omitted from prompt")
	}
}

func TestBuildSystemPrompt_HidesPlanOptionWithoutPlanCreate(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(schemaMarkerTool{})

	prompt := BuildSystemPrompt(reg, DefaultPromptSpec())
	if strings.Contains(prompt, "### Option 1: Plan") {
		t.Fatalf("did not expect plan response format without plan_create tool")
	}
	if !strings.Contains(prompt, "### Final") {
		t.Fatalf("expected final response format section")
	}
}

func TestBuildSystemPrompt_ShowsPlanOptionWithPlanCreate(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(schemaMarkerTool{})
	reg.Register(planCreateMarkerTool{})

	prompt := BuildSystemPrompt(reg, DefaultPromptSpec())
	if !strings.Contains(prompt, "### Option 1: Plan") {
		t.Fatalf("expected plan response format with plan_create tool")
	}
	if !strings.Contains(prompt, "### Option 2: Final") {
		t.Fatalf("expected final response format option label with plan_create tool")
	}
}

func TestDefaultPromptSpec_IncludesStaticTodoWorkflowInAdditionalContext(t *testing.T) {
	spec := DefaultPromptSpec()
	joinedRules := strings.Join(spec.Rules, "\n")
	if strings.Contains(joinedRules, "TODO.md entry format examples") {
		t.Fatalf("did not expect TODO workflow content in DefaultPromptSpec rules")
	}
	if len(spec.Blocks) != 0 {
		t.Fatalf("did not expect default blocks in DefaultPromptSpec")
	}

	reg := tools.NewRegistry()
	prompt := BuildSystemPrompt(reg, spec)
	if !strings.Contains(prompt, "## Additional Context") {
		t.Fatalf("expected Additional Context section in rendered prompt")
	}
	if !strings.Contains(prompt, "TODO.md entry format examples") {
		t.Fatalf("expected static TODO workflow examples in rendered prompt")
	}
	if !strings.Contains(prompt, "contacts_send") {
		t.Fatalf("expected TODO workflow contacts_send rule in rendered prompt")
	}
}

func TestPromptRules_StaticURLGuidanceAlwaysPresent(t *testing.T) {
	client := newMockClient(finalResponse("ok"))
	e := New(client, baseRegistry(), baseCfg(), DefaultPromptSpec())

	_, _, err := e.Run(context.Background(), "summarize this text", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := client.allCalls()[0].Messages[0].Content
	if !strings.Contains(prompt, "When a user provides a direct URL, prefer `url_fetch`") {
		t.Fatalf("expected static url_fetch guidance in prompt")
	}
	if !strings.Contains(prompt, "If `url_fetch` fails (blocked, timeout, non-2xx), do not fabricate") {
		t.Fatalf("expected static url_fetch failure guidance in prompt")
	}
}

func TestPromptRules_PlanCreateRules_WhenToolRegistered(t *testing.T) {
	client := newMockClient(finalResponse("ok"))
	reg := baseRegistry()
	reg.Register(planCreateMarkerTool{})
	e := New(client, reg, baseCfg(), DefaultPromptSpec())

	_, _, err := e.Run(context.Background(), "do a complex migration task", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := client.allCalls()[0].Messages[0].Content
	if !strings.Contains(prompt, planCreatePromptBlockTitle) {
		t.Fatalf("expected plan_create guidance block in prompt")
	}
	if !strings.Contains(prompt, "use the `plan_create` tool first") {
		t.Fatalf("expected plan_create guidance content in prompt")
	}
}

func TestPromptRules_PlanCreateRules_NotInjected_WhenToolMissing(t *testing.T) {
	client := newMockClient(finalResponse("ok"))
	e := New(client, baseRegistry(), baseCfg(), DefaultPromptSpec())

	_, _, err := e.Run(context.Background(), "do a complex migration task", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := client.allCalls()[0].Messages[0].Content
	if strings.Contains(prompt, planCreatePromptBlockTitle) {
		t.Fatalf("did not expect plan_create guidance block without tool")
	}
}
