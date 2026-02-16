package slackcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type slackAPI struct {
	http     *http.Client
	baseURL  string
	botToken string
	appToken string
}

func newSlackAPI(httpClient *http.Client, baseURL, botToken, appToken string) *slackAPI {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL = strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if baseURL == "" {
		baseURL = "https://slack.com/api"
	}
	return &slackAPI{
		http:     httpClient,
		baseURL:  baseURL,
		botToken: strings.TrimSpace(botToken),
		appToken: strings.TrimSpace(appToken),
	}
}

type slackAuthTestResult struct {
	TeamID  string
	UserID  string
	BotID   string
	URL     string
	Team    string
	User    string
	IsOwner bool
}

type slackAuthTestResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	TeamID  string `json:"team_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	BotID   string `json:"bot_id,omitempty"`
	URL     string `json:"url,omitempty"`
	Team    string `json:"team,omitempty"`
	User    string `json:"user,omitempty"`
	IsOwner bool   `json:"is_owner,omitempty"`
}

func (api *slackAPI) authTest(ctx context.Context) (slackAuthTestResult, error) {
	if api == nil {
		return slackAuthTestResult{}, fmt.Errorf("slack api is not initialized")
	}
	body, status, _, err := api.postAuthJSON(ctx, api.botToken, "/auth.test", nil)
	if err != nil {
		return slackAuthTestResult{}, err
	}
	if status < 200 || status >= 300 {
		return slackAuthTestResult{}, fmt.Errorf("slack auth.test http %d", status)
	}
	var out slackAuthTestResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return slackAuthTestResult{}, err
	}
	if !out.OK {
		code := strings.TrimSpace(out.Error)
		if code == "" {
			code = "unknown_error"
		}
		return slackAuthTestResult{}, fmt.Errorf("slack auth.test failed: %s", code)
	}
	return slackAuthTestResult{
		TeamID:  strings.TrimSpace(out.TeamID),
		UserID:  strings.TrimSpace(out.UserID),
		BotID:   strings.TrimSpace(out.BotID),
		URL:     strings.TrimSpace(out.URL),
		Team:    strings.TrimSpace(out.Team),
		User:    strings.TrimSpace(out.User),
		IsOwner: out.IsOwner,
	}, nil
}

type slackOpenConnectionResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	URL   string `json:"url,omitempty"`
}

func (api *slackAPI) openSocketURL(ctx context.Context) (string, error) {
	if api == nil {
		return "", fmt.Errorf("slack api is not initialized")
	}
	body, status, _, err := api.postAuthJSON(ctx, api.appToken, "/apps.connections.open", nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("slack apps.connections.open http %d", status)
	}
	var out slackOpenConnectionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if !out.OK {
		code := strings.TrimSpace(out.Error)
		if code == "" {
			code = "unknown_error"
		}
		return "", fmt.Errorf("slack apps.connections.open failed: %s", code)
	}
	url := strings.TrimSpace(out.URL)
	if url == "" {
		return "", fmt.Errorf("slack apps.connections.open returned empty url")
	}
	return url, nil
}

func (api *slackAPI) connectSocket(ctx context.Context) (*websocket.Conn, error) {
	url, err := api.openSocketURL(ctx)
	if err != nil {
		return nil, err
	}
	dialer := *websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

type slackPostMessageRequest struct {
	Channel  string `json:"channel"`
	Text     string `json:"text"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

type slackPostMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	TS    string `json:"ts,omitempty"`
}

func (api *slackAPI) postMessage(ctx context.Context, channelID, text, threadTS string) error {
	channelID = strings.TrimSpace(channelID)
	text = strings.TrimSpace(text)
	threadTS = strings.TrimSpace(threadTS)
	if channelID == "" {
		return fmt.Errorf("channel_id is required")
	}
	if text == "" {
		return fmt.Errorf("text is required")
	}
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, status, headers, err := api.postAuthJSON(ctx, api.botToken, "/chat.postMessage", slackPostMessageRequest{
			Channel:  channelID,
			Text:     text,
			ThreadTS: threadTS,
		})
		if err != nil {
			lastErr = err
		} else {
			var out slackPostMessageResponse
			if parseErr := json.Unmarshal(body, &out); parseErr != nil {
				lastErr = parseErr
			} else if status < 200 || status >= 300 {
				lastErr = fmt.Errorf("slack chat.postMessage http %d", status)
			} else if out.OK {
				return nil
			} else {
				code := strings.TrimSpace(out.Error)
				if code == "" {
					code = "unknown_error"
				}
				lastErr = fmt.Errorf("slack chat.postMessage failed: %s", code)
			}
		}

		if attempt >= maxAttempts {
			break
		}
		wait, retryable := slackRetryDelay(status, headers, attempt)
		if !retryable {
			break
		}
		if err := sleepWithContext(ctx, wait); err != nil {
			return err
		}
	}
	return lastErr
}

func slackRetryDelay(status int, headers http.Header, attempt int) (time.Duration, bool) {
	switch {
	case status == http.StatusTooManyRequests:
		retryAfter := strings.TrimSpace(headers.Get("Retry-After"))
		if retryAfter == "" {
			return 1 * time.Second, true
		}
		secs, err := strconv.Atoi(retryAfter)
		if err != nil || secs <= 0 {
			return 1 * time.Second, true
		}
		return time.Duration(secs) * time.Second, true
	case status >= 500 && status <= 599:
		switch attempt {
		case 1:
			return 300 * time.Millisecond, true
		case 2:
			return 1 * time.Second, true
		default:
			return 2 * time.Second, true
		}
	default:
		return 0, false
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (api *slackAPI) postAuthJSON(ctx context.Context, token, path string, payload any) ([]byte, int, http.Header, error) {
	if api == nil || api.http == nil {
		return nil, 0, nil, fmt.Errorf("slack api is not initialized")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, 0, nil, fmt.Errorf("slack token is required")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, 0, nil, fmt.Errorf("slack api path is required")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.baseURL+path, body)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := api.http.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	raw, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, resp.StatusCode, resp.Header, readErr
	}
	return raw, resp.StatusCode, resp.Header, nil
}
