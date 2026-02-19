package daemonruntime

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SubmitFunc func(ctx context.Context, req SubmitTaskRequest) (SubmitTaskResponse, error)

type badRequestError struct {
	msg string
}

func (e badRequestError) Error() string {
	return strings.TrimSpace(e.msg)
}

func BadRequest(msg string) error {
	return badRequestError{msg: msg}
}

func badRequestMessage(err error) (string, bool) {
	var reqErr badRequestError
	if errors.As(err, &reqErr) {
		return strings.TrimSpace(reqErr.msg), true
	}
	return "", false
}

type RoutesOptions struct {
	Mode          string
	AuthToken     string
	TaskReader    TaskReader
	Submit        SubmitFunc
	HealthEnabled bool
}

func RegisterRoutes(mux *http.ServeMux, opts RoutesOptions) {
	if mux == nil {
		return
	}
	mode := strings.TrimSpace(opts.Mode)
	authToken := strings.TrimSpace(opts.AuthToken)
	reader := opts.TaskReader
	submit := opts.Submit

	if opts.HealthEnabled {
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead:
			default:
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			payload := map[string]any{
				"ok":   true,
				"time": time.Now().Format(time.RFC3339Nano),
			}
			if mode != "" {
				payload["mode"] = mode
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodHead {
				return
			}
			_ = json.NewEncoder(w).Encode(payload)
		})
	}

	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, authToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if reader == nil {
				http.Error(w, "task reader is unavailable", http.StatusServiceUnavailable)
				return
			}
			rawStatus := strings.TrimSpace(r.URL.Query().Get("status"))
			status, ok := ParseTaskStatus(rawStatus)
			if !ok {
				http.Error(w, "invalid status", http.StatusBadRequest)
				return
			}
			limit := 20
			if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
				parsed, err := strconv.Atoi(rawLimit)
				if err != nil || parsed <= 0 {
					http.Error(w, "invalid limit", http.StatusBadRequest)
					return
				}
				limit = parsed
			}
			items := reader.List(status, limit)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
			return

		case http.MethodPost:
			if submit == nil {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req SubmitTaskRequest
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			req.Task = strings.TrimSpace(req.Task)
			if req.Task == "" {
				http.Error(w, "missing task", http.StatusBadRequest)
				return
			}
			resp, err := submit(r.Context(), req)
			if err != nil {
				if msg, ok := badRequestMessage(err); ok {
					http.Error(w, msg, http.StatusBadRequest)
					return
				}
				http.Error(w, strings.TrimSpace(err.Error()), http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !checkAuth(r, authToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if reader == nil {
			http.Error(w, "task reader is unavailable", http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/tasks/"))
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		info, ok := reader.Get(id)
		if !ok || info == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	})
}

type ServerOptions struct {
	Listen string
	Routes RoutesOptions
}

func StartServer(ctx context.Context, logger *slog.Logger, opts ServerOptions) (*http.Server, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	listen := strings.TrimSpace(opts.Listen)
	if listen == "" {
		return nil, errors.New("empty daemon listen address")
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, opts.Routes)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		cancel()
	}()

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("daemon_server_error", "addr", listen, "error", err.Error())
		}
	}()

	logger.Info("daemon_server_start",
		"addr", listen,
		"mode", strings.TrimSpace(opts.Routes.Mode),
		"health_enabled", opts.Routes.HealthEnabled,
		"tasks_path", "/tasks",
	)
	return srv, nil
}

func checkAuth(r *http.Request, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("Authorization"))
	want := "Bearer " + token
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func IsContextDeadline(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "context canceled")
}

func TruncateUTF8(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return string(runes[:maxChars])
}

func BuildTaskID(prefix string, parts ...any) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "task"
	}
	buf := make([]string, 0, len(parts)+1)
	buf = append(buf, prefix)
	for _, part := range parts {
		buf = append(buf, sanitizeTaskIDPart(fmt.Sprint(part)))
	}
	return strings.Join(buf, "_")
}

func sanitizeTaskIDPart(part string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return "x"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_", "?", "_", "#", "_", "&", "_", "=", "_", ".", "_")
	part = replacer.Replace(part)
	part = strings.Trim(part, "_")
	if part == "" {
		return "x"
	}
	return part
}
