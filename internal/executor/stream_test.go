package executor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamLine_WhenSystemInit_ExtractsSessionID(t *testing.T) {
	t.Parallel()

	line := `{"type":"system","subtype":"init","session_id":"ses_abc123","tools":["Read","Write"]}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	assert.Equal(t, "system", event.Type)
	assert.Equal(t, "init", event.Subtype)
	assert.Equal(t, "ses_abc123", event.SessionID)
}

func TestParseStreamLine_WhenAssistantText_ExtractsContent(t *testing.T) {
	t.Parallel()

	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll fix the auth bug."}]}}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	assert.Equal(t, "assistant", event.Type)
	require.NotNil(t, event.Message)
	require.Len(t, event.Message.Content, 1)
	assert.Equal(t, "text", event.Message.Content[0].Type)
	assert.Equal(t, "I'll fix the auth bug.", event.Message.Content[0].Text)
}

func TestParseStreamLine_WhenToolUse_ExtractsToolName(t *testing.T) {
	t.Parallel()

	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"auth.go"}}]}}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	require.NotNil(t, event.Message)
	require.Len(t, event.Message.Content, 1)
	assert.Equal(t, "tool_use", event.Message.Content[0].Type)
	assert.Equal(t, "Write", event.Message.Content[0].Name)
}

func TestParseStreamLine_WhenResultSuccess_ExtractsCostAndTurns(t *testing.T) {
	t.Parallel()

	line := `{"type":"result","subtype":"success","session_id":"ses_abc","cost_usd":0.34,"duration_ms":45000,"num_turns":5}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	assert.Equal(t, "result", event.Type)
	assert.Equal(t, "success", event.Subtype)
	assert.InDelta(t, 0.34, event.CostUSD, 0.001)
	assert.Equal(t, int64(45000), event.Duration)
	assert.Equal(t, 5, event.NumTurns)
	assert.Equal(t, "ses_abc", event.SessionID)
}

func TestParseStreamLine_WhenResultFailure_HasSubtype(t *testing.T) {
	t.Parallel()

	line := `{"type":"result","subtype":"error","cost_usd":0.12,"duration_ms":5000,"num_turns":2}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	assert.Equal(t, "result", event.Type)
	assert.Equal(t, "error", event.Subtype)
}

func TestParseStreamLine_WhenMalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()

	line := `{this is not json}`

	event, err := ParseStreamLine([]byte(line))
	assert.Nil(t, event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing stream event")
}

func TestParseStreamLine_WhenEmptyLine_ReturnsNil(t *testing.T) {
	t.Parallel()

	event, err := ParseStreamLine([]byte(""))
	assert.Nil(t, event)
	assert.NoError(t, err)
}

func TestParseStreamLine_WhenUnknownType_StillParses(t *testing.T) {
	t.Parallel()

	line := `{"type":"unknown_event","subtype":"foo"}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	assert.Equal(t, "unknown_event", event.Type)
	assert.Equal(t, "foo", event.Subtype)
}

func TestParseStreamLine_WhenMultipleContentBlocks_ParsesAll(t *testing.T) {
	t.Parallel()

	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hello"},{"type":"tool_use","name":"Read"},{"type":"text","text":"world"}]}}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	require.Len(t, event.Message.Content, 3)
	assert.Equal(t, "text", event.Message.Content[0].Type)
	assert.Equal(t, "tool_use", event.Message.Content[1].Type)
	assert.Equal(t, "text", event.Message.Content[2].Type)
}

func TestParseStreamLine_WhenZeroCost_ParsesAsZero(t *testing.T) {
	t.Parallel()

	line := `{"type":"result","subtype":"success","cost_usd":0,"num_turns":0}`

	event, err := ParseStreamLine([]byte(line))
	require.NoError(t, err)
	assert.Equal(t, float64(0), event.CostUSD)
	assert.Equal(t, 0, event.NumTurns)
}

func TestExtractProgress_WhenTextContent_ReturnsTruncated(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{
		Message: &StreamMessage{
			Content: []ContentBlock{
				{Type: "text", Text: "I'll fix this bug now."},
			},
		},
	}

	progress := ExtractProgress(event)
	assert.Equal(t, "I'll fix this bug now.", progress)
}

func TestExtractProgress_WhenLongText_Truncates(t *testing.T) {
	t.Parallel()

	longText := strings.Repeat("a", 300)
	event := &StreamEvent{
		Message: &StreamMessage{
			Content: []ContentBlock{
				{Type: "text", Text: longText},
			},
		},
	}

	progress := ExtractProgress(event)
	assert.Len(t, progress, 203) // 200 + "..."
	assert.True(t, strings.HasSuffix(progress, "..."))
}

