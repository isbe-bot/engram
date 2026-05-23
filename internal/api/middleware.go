package api

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", clientID(r),
		)
	})
}

type rateLimiter struct {
	limit   int
	window  time.Duration
	now     func() time.Time
	mu      sync.Mutex
	clients map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		now:     time.Now,
		clients: map[string][]time.Time{},
	}
}

func (l *rateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	if strings.TrimSpace(key) == "" {
		key = "unknown"
	}
	now := l.now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.clients[key]
	kept := recent[:0]
	for _, ts := range recent {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= l.limit {
		l.clients[key] = kept
		return false
	}
	kept = append(kept, now)
	l.clients[key] = kept
	return true
}

func clientID(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || strings.TrimSpace(host) == "" {
		return r.RemoteAddr
	}
	return host
}
