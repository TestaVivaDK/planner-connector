package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

const graphBase = "https://graph.microsoft.com/v1.0"

// TokenProvider abstracts token acquisition so the graph client
// does not depend on the auth package directly.
type TokenProvider interface {
	GetToken() (string, error)
}

// Client is a thin wrapper around the Microsoft Graph REST API.
type Client struct {
	baseURL string
	auth    TokenProvider
	http    *http.Client
}

// NewClient creates a Graph API client with a 30-second timeout.
func NewClient(auth TokenProvider) *Client {
	return &Client{
		baseURL: graphBase,
		auth:    auth,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// buildURL constructs a full URL from a path and optional query parameters.
func (c *Client) buildURL(path string, queryParams map[string]string) string {
	u := c.baseURL + path
	if len(queryParams) == 0 {
		return u
	}
	var parts []string
	for k, v := range queryParams {
		parts = append(parts, k+"="+url.QueryEscape(v))
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + strings.Join(parts, "&")
}

// Get performs a GET request. If path starts with "http", it is used as-is
// (useful for @odata.nextLink pagination).
func (c *Client) Get(path string, queryParams map[string]string, extraHeaders map[string]string) (json.RawMessage, error) {
	var reqURL string
	if strings.HasPrefix(path, "http") {
		reqURL = path
	} else {
		reqURL = c.buildURL(path, queryParams)
	}
	return c.doRequest(http.MethodGet, reqURL, nil, "", extraHeaders)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(path string, body any) (json.RawMessage, error) {
	reqURL := c.baseURL + path
	return c.doRequest(http.MethodPost, reqURL, body, "", nil)
}

// Patch performs a PATCH request with a JSON body and an If-Match ETag header.
func (c *Client) Patch(path string, body any, etag string) (json.RawMessage, error) {
	reqURL := c.baseURL + path
	return c.doRequest(http.MethodPatch, reqURL, body, etag, nil)
}

// Delete performs a DELETE request with an If-Match ETag header.
func (c *Client) Delete(path string, etag string) (json.RawMessage, error) {
	reqURL := c.baseURL + path
	return c.doRequest(http.MethodDelete, reqURL, nil, etag, nil)
}

// GetEtag fetches a resource and returns its @odata.etag value.
func (c *Client) GetEtag(path string) (string, error) {
	raw, err := c.Get(path, nil, nil)
	if err != nil {
		return "", err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("failed to parse response for etag: %w", err)
	}
	etagRaw, ok := obj["@odata.etag"]
	if !ok {
		return "", fmt.Errorf("no @odata.etag found on resource at %s", path)
	}
	var etag string
	if err := json.Unmarshal(etagRaw, &etag); err != nil {
		return "", fmt.Errorf("failed to parse @odata.etag value: %w", err)
	}
	return etag, nil
}

func (c *Client) doRequest(method, reqURL string, body any, etag string, extraHeaders map[string]string) (json.RawMessage, error) {
	return c.doRequestWithRetry(method, reqURL, body, etag, extraHeaders)
}

func (c *Client) doRequestWithRetry(method, reqURL string, body any, etag string, extraHeaders map[string]string) (json.RawMessage, error) {
	token, err := c.auth.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	if logger.Log != nil {
		logger.Log.Info(fmt.Sprintf("[GRAPH] %s %s", method, reqURL))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Retry once on 429 (throttled).
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := 1 // default 1 second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if parsed, err := strconv.Atoi(ra); err == nil {
				retryAfter = parsed
			}
		}
		if logger.Log != nil {
			logger.Log.Warn(fmt.Sprintf("Throttled by Graph API, retrying after %ds", retryAfter))
		}
		time.Sleep(time.Duration(retryAfter) * time.Second)

		// Rebuild request for retry (body may have been consumed).
		var retryBody io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			retryBody = bytes.NewReader(b)
		}
		retryReq, err := http.NewRequest(method, reqURL, retryBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create retry request: %w", err)
		}
		retryReq.Header.Set("Authorization", "Bearer "+token)
		retryReq.Header.Set("Content-Type", "application/json")
		if etag != "" {
			retryReq.Header.Set("If-Match", etag)
		}
		for k, v := range extraHeaders {
			retryReq.Header.Set(k, v)
		}

		resp, err = c.http.Do(retryReq)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
		defer resp.Body.Close()
	}

	// 412 Precondition Failed — ETag mismatch.
	if resp.StatusCode == http.StatusPreconditionFailed {
		return nil, fmt.Errorf("resource was modified by another user (ETag mismatch)")
	}

	// Other error status codes.
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		message := fmt.Sprintf("Graph API error %d", resp.StatusCode)
		var parsed struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(errBody, &parsed) == nil && parsed.Error.Message != "" {
			message = fmt.Sprintf("%s: %s", parsed.Error.Code, parsed.Error.Message)
		}
		return nil, fmt.Errorf("%s", message)
	}

	// Empty body (e.g. 204 No Content).
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(respBody) == 0 {
		return json.RawMessage(`{"success":true}`), nil
	}

	return json.RawMessage(respBody), nil
}
