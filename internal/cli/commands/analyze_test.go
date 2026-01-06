package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ccollicutt/negalog/pkg/config"
	"github.com/ccollicutt/negalog/pkg/output"
)

func TestShouldFireWebhook(t *testing.T) {
	tests := []struct {
		name      string
		trigger   config.WebhookTrigger
		hasIssues bool
		want      bool
	}{
		{"on_issues with issues", config.WebhookTriggerOnIssues, true, true},
		{"on_issues without issues", config.WebhookTriggerOnIssues, false, false},
		{"always with issues", config.WebhookTriggerAlways, true, true},
		{"always without issues", config.WebhookTriggerAlways, false, true},
		{"never with issues", config.WebhookTriggerNever, true, false},
		{"never without issues", config.WebhookTriggerNever, false, false},
		{"empty trigger with issues", "", true, true},
		{"empty trigger without issues", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFireWebhook(tt.trigger, tt.hasIssues)
			if got != tt.want {
				t.Errorf("shouldFireWebhook(%q, %v) = %v, want %v",
					tt.trigger, tt.hasIssues, got, tt.want)
			}
		})
	}
}

func TestCollectWebhooks(t *testing.T) {
	// Test with config webhooks only
	t.Run("config only", func(t *testing.T) {
		cfg := &config.Config{
			Webhooks: []config.WebhookConfig{
				{Name: "slack", URL: "https://slack.com/webhook"},
				{Name: "pagerduty", URL: "https://pagerduty.com/webhook"},
			},
		}
		opts := &AnalyzeOptions{}

		webhooks := collectWebhooks(cfg, opts)

		if len(webhooks) != 2 {
			t.Errorf("got %d webhooks, want 2", len(webhooks))
		}
	})

	// Test with CLI webhook only
	t.Run("cli only", func(t *testing.T) {
		cfg := &config.Config{}
		opts := &AnalyzeOptions{
			WebhookURL:     "https://cli.example.com/webhook",
			WebhookToken:   "secret",
			WebhookTrigger: "always",
		}

		webhooks := collectWebhooks(cfg, opts)

		if len(webhooks) != 1 {
			t.Errorf("got %d webhooks, want 1", len(webhooks))
		}
		if webhooks[0].Name != "cli" {
			t.Errorf("got name %q, want cli", webhooks[0].Name)
		}
		if webhooks[0].Token != "secret" {
			t.Errorf("got token %q, want secret", webhooks[0].Token)
		}
		if webhooks[0].Trigger != config.WebhookTriggerAlways {
			t.Errorf("got trigger %q, want always", webhooks[0].Trigger)
		}
	})

	// Test with both config and CLI webhooks
	t.Run("config and cli", func(t *testing.T) {
		cfg := &config.Config{
			Webhooks: []config.WebhookConfig{
				{Name: "config-webhook", URL: "https://config.example.com/webhook"},
			},
		}
		opts := &AnalyzeOptions{
			WebhookURL: "https://cli.example.com/webhook",
		}

		webhooks := collectWebhooks(cfg, opts)

		if len(webhooks) != 2 {
			t.Errorf("got %d webhooks, want 2", len(webhooks))
		}
	})

	// Test with empty trigger defaults to on_issues
	t.Run("default trigger", func(t *testing.T) {
		cfg := &config.Config{}
		opts := &AnalyzeOptions{
			WebhookURL: "https://example.com/webhook",
		}

		webhooks := collectWebhooks(cfg, opts)

		if len(webhooks) != 1 {
			t.Fatalf("got %d webhooks, want 1", len(webhooks))
		}
		if webhooks[0].Trigger != config.WebhookTriggerOnIssues {
			t.Errorf("got trigger %q, want on_issues", webhooks[0].Trigger)
		}
	})
}

func TestSendWebhooks(t *testing.T) {
	var receivedPayloads [][]byte
	var receivedAuths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedPayloads = append(receivedPayloads, body)
		receivedAuths = append(receivedAuths, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Webhooks: []config.WebhookConfig{
			{
				Name:    "test-webhook",
				URL:     server.URL,
				Token:   "test-token",
				Trigger: config.WebhookTriggerAlways,
				Timeout: 10 * time.Second,
			},
		},
	}
	opts := &AnalyzeOptions{}

	report := &output.Report{
		Summary: output.Summary{
			RulesChecked:    1,
			RulesWithIssues: 1,
			TotalIssues:     5,
			LinesProcessed:  100,
		},
	}

	// Call sendWebhooks
	sendWebhooks(context.Background(), cfg, opts, report)

	if len(receivedPayloads) != 1 {
		t.Fatalf("expected 1 webhook call, got %d", len(receivedPayloads))
	}

	// Verify payload is valid JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(receivedPayloads[0], &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}

	// Verify auth header
	if receivedAuths[0] != "Bearer test-token" {
		t.Errorf("got auth %q, want Bearer test-token", receivedAuths[0])
	}
}

