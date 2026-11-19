package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

const counterKey = "visits"

// redisLogger routes go-redis' own log lines into slog (stdout, JSON),
// so the library never writes to stderr.
type redisLogger struct{}

func (redisLogger) Printf(_ context.Context, format string, v ...interface{}) {
	slog.Warn(fmt.Sprintf(format, v...), "source", "go-redis")
}

// statusRecorder remembers the status code a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs exactly one JSON event per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	redis.SetLogger(redisLogger{})

	port := getenv("PORT", "8080")
	greeting := getenv("GREETING", "hello from my container")
	redisURL := getenv("REDIS_URL", "redis://localhost:6379/0")

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", "error", err.Error())
		os.Exit(1)
	}
	opts.DialTimeout = time.Second // fail fast when Redis is unreachable
	opts.MaxRetries = -1           // no hidden retries: one attempt, one answer
	rdb := redis.NewClient(opts)
	defer rdb.Close()

	// Admin process: `hello reset` zeroes the counter and exits.
	if len(os.Args) > 1 && os.Args[1] == "reset" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rdb.Set(ctx, counterKey, 0, 0).Err(); err != nil {
			slog.Error("reset failed", "error", err.Error())
			os.Exit(1)
		}
		slog.Info("counter reset")
		return
	}

	hostname, _ := os.Hostname()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		n, err := rdb.Incr(ctx, counterKey).Result()
		if err != nil {
			slog.Error("redis unavailable", "error", err.Error())
			http.Error(w, "backing service unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, "%s (visit %d, served by %s)\n", greeting, n, hostname)
	})
	// Liveness is about this process only: it must never ask Redis.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
			fmt.Fprintln(w, "done")
		case <-r.Context().Done():
		}
	})

	srv := &http.Server{Addr: ":" + port, Handler: logRequests(mux)}

	// Graceful shutdown: SIGTERM (Kubernetes) or SIGINT (Ctrl-C).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown failed", "error", err.Error())
		}
	}()

	slog.Info("listening", "port", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err.Error())
		os.Exit(1)
	}
	<-done // ListenAndServe returns at once on Shutdown; wait for in-flight requests
	slog.Info("stopped")
}