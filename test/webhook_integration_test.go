package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestIntegration_WebhookSite tests webhook functionality against webhook.site
// This test is skipped by default. Set WEBHOOK_INTEGRATION_TEST=1 to run.
func TestIntegration_WebhookSite(t *testing.T) {
	if os.Getenv("WEBHOOK_INTEGRATION_TEST") != "1" {
		t.Skip("Skipping webhook.site integration test. Set WEBHOOK_INTEGRATION_TEST=1 to run")
	}

	// Change to project root
	chdir(t)
	binary := "./bin/negalog"

	if _, err := os.Stat(binary); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first", binary)
	}

	// Step 1: Create a new webhook.site token
	t.Log("Creating webhook.site token...")
	token, err := createWebhookSiteToken()
	if err != nil {
		t.Fatalf("Failed to create webhook.site token: %v", err)
	}
	t.Logf("Created webhook URL: https://webhook.site/%s", token.UUID)

	// Cleanup: delete token when done
	defer func() {
		if err := deleteWebhookSiteToken(token.UUID); err != nil {
			t.Logf("Warning: failed to delete token: %v", err)
		}
	}()

	// Step 2: Use the webhook_e2e.yaml config with CLI webhook override
	webhookURL := fmt.Sprintf("https://webhook.site/%s", token.UUID)
	configFile := filepath.Join("testdata", "configs", "webhook_e2e.yaml")

	// Step 3: Run negalog analyze with webhook URL via CLI flag
	t.Log("Running negalog analyze...")
	cmd := exec.Command(binary, "analyze", configFile,
		"--webhook-url", webhookURL,
		"--webhook-trigger", "always")
	output, _ := cmd.CombinedOutput() // Don't fail on exit code - we expect issues to be found
	t.Logf("negalog output:\n%s", string(output))

	// Step 4: Wait a moment for webhook to be received
	t.Log("Waiting for webhook delivery...")
	time.Sleep(2 * time.Second)

	// Step 5: Check webhook.site for received requests
	t.Log("Checking webhook.site for received payload...")
	requests, err := getWebhookSiteRequests(token.UUID)
	if err != nil {
		t.Fatalf("Failed to get webhook requests: %v", err)
	}

	if len(requests.Data) == 0 {
		t.Fatal("No webhook requests received at webhook.site")
	}

	t.Logf("Received %d webhook request(s)", len(requests.Data))

	// Step 6: Verify the payload
	req := requests.Data[0]
	contentType := req.GetHeader("content-type")
	t.Logf("Request method: %s", req.Method)
	t.Logf("Content-Type: %s", contentType)

	if req.Method != "POST" {
		t.Errorf("Expected POST method, got %s", req.Method)
	}

	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected application/json content-type, got %s", contentType)
	}

	// Parse and verify the payload
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(req.Content), &payload); err != nil {
		t.Fatalf("Failed to parse webhook payload: %v", err)
	}

	t.Logf("Webhook payload: %s", req.Content)

	// Verify payload structure (JSON fields are capitalized: Summary, Metadata)
	if _, ok := payload["Summary"]; !ok {
		t.Error("Payload missing 'Summary' field")
	}
	if _, ok := payload["Metadata"]; !ok {
		t.Error("Payload missing 'Metadata' field")
	}

	// Verify issues were detected
	if summary, ok := payload["Summary"].(map[string]interface{}); ok {
		if totalIssues, ok := summary["TotalIssues"].(float64); ok {
			if totalIssues == 0 {
				t.Error("Expected issues to be detected, but TotalIssues is 0")
			} else {
				t.Logf("Webhook reported %v issues", totalIssues)
			}
		}
	}

	t.Log("Integration test passed!")
}

// webhook.site API types
type webhookSiteToken struct {
	UUID string `json:"uuid"`
}

type webhookSiteRequests struct {
	Data []webhookSiteRequest `json:"data"`
}

type webhookSiteRequest struct {
	UUID      string          `json:"uuid"`
	Method    string          `json:"method"`
	Content   string          `json:"content"`
	Headers   json.RawMessage `json:"headers"`
	CreatedAt string          `json:"created_at"`
}

func (r *webhookSiteRequest) GetHeader(name string) string {
	var headers map[string]interface{}
	if err := json.Unmarshal(r.Headers, &headers); err != nil {
		return ""
	}
	if val, ok := headers[name]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []interface{}:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

func createWebhookSiteToken() (*webhookSiteToken, error) {
	resp, err := http.Post("https://webhook.site/token", "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("POST /token failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var token webhookSiteToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &token, nil
}

func getWebhookSiteRequests(uuid string) (*webhookSiteRequests, error) {
	url := fmt.Sprintf("https://webhook.site/token/%s/requests", uuid)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET requests failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var requests webhookSiteRequests
	if err := json.NewDecoder(resp.Body).Decode(&requests); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &requests, nil
}

func deleteWebhookSiteToken(uuid string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("https://webhook.site/token/%s", uuid), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