func TestSendWebhooks_OnIssuesTrigger(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Webhooks: []config.WebhookConfig{
			{
				Name:    "on-issues-webhook",
				URL:     server.URL,
				Trigger: config.WebhookTriggerOnIssues,
				Timeout: 10 * time.Second,
			},
		},
	}
	opts := &AnalyzeOptions{}

	// Report with no issues - should NOT fire
	reportNoIssues := &output.Report{
		Summary: output.Summary{TotalIssues: 0},
	}
	sendWebhooks(context.Background(), cfg, opts, reportNoIssues)

	if callCount != 0 {
		t.Errorf("on_issues webhook fired with no issues, callCount = %d", callCount)
	}

	// Report with issues - should fire
	reportWithIssues := &output.Report{
		Summary: output.Summary{TotalIssues: 3},
	}
	sendWebhooks(context.Background(), cfg, opts, reportWithIssues)

	if callCount != 1 {
		t.Errorf("on_issues webhook should fire with issues, callCount = %d", callCount)
	}
}

func TestSendWebhooks_NeverTrigger(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Webhooks: []config.WebhookConfig{
			{
				Name:    "never-webhook",
				URL:     server.URL,
				Trigger: config.WebhookTriggerNever,
				Timeout: 10 * time.Second,
			},
		},
	}
	opts := &AnalyzeOptions{}

	report := &output.Report{
		Summary: output.Summary{TotalIssues: 10},
	}
	sendWebhooks(context.Background(), cfg, opts, report)

	if callCount != 0 {
		t.Errorf("never trigger webhook should not fire, callCount = %d", callCount)
	}
}

func TestSendWebhooks_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		Webhooks: []config.WebhookConfig{
			{
				Name:    "error-webhook",
				URL:     server.URL,
				Trigger: config.WebhookTriggerAlways,
				Timeout: 10 * time.Second,
			},
		},
	}
	opts := &AnalyzeOptions{}

	report := &output.Report{
		Summary: output.Summary{TotalIssues: 1},
	}

	// Should not panic, just log error
	sendWebhooks(context.Background(), cfg, opts, report)
}

func TestSendWebhooks_NoWebhooks(t *testing.T) {
	cfg := &config.Config{}
	opts := &AnalyzeOptions{}
	report := &output.Report{}

	// Should return immediately, no panic
	sendWebhooks(context.Background(), cfg, opts, report)
}

func TestCreateFormatter_Options(t *testing.T) {
	opts := &AnalyzeOptions{
		Output:  "text",
		Verbose: true,
		Quiet:   true,
	}

	formatter, err := createFormatter(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if formatter == nil {
		t.Error("expected formatter, got nil")
	}
}

func TestSendWebhooks_MultipleWebhooks(t *testing.T) {
	var callURLs []string

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callURLs = append(callURLs, "server1")
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callURLs = append(callURLs, "server2")
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	cfg := &config.Config{
		Webhooks: []config.WebhookConfig{
			{Name: "webhook1", URL: server1.URL, Trigger: config.WebhookTriggerAlways, Timeout: 10 * time.Second},
			{Name: "webhook2", URL: server2.URL, Trigger: config.WebhookTriggerAlways, Timeout: 10 * time.Second},
		},
	}
	opts := &AnalyzeOptions{}

	report := &output.Report{Summary: output.Summary{TotalIssues: 1}}
	sendWebhooks(context.Background(), cfg, opts, report)

	if len(callURLs) != 2 {
		t.Errorf("expected 2 webhook calls, got %d", len(callURLs))
	}
	if !strings.Contains(strings.Join(callURLs, ","), "server1") {
		t.Error("server1 was not called")
	}
	if !strings.Contains(strings.Join(callURLs, ","), "server2") {
		t.Error("server2 was not called")
	}
}
