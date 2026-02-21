package consolecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/quailyquaily/mistermorph/cmd/mistermorph/daemoncmd"
)

var errTaskNotFound = errors.New("task not found")

type daemonTaskClient struct {
	baseURL   string
	authToken string
	client    *http.Client
}

func newDaemonTaskClient(baseURL, authToken string) *daemonTaskClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	authToken = strings.TrimSpace(authToken)
	return &daemonTaskClient{
		baseURL:   baseURL,
		authToken: authToken,
		client:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *daemonTaskClient) readyBaseURL() error {
	if c == nil || strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("daemon server url is not configured")
	}
	return nil
}

func (c *daemonTaskClient) ready() error {
	if err := c.readyBaseURL(); err != nil {
		return err
	}
	if strings.TrimSpace(c.authToken) == "" {
		return fmt.Errorf("daemon server auth token is not configured")
	}
	return nil
}

func (c *daemonTaskClient) HealthMode(ctx context.Context) (string, error) {
	if err := c.readyBaseURL(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("daemon health http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("invalid daemon health response: %w", err)
	}
	return strings.ToLower(strings.TrimSpace(out.Mode)), nil
}

func (c *daemonTaskClient) Overview(ctx context.Context) (map[string]any, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/overview", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return c.legacyOverviewFromHealth(ctx)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("daemon overview http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		if looksLikeLegacyOverviewBody(raw) {
			return c.legacyOverviewFromHealth(ctx)
		}
		return nil, fmt.Errorf("invalid daemon overview response: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func (c *daemonTaskClient) legacyOverviewFromHealth(ctx context.Context) (map[string]any, error) {
	mode, err := c.HealthMode(ctx)
	if err != nil {
		return nil, err
	}
	channel := map[string]any{
		"configured":          false,
		"telegram_configured": false,
		"slack_configured":    false,
		"running":             "unknown",
		"telegram_running":    false,
		"slack_running":       false,
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "telegram":
		channel["configured"] = true
		channel["telegram_configured"] = true
		channel["running"] = "telegram"
		channel["telegram_running"] = true
	case "slack":
		channel["configured"] = true
		channel["slack_configured"] = true
		channel["running"] = "slack"
		channel["slack_running"] = true
	case "serve":
		channel["running"] = "none"
	default:
		channel["running"] = "unknown"
	}
	return map[string]any{
		"mode":    mode,
		"health":  "ok",
		"channel": channel,
	}, nil
}

func looksLikeLegacyOverviewBody(raw []byte) bool {
	text := strings.ToLower(strings.TrimSpace(string(raw)))
	return text == "" || text == "ok"
}

func (c *daemonTaskClient) Proxy(ctx context.Context, method, endpointPath string, body []byte) (int, []byte, error) {
	if err := c.ready(); err != nil {
		return 0, nil, err
	}
	endpointPath = strings.TrimSpace(endpointPath)
	if endpointPath == "" {
		endpointPath = "/"
	}
	if !strings.HasPrefix(endpointPath, "/") {
		endpointPath = "/" + endpointPath
	}
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, strings.TrimSpace(method), c.baseURL+endpointPath, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	return resp.StatusCode, raw, nil
}

func (c *daemonTaskClient) List(ctx context.Context, status daemoncmd.TaskStatus, limit int) ([]daemoncmd.TaskInfo, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	q := url.Values{}
	if strings.TrimSpace(string(status)) != "" {
		q.Set("status", strings.TrimSpace(string(status)))
	}
	q.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := c.baseURL + "/tasks"
	if qs := q.Encode(); qs != "" {
		endpoint = endpoint + "?" + qs
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("daemon http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out struct {
		Items []daemoncmd.TaskInfo `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("invalid daemon response: %w", err)
	}
	return out.Items, nil
}

func (c *daemonTaskClient) Get(ctx context.Context, id string) (*daemoncmd.TaskInfo, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("missing task id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/tasks/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, errTaskNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("daemon http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out daemoncmd.TaskInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("invalid daemon response: %w", err)
	}
	return &out, nil
}
