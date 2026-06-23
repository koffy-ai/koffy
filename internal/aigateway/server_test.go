package aigateway

import (
	"encoding/json"
	"testing"
)

func TestInspectChatRequest(t *testing.T) {
	model, stream, err := inspectChatRequest([]byte(`{"model":"openai-chat-default","messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if model != "openai-chat-default" {
		t.Fatalf("model = %q", model)
	}
	if stream {
		t.Fatal("stream should be false")
	}

	_, stream, err = inspectChatRequest([]byte(`{"model":"openai-chat-default","stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !stream {
		t.Fatal("stream should be true")
	}
}

func TestParseUsage(t *testing.T) {
	usage, model, jobID, err := parseUsage([]byte(`{
		"id": "chatcmpl_test",
		"model": "gpt-test",
		"usage": {
			"prompt_tokens": 12,
			"completion_tokens": 8,
			"total_tokens": 20
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-test" || jobID != "chatcmpl_test" {
		t.Fatalf("model/jobID = %q/%q", model, jobID)
	}
	if usage.PromptTokens != 12 || usage.CompletionTokens != 8 || usage.TotalTokens != 20 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestParseUsageRequiresTotalTokens(t *testing.T) {
	_, _, _, err := parseUsage([]byte(`{"id":"x","model":"m","usage":{"prompt_tokens":1}}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInspectImageRequest(t *testing.T) {
	model, images, err := inspectImageRequest([]byte(`{"model":"openai-image-default","prompt":"x","n":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if model != "openai-image-default" || images != 3 {
		t.Fatalf("model/images = %q/%d", model, images)
	}

	_, images, err = inspectImageRequest([]byte(`{"model":"openai-image-default","prompt":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if images != 1 {
		t.Fatalf("default images = %d", images)
	}
}

func TestParseImageUsage(t *testing.T) {
	usage, model, jobID, err := parseImageUsage([]byte(`{
		"id": "img_test",
		"model": "gpt-image-test",
		"data": [{"url":"https://example.test/1.png"},{"url":"https://example.test/2.png"}]
	}`), "fallback-image-model")
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-image-test" || jobID != "img_test" {
		t.Fatalf("model/jobID = %q/%q", model, jobID)
	}
	if usage.Images != 2 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestParseImageUsageRequiresData(t *testing.T) {
	_, _, _, err := parseImageUsage([]byte(`{"id":"img_test","model":"m","data":[]}`), "fallback")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureStreamUsage(t *testing.T) {
	body, err := ensureStreamUsage([]byte(`{"model":"openai-chat-default","stream":true}`))
	if err != nil {
		t.Fatal(err)
	}

	model, stream, err := inspectChatRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if model != "openai-chat-default" || !stream {
		t.Fatalf("model/stream = %q/%v", model, stream)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	options, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options missing: %s", string(body))
	}
	if options["include_usage"] != true {
		t.Fatalf("include_usage = %#v", options["include_usage"])
	}
}

func TestParseSSEUsageLine(t *testing.T) {
	usage, model, jobID, ok := parseSSEUsageLine([]byte(`data: {"id":"chatcmpl_stream","model":"gpt-test","usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
	if !ok {
		t.Fatal("expected usage chunk")
	}
	if model != "gpt-test" || jobID != "chatcmpl_stream" {
		t.Fatalf("model/jobID = %q/%q", model, jobID)
	}
	if usage.TotalTokens != 7 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestParseSSEUsageLineIgnoresDone(t *testing.T) {
	if _, _, _, ok := parseSSEUsageLine([]byte("data: [DONE]")); ok {
		t.Fatal("DONE should not parse as usage")
	}
}
