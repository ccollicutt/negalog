package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"negalog/pkg/analyzer"
	"negalog/pkg/output"
)

func newTestReport() *output.Report {
	return &output.Report{
		Summary: output.Summary{
			RulesChecked:    2,
			RulesWithIssues: 1,
			TotalIssues:     3,
			LinesProcessed:  100,
		},
		Results: []*analyzer.RuleResult{
			{
				RuleName:    "test-rule",
				RuleType:    analyzer.RuleTypeSequence,
				Description: "Test rule",
				Issues: []analyzer.Issue{
					{
						Type:        analyzer.IssueTypeMissingEnd,
						Description: "Missing end event",
					},
				},
			},
		},
		Metadata: output.Metadata{
			ConfigFile: "test.yaml",
			Sources:    []string{"test.log"},
			AnalyzedAt: time.Now(),
			Duration:   time.Second,
		},
	}
}

func TestClient_Send_Success(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedAuth = r.Header.Get("Authorization")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient()
	report := newTestReport()

	resp := client.Send(context.Background(), report, SendOptions{
		URL: server.URL,
	})

	if !resp.Success() {
		t.Errorf("expected success, got error: %v", resp.Error)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Body != `{"status":"ok"}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}

	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}

	if receivedAuth != "" {
		t.Errorf("expected no auth header, got %s", receivedAuth)
	}

	// Verify payload is valid JSON containing expected fields
	var payload map[string]interface{}
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Errorf("failed to parse received payload: %v", err)
	}

	if _, ok := payload["Summary"]; !ok {
		t.Error("payload missing Summary field")
	}
}

func TestClient_Send_WithBearerToken(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	report := newTestReport()

	resp := client.Send(context.Background(), report, SendOptions{
		URL:   server.URL,
		Token: "secret-token-123",
	})

	if !resp.Success() {
		t.Errorf("expected success, got error: %v", resp.Error)
	}

	if receivedAuth != "Bearer secret-token-123" {
		t.Errorf("expected Bearer token, got %s", receivedAuth)
	}
}

func TestClient_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer server.Close()

	client := NewClient()
	report := newTestReport()

	resp := client.Send(context.Background(), report, SendOptions{
		URL: server.URL,
	})

	if resp.Success() {
		t.Error("expected failure, got success")
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}

	if resp.Error == nil {
		t.Error("expected error to be set")
	}
}

func TestClient_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	report := newTestReport()

	resp := client.Send(context.Background(), report, SendOptions{
		URL:     server.URL,
		Timeout: 50 * time.Millisecond,
	})

	if resp.Success() {
		t.Error("expected failure due to timeout")
	}

	if resp.Error == nil {
		t.Error("expected error to be set")
	}
}

func TestClient_Send_InvalidURL(t *testing.T) {
	client := NewClient()
	report := newTestReport()

	resp := client.Send(context.Background(), report, SendOptions{
		URL: "://invalid-url",
	})

	if resp.Success() {
		t.Error("expected failure for invalid URL")
	}

	if resp.Error == nil {
		t.Error("expected error to be set")
	}
}

func TestClient_Send_ConnectionRefused(t *testing.T) {
	client := NewClient()
	report := newTestReport()

	resp := client.Send(context.Background(), report, SendOptions{
		URL:     "http://127.0.0.1:59999", // Unlikely to be listening
		Timeout: 100 * time.Millisecond,
	})

	if resp.Success() {
		t.Error("expected failure for connection refused")
	}

	if resp.Error == nil {
		t.Error("expected error to be set")
	}
}

func TestResponse_Success(t *testing.T) {
	tests := []struct {
		name        string
		resp        Response
		wantSuccess bool
	}{
		{"200 OK", Response{StatusCode: 200}, true},
		{"201 Created", Response{StatusCode: 201}, true},
		{"204 No Content", Response{StatusCode: 204}, true},
		{"400 Bad Request", Response{StatusCode: 400}, false},
		{"500 Server Error", Response{StatusCode: 500}, false},
		{"With Error", Response{StatusCode: 200, Error: io.EOF}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.Success(); got != tt.wantSuccess {
				t.Errorf("Success() = %v, want %v", got, tt.wantSuccess)
			}
		})
	}
}
