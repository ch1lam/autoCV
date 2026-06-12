package openaiprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/openai/openai-go/v3/option"
)

func TestSDKClientSendsStrictResponsesRequest(t *testing.T) {
	var received map[string]any
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected request path %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf(
				"unexpected authorization header %q",
				request.Header.Get("Authorization"),
			)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		return jsonResponse(http.StatusOK, `{
					"id": "resp_test",
					"object": "response",
					"created_at": 1781222400,
					"status": "completed",
					"model": "gpt-test",
					"output": [{
						"id": "msg_test",
						"type": "message",
						"status": "completed",
						"role": "assistant",
						"content": [{
							"type": "output_text",
							"text": "{\"ok\":true}",
							"annotations": [],
							"logprobs": []
						}]
					}],
					"usage": {
						"input_tokens": 12,
						"input_tokens_details": {"cached_tokens": 0},
						"output_tokens": 7,
						"output_tokens_details": {"reasoning_tokens": 0},
						"total_tokens": 19
					}
				}`), nil
	})

	client, err := newSDKClient(
		"test-key",
		"https://example.test/v1",
		"gpt-test",
		option.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("create SDK client: %v", err)
	}
	response, err := client.Generate(context.Background(), Request{
		Prompt:     "Return a JSON object.",
		SchemaName: "test_response",
		Schema: json.RawMessage(`{
			"type": "object",
			"properties": {"ok": {"type": "boolean"}},
			"required": ["ok"],
			"additionalProperties": false
		}`),
		Input: json.RawMessage(`{"source":"fixture"}`),
	})
	if err != nil {
		t.Fatalf("generate structured response: %v", err)
	}
	if string(response.Output) != `{"ok":true}` ||
		response.Usage != (Usage{
			InputTokens: 12, OutputTokens: 7, TotalTokens: 19,
		}) {
		t.Fatalf("unexpected response %#v", response)
	}

	if received["model"] != "gpt-test" ||
		received["instructions"] != "Return a JSON object." ||
		received["input"] != `{"source":"fixture"}` ||
		received["store"] != false {
		t.Fatalf("unexpected request payload %#v", received)
	}
	text, ok := received["text"].(map[string]any)
	if !ok {
		t.Fatalf("missing text config %#v", received["text"])
	}
	format, ok := text["format"].(map[string]any)
	if !ok ||
		format["type"] != "json_schema" ||
		format["name"] != "test_response" ||
		format["strict"] != true {
		t.Fatalf("unexpected response format %#v", text["format"])
	}
}

func TestSDKClientMarksRateLimitAsRetryable(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(
			http.StatusTooManyRequests,
			`{"error":{"message":"slow down","type":"rate_limit_error","param":"","code":"rate_limit"}}`,
		), nil
	})

	client, err := newSDKClient(
		"test-key",
		"https://example.test",
		"gpt-test",
		option.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("create SDK client: %v", err)
	}
	_, err = client.Generate(context.Background(), Request{
		SchemaName: "test_response",
		Schema:     json.RawMessage(`{"type":"object"}`),
		Input:      json.RawMessage(`{"source":"fixture"}`),
	})
	if err == nil || !isRetryable(err) {
		t.Fatalf("expected retryable rate limit error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return function(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestSDKClientBuildsRepairInputWithoutChangingTaskInput(t *testing.T) {
	input := json.RawMessage(`{"source":"fixture"}`)
	contents, err := requestInput(Request{
		Input: input,
		Repair: &RepairRequest{
			InvalidOutput:   `{"ok":"yes"}`,
			ValidationError: "ok must be boolean",
		},
	})
	if err != nil {
		t.Fatalf("build repair input: %v", err)
	}
	var payload struct {
		TaskInput map[string]string `json:"task_input"`
		Repair    struct {
			InvalidOutput   string `json:"invalid_output"`
			ValidationError string `json:"validation_error"`
			Instruction     string `json:"instruction"`
		} `json:"repair"`
	}
	if err := json.Unmarshal([]byte(contents), &payload); err != nil {
		t.Fatalf("decode repair input: %v", err)
	}
	if payload.TaskInput["source"] != "fixture" ||
		payload.Repair.InvalidOutput != `{"ok":"yes"}` ||
		payload.Repair.ValidationError != "ok must be boolean" ||
		payload.Repair.Instruction == "" {
		t.Fatalf("unexpected repair input %#v", payload)
	}
}
