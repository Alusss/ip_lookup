package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"
const realIPKey contextKey = "real_ip"

func getRealIP(r *http.Request, extractor *IPExtractor) (net.IP, error) {
	if ip, ok := r.Context().Value(realIPKey).(net.IP); ok && ip != nil {
		return ip, nil
	}
	return extractor.RealIP(r)
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func realIPMiddleware(next http.Handler, extractor *IPExtractor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realIP, err := extractor.RealIP(r)
		if err == nil && realIP != nil {
			ctx := context.WithValue(r.Context(), realIPKey, realIP)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

const connCounterShards = 16

type connCounterShard struct {
	mu   sync.Mutex
	conn map[string]int
}

type connCounter struct {
	shards [connCounterShards]*connCounterShard
}

func newConnCounter() *connCounter {
	cc := &connCounter{}
	for i := 0; i < connCounterShards; i++ {
		cc.shards[i] = &connCounterShard{conn: make(map[string]int)}
	}
	return cc
}

func (cc *connCounter) shardIndex(ip string) int {
	var h uint32
	for i := 0; i < len(ip); i++ {
		h = h*31 + uint32(ip[i])
	}
	return int(h % connCounterShards)
}

func (cc *connCounter) TryAcquire(ip string, max int) bool {
	s := cc.shards[cc.shardIndex(ip)]
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn[ip] >= max {
		return false
	}
	s.conn[ip]++
	return true
}

func (cc *connCounter) Release(ip string) {
	s := cc.shards[cc.shardIndex(ip)]
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn[ip] > 1 {
		s.conn[ip]--
	} else {
		delete(s.conn, ip)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

func loggingMiddleware(next http.Handler, cfg *Config, extractor *IPExtractor, metrics *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		realIP, _ := getRealIP(r, extractor)
		ipStr := ""
		if realIP != nil {
			ipStr = realIP.String()
			if cfg.LogIpMasking {
				ipStr = maskIP(ipStr)
			}
		}

		latency := time.Since(start).Milliseconds()

		requestID := ""
		if id, ok := r.Context().Value(requestIDKey).(string); ok {
			requestID = id
		}

		logAttrs := []slog.Attr{
			slog.String("ip", ipStr),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.Int64("latency_ms", latency),
			slog.String("request_id", requestID),
			slog.String("ua", truncateUA(r.UserAgent())),
		}

		slog.LogAttrs(nil, slog.LevelInfo, "request", logAttrs...)

		metrics.IncRequestsTotal()
		metrics.ObserveRequest(rw.statusCode, latency)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logError(nil, "panic recovered", "panic", rec)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func bodyRejectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > 0 {
			http.Error(w, errBodyNotAllowed, http.StatusBadRequest)
			return
		}
		if r.Body != nil {
			r.Body.Close()
		}
		next.ServeHTTP(w, r)
	})
}

func methodCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
			return
		}
		if len(r.URL.Path) > 256 {
			http.Error(w, errURLTooLong, http.StatusRequestURITooLong)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler, enabled bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "X-Client")
			w.Header().Set("Access-Control-Expose-Headers", "X-Ad-Enabled, X-Ad-Text, X-Ad-URL")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func connLimitMiddleware(next http.Handler, counter *connCounter, max int, extractor *IPExtractor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realIP, err := getRealIP(r, extractor)
		if err != nil {
			http.Error(w, errBadRequest, http.StatusBadRequest)
			return
		}
		ipStr := realIP.String()

		if !counter.TryAcquire(ipStr, max) {
			http.Error(w, errTooManyRequests, http.StatusTooManyRequests)
			return
		}
		defer counter.Release(ipStr)

		next.ServeHTTP(w, r)
	})
}

func denylistMiddleware(next http.Handler, cfg *Config, extractor *IPExtractor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		realIP, err := getRealIP(r, extractor)
		if err == nil && realIP != nil {
			ipStr := realIP.String()
			for _, denied := range cfg.GetDenylistIPs() {
				if ipStr == denied {
					logWarn(r, "denylist ip matched", "ip", ipStr, "rule", denied, "ua", truncateUA(r.UserAgent()))
					http.Error(w, errForbidden, http.StatusForbidden)
					return
				}
				if strings.Contains(denied, "/") {
					_, cidr, err := net.ParseCIDR(denied)
					if err == nil && cidr.Contains(realIP) {
						logWarn(r, "denylist cidr matched", "ip", ipStr, "rule", denied, "ua", truncateUA(r.UserAgent()))
						http.Error(w, errForbidden, http.StatusForbidden)
						return
					}
				}
			}
		}

		ua := r.UserAgent()
		for _, deniedUA := range cfg.GetDenylistUAs() {
			if strings.Contains(strings.ToLower(ua), strings.ToLower(deniedUA)) {
				logWarn(r, "denylist ua matched", "ua", truncateUA(ua), "rule", deniedUA)
				http.Error(w, errForbidden, http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func maskIP(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}

	if parsed.To4() != nil {
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			return parts[0] + "." + parts[1] + "." + parts[2] + ".0"
		}
		return ip
	}

	parts := strings.Split(ip, ":")
	if len(parts) > 4 {
		return strings.Join(parts[:4], ":") + "::"
	}
	return ip
}

func truncateUA(ua string) string {
	if len(ua) > 128 {
		return ua[:128]
	}
	return ua
}

func metricsMiddleware(next http.Handler, metrics *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.IncInflight()
		defer metrics.DecInflight()
		next.ServeHTTP(w, r)
	})
}
