// Package webhook provides HTTP client for sending analysis results to webhook endpoints.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"negalog/pkg/output"
)

// DefaultTimeout is the default HTTP request timeout.
const DefaultTimeout = 10 * time.Second

// Client sends analysis reports to webhook endpoints.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new webhook client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
	}
}

// SendOptions configures a webhook request.
type SendOptions struct {
	URL     string
	Token   string        // Bearer token (optional)
	Timeout time.Duration // Request timeout (uses DefaultTimeout if zero)
}

// Response contains the result of a webhook request.
type Response struct {
	StatusCode int
	Body       string
	Duration   time.Duration
	Error      error
}

// Success returns true if the webhook was sent successfully (2xx status).
func (r *Response) Success() bool {
	return r.Error == nil && r.StatusCode >= 200 && r.StatusCode < 300
}

// Send posts an analysis report to a webhook endpoint.
func (c *Client) Send(ctx context.Context, report *output.Report, opts SendOptions) *Response {
	start := time.Now()
	resp := &Response{}

	// Marshal report to JSON
	payload, err := json.Marshal(report)
	if err != nil {
		resp.Error = fmt.Errorf("failed to marshal report: %w", err)
		resp.Duration = time.Since(start)
		return resp
	}

	// Apply timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.URL, bytes.NewReader(payload))
	if err != nil {
		resp.Error = fmt.Errorf("failed to create request: %w", err)
		resp.Duration = time.Since(start)
		return resp
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "negalog-webhook")
	if opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+opts.Token)
	}

	// Send request
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		resp.Error = fmt.Errorf("request failed: %w", err)
		resp.Duration = time.Since(start)
		return resp
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1024*1024)) // Limit to 1MB
	if err != nil {
		resp.Error = fmt.Errorf("failed to read response: %w", err)
		resp.Duration = time.Since(start)
		return resp
	}

	resp.StatusCode = httpResp.StatusCode
	resp.Body = string(body)
	resp.Duration = time.Since(start)

	// Check for error status codes
	if resp.StatusCode >= 400 {
		resp.Error = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return resp
}