func TestExtractProgress_WhenToolUse_ReturnsToolName(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{
		Message: &StreamMessage{
			Content: []ContentBlock{
				{Type: "tool_use", Name: "Edit"},
			},
		},
	}

	progress := ExtractProgress(event)
	assert.Equal(t, "Using tool: Edit", progress)
}

func TestExtractProgress_WhenNoMessage_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{Type: "system"}
	assert.Empty(t, ExtractProgress(event))
}

func TestExtractProgress_WhenEmptyContent_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{
		Message: &StreamMessage{
			Content: []ContentBlock{},
		},
	}
	assert.Empty(t, ExtractProgress(event))
}

func TestExtractOutput_CollectsAllTextBlocks(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{
		Message: &StreamMessage{
			Content: []ContentBlock{
				{Type: "text", Text: "Part 1. "},
				{Type: "tool_use", Name: "Read"},
				{Type: "text", Text: "Part 2."},
			},
		},
	}

	output := ExtractOutput(event)
	assert.Equal(t, "Part 1. Part 2.", output)
}

func TestExtractOutput_WhenNoMessage_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{Type: "system"}
	assert.Empty(t, ExtractOutput(event))
}

func TestExtractOutput_WhenOnlyToolUse_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	event := &StreamEvent{
		Message: &StreamMessage{
			Content: []ContentBlock{
				{Type: "tool_use", Name: "Read"},
			},
		},
	}
	assert.Empty(t, ExtractOutput(event))
}

func TestTruncateStr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		max    int
		expect string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"too long", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, truncateStr(tt.input, tt.max))
		})
	}
}

func TestTruncateBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  []byte
		max    int
		expect string
	}{
		{"short", []byte("hi"), 10, "hi"},
		{"exact", []byte("hello"), 5, "hello"},
		{"too long", []byte("hello world"), 5, "hello..."},
		{"empty", []byte{}, 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, truncateBytes(tt.input, tt.max))
		})
	}
}

func TestParseStream_FullConversation(t *testing.T) {
	t.Parallel()

	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"ses_test123"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll fix the bug."}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done, the fix is applied."}]}}`,
		`{"type":"result","subtype":"success","cost_usd":0.42,"duration_ms":30000,"num_turns":3}`,
	}, "\n")

	result := &Result{}
	var progressMessages []string
	onProgress := func(eventType, message string) {
		progressMessages = append(progressMessages, eventType+":"+message)
	}

	exec := &ClaudeExecutor{}
	exec.parseStream("test-task", strings.NewReader(stream), result, onProgress)

	assert.Equal(t, "ses_test123", result.SessionID)
	assert.InDelta(t, 0.42, result.CostUSD, 0.001)
	assert.Equal(t, 3, result.Turns)
	assert.Contains(t, result.Output, "I'll fix the bug.")
	assert.Contains(t, result.Output, "Done, the fix is applied.")
	assert.Contains(t, progressMessages, "progress:I'll fix the bug.")
	assert.Contains(t, progressMessages, "progress:Using tool: Edit")
}

func TestParseStream_WhenMalformedLines_SkipsThem(t *testing.T) {
	t.Parallel()

	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"ses_ok"}`,
		`{broken json here}`,
		`not even json`,
		`{"type":"result","subtype":"success","cost_usd":0.10,"num_turns":1}`,
	}, "\n")

	result := &Result{}
	exec := &ClaudeExecutor{}
	exec.parseStream("test-task", strings.NewReader(stream), result, nil)

	assert.Equal(t, "ses_ok", result.SessionID)
	assert.InDelta(t, 0.10, result.CostUSD, 0.001)
}

func TestParseStream_WhenEmptyStream_ProducesEmptyResult(t *testing.T) {
	t.Parallel()

	result := &Result{}
	exec := &ClaudeExecutor{}
	exec.parseStream("test-task", strings.NewReader(""), result, nil)

	assert.Empty(t, result.SessionID)
	assert.Empty(t, result.Output)
	assert.Equal(t, float64(0), result.CostUSD)
}

func TestParseStream_WhenNoProgressFunc_DoesNotPanic(t *testing.T) {
	t.Parallel()

	stream := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hello"}]}}`

	result := &Result{}
	exec := &ClaudeExecutor{}
	require.NotPanics(t, func() {
		exec.parseStream("test-task", strings.NewReader(stream), result, nil)
	})
	assert.Equal(t, "hello", result.Output)
}
