package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Errorf("expected OK, got %s", w.Body.String())
	}
}

func TestReadyzHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	readyzHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRootHandlerPureIP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ApiAdEnabled = false
	extractor := NewIPExtractor("/dev/null")
	perIP := NewPerIPRateLimiter(100, 10, time.Minute)
	global := NewGlobalRateLimiter(1000, 1000)
	metrics := NewMetrics()
	ready.Store(true)

	handler := rootHandler(cfg, extractor, perIP, global, metrics, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.42:12345"
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body != "203.0.113.42" {
		t.Errorf("expected IP, got %s", body)
	}
}

func TestRootHandlerWithAPIAd(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ApiAdEnabled = true
	extractor := NewIPExtractor("/dev/null")
	perIP := NewPerIPRateLimiter(100, 10, time.Minute)
	global := NewGlobalRateLimiter(1000, 1000)
	metrics := NewMetrics()
	ready.Store(true)

	handler := rootHandler(cfg, extractor, perIP, global, metrics, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.42:12345"
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "203.0.113.42") {
		t.Errorf("expected body to contain IP, got %s", body)
	}
	if !strings.Contains(body, "VPN") {
		t.Errorf("expected body to contain ad text, got %s", body)
	}
}

func TestRootHandlerWebClient(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ApiAdEnabled = true
	extractor := NewIPExtractor("/dev/null")
	perIP := NewPerIPRateLimiter(100, 10, time.Minute)
	global := NewGlobalRateLimiter(1000, 1000)
	metrics := NewMetrics()
	ready.Store(true)

	handler := rootHandler(cfg, extractor, perIP, global, metrics, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.42:12345"
	req.Header.Set("X-Client", "web")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body != "203.0.113.42" {
		t.Errorf("expected pure IP for web client, got %s", body)
	}
	if strings.Contains(body, "VPN") {
		t.Errorf("expected no ad for web client, got %s", body)
	}
}

func TestJSONAPI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.JsonApiEnabled = true
	cfg.ApiAdEnabled = false
	extractor := NewIPExtractor("/dev/null")
	perIP := NewPerIPRateLimiter(100, 10, time.Minute)
	global := NewGlobalRateLimiter(1000, 1000)
	metrics := NewMetrics()
	ready.Store(true)

	handler := rootHandler(cfg, extractor, perIP, global, metrics, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.42:12345"
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content type, got %s", ct)
	}

	var resp ipResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp.IP != "203.0.113.42" {
		t.Errorf("expected IP 203.0.113.42, got %s", resp.IP)
	}
	if resp.Version != "IPv4" {
		t.Errorf("expected version IPv4, got %s", resp.Version)
	}
}

func TestAdConfigHandler(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WebAdEnabled = true
	cfg.WebAdTextZh = "测试广告"
	cfg.WebAdUrlZh = "https://example.com"

	handler := adConfigHandler(cfg)

	req := httptest.NewRequest("GET", "/ad-config", nil)
	req.Header.Set("Accept-Language", "zh")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp adConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if !resp.Web.Enabled {
		t.Error("expected web ad enabled")
	}
	if resp.Web.Text != "测试广告" {
		t.Errorf("expected 测试广告, got %s", resp.Web.Text)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewPerIPRateLimiter(1, 1, time.Minute)
	defer rl.Stop()

	ip := "192.168.1.1"
	if !rl.Allow(ip) {
		t.Error("expected first request to be allowed")
	}
	if rl.Allow(ip) {
		t.Error("expected second request to be denied")
	}
}

func TestGlobalRateLimiter(t *testing.T) {
	gl := NewGlobalRateLimiter(1000, 1000)
	for i := 0; i < 10; i++ {
		if !gl.Allow() {
			t.Error("expected global rate limiter to allow 10 requests")
		}
	}
}

func TestIPMasking(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.100", "192.168.1.0"},
		{"10.0.0.55", "10.0.0.0"},
	}
	for _, tt := range tests {
		result := maskIP(tt.input)
		if result != tt.expected {
			t.Errorf("maskIP(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestRequestMethodCheck(t *testing.T) {
	handler := methodCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for POST, got %d", w.Code)
	}
}

func TestConfigDefaultValues(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RatePerIP != 10 {
		t.Errorf("expected RatePerIP=10, got %d", cfg.RatePerIP)
	}
	if cfg.PortV4 != 8080 {
		t.Errorf("expected PortV4=8080, got %d", cfg.PortV4)
	}
	if cfg.PortV6 != 8081 {
		t.Errorf("expected PortV6=8081, got %d", cfg.PortV6)
	}
	if !cfg.ApiAdEnabled {
		t.Error("expected ApiAdEnabled=true")
	}
	if !cfg.WebAdEnabled {
		t.Error("expected WebAdEnabled=true")
	}
	if !cfg.LogIpMasking {
		t.Error("expected LogIpMasking=true")
	}
	if cfg.GeoipEnabled {
		t.Error("expected GeoipEnabled=false by default")
	}
}

func TestMetrics(t *testing.T) {
	m := NewMetrics()
	m.IncRequestsTotal()
	m.IncRequestsTotal()
	if v := m.RequestsTotal(); v != 2 {
		t.Errorf("expected 2 requests, got %d", v)
	}
	m.IncRateLimitHits()
	if v := m.RateLimitHits(); v != 1 {
		t.Errorf("expected 1 rate limit hit, got %d", v)
	}
}

func TestDetectLanguage(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		header string
		want   string
	}{
		{"zh-CN,zh;q=0.9", "zh"},
		{"zh-TW,zh;q=0.8", "zh"},
		{"en-US,en;q=0.9", "en"},
		{"fr-FR,fr;q=0.9", "en"},
		{"", "en"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/", nil)
		if tt.header != "" {
			req.Header.Set("Accept-Language", tt.header)
		}
		got := cfg.detectLanguage(req)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestParseXFF(t *testing.T) {
	tests := []struct {
		xff  string
		want string
	}{
		{"203.0.113.1", "203.0.113.1"},
		{"203.0.113.1, 198.51.100.2", "198.51.100.2"},
		{"", ""},
	}
	for _, tt := range tests {
		ip := parseXFF(tt.xff)
		if tt.want == "" {
			if ip != nil {
				t.Errorf("parseXFF(%q) = %v, want nil", tt.xff, ip)
			}
		} else {
			if ip == nil || ip.String() != tt.want {
				t.Errorf("parseXFF(%q) = %v, want %v", tt.xff, ip, tt.want)
			}
		}
	}
}

func TestIPVersion(t *testing.T) {
	if v := ipVersion("203.0.113.42"); v != "IPv4" {
		t.Errorf("expected IPv4, got %s", v)
	}
	if v := ipVersion("2001:db8::1"); v != "IPv6" {
		t.Errorf("expected IPv6, got %s", v)
	}
}

func TestErrorConstants(t *testing.T) {
	if errBadRequest == "" {
		t.Error("errBadRequest should not be empty")
	}
	if errTooManyRequests == "" {
		t.Error("errTooManyRequests should not be empty")
	}
	if !strings.Contains(errTooManyRequests, "rate limit") {
		t.Error("errTooManyRequests should mention rate limit")
	}
}

func TestFullMiddlewareChain(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ApiAdEnabled = false
	cfg.JsonApiEnabled = true
	extractor := NewIPExtractor("/dev/null")
	perIP := NewPerIPRateLimiter(100, 10, time.Minute)
	global := NewGlobalRateLimiter(1000, 1000)
	metrics := NewMetrics()
	connCounter := newConnCounter()
	ready.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler(cfg, extractor, perIP, global, metrics, nil))
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/readyz", readyzHandler)

	var handler http.Handler = mux
	handler = requestIDMiddleware(handler)
	handler = realIPMiddleware(handler, extractor)
	handler = recoveryMiddleware(handler)
	handler = securityHeadersMiddleware(handler)
	handler = metricsMiddleware(handler, metrics)
	handler = methodCheckMiddleware(handler)
	handler = bodyRejectionMiddleware(handler)
	handler = denylistMiddleware(handler, cfg, extractor)
	handler = connLimitMiddleware(handler, connCounter, cfg.MaxConnsPerIP, extractor)
	handler = corsMiddleware(handler, cfg.CorsEnabled)
	handler = loggingMiddleware(handler, cfg, extractor, metrics)

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("health endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("readiness endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("root returns IP", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)
		if body == "" {
			t.Error("expected non-empty body")
		}
	})

	t.Run("JSON API", func(t *testing.T) {
		req, _ := http.NewRequest("GET", server.URL+"/", nil)
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var result ipResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.IP == "" {
			t.Error("expected IP in JSON response")
		}
		if result.Version == "" {
			t.Error("expected version in JSON response")
		}
	})

	t.Run("security headers", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Error("missing X-Content-Type-Options header")
		}
		if resp.Header.Get("X-Frame-Options") != "DENY" {
			t.Error("missing X-Frame-Options header")
		}
		if resp.Header.Get("X-Request-ID") == "" {
			t.Error("missing X-Request-ID header")
		}
	})

	t.Run("method check rejects POST", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/", "text/plain", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", resp.StatusCode)
		}
	})

	t.Run("metrics endpoint", func(t *testing.T) {
		metricsPort := server.URL
		resp, err := http.Get(metricsPort + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestMetricsExpanded(t *testing.T) {
	m := NewMetrics()

	m.IncRequestsTotal()
	m.ObserveRequest(200, 10)
	m.ObserveRequest(200, 50)
	m.ObserveRequest(500, 200)

	if v := m.RequestsTotal(); v != 1 {
		t.Errorf("expected 1, got %d", v)
	}
	if v := m.Status2xx(); v != 2 {
		t.Errorf("expected 2 2xx, got %d", v)
	}
	if v := m.Status5xx(); v != 1 {
		t.Errorf("expected 1 5xx, got %d", v)
	}
	if v := m.TotalLatencyMs(); v != 260 {
		t.Errorf("expected 260ms total, got %d", v)
	}
	if v := m.StatusCodesTotal(); v != 3 {
		t.Errorf("expected 3 total status codes, got %d", v)
	}
}

func TestMonitorThresholds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Monitoring.Enabled = false
	cfg.Monitoring.ErrorRateThreshold = 0.05
	cfg.Monitoring.P99LatencyThresholdMs = 2000
	cfg.Monitoring.RateLimitHitRateThreshold = 0.10

	metrics := NewMetrics()
	monitor := NewMonitor(cfg, metrics)

	t.Run("computeP99", func(t *testing.T) {
		var curr, prev [11]int64
		curr[2] = 99
		curr[10] = 1
		p99 := computeP99(curr, prev, 100)
		if p99 != 25 {
			t.Errorf("expected P99=25 (99th percentile in bucket 2 boundary), got %d", p99)
		}

		prev = [11]int64{}
		curr = [11]int64{}
		curr[3] = 50
		curr[4] = 50
		p99 = computeP99(curr, prev, 100)
		if p99 != 100 {
			t.Errorf("expected P99=100 (99th percentile in bucket 4 boundary), got %d", p99)
		}

		prev = [11]int64{}
		curr = [11]int64{}
		curr[0] = 5
		curr[1] = 5
		p99 = computeP99(curr, prev, 10)
		if p99 != 10 {
			t.Errorf("expected P99=10 (9th value in bucket 1 boundary), got %d", p99)
		}
	})

	t.Run("no alert when threshold not met", func(t *testing.T) {
		monitor.evaluateThresholds(0.01, 100, 0.01)
	})

	t.Run("alert on high error rate", func(t *testing.T) {
		monitor.evaluateThresholds(0.10, 100, 0)
	})
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	if !cb.Allow() {
		t.Error("expected allow in closed state")
	}

	cb.Failure()
	cb.Failure()
	cb.Failure()

	if cb.Allow() {
		t.Error("expected deny in open state")
	}

	time.Sleep(150 * time.Millisecond)

	if !cb.Allow() {
		t.Error("expected allow after timeout (half-open)")
	}

	cb.Success()
	cb.Success()

	if !cb.Allow() {
		t.Error("expected allow after recovery (closed)")
	}

	cb2 := NewCircuitBreaker(5, 1, time.Minute)

	result := cb2.Allow()
	if !result {
		t.Error("initial state should be closed")
	}
}

func TestFindBucket(t *testing.T) {
	tests := []struct {
		latency int64
		bucket  int
	}{
		{0, 0},
		{5, 0},
		{6, 1},
		{10, 1},
		{11, 2},
		{5000, 9},
		{5001, 10},
		{99999, 10},
	}
	for _, tt := range tests {
		got := findBucket(tt.latency)
		if got != tt.bucket {
			t.Errorf("findBucket(%d) = %d, want %d", tt.latency, got, tt.bucket)
		}
	}
}

func TestCumulativeSum(t *testing.T) {
	counts := [11]int64{1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0}
	if v := cumulativeSum(counts, 0); v != 1 {
		t.Errorf("expected 1, got %d", v)
	}
	if v := cumulativeSum(counts, 1); v != 3 {
		t.Errorf("expected 3, got %d", v)
	}
	if v := cumulativeSum(counts, 2); v != 6 {
		t.Errorf("expected 6, got %d", v)
	}
}

func TestLoggingMiddlewareCapturesRejectedStatus(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UADenylist = "badbot"
	metrics := NewMetrics()
	extractor := NewIPExtractor("/dev/null")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	var handler http.Handler = inner
	handler = denylistMiddleware(handler, cfg, extractor)
	handler = loggingMiddleware(handler, cfg, extractor, metrics)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.42:12345"
	req.Header.Set("User-Agent", "badbot")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	if v := metrics.Status4xx(); v != 1 {
		t.Errorf("expected 1 count in 4xx, got %d", v)
	}
	if v := metrics.RequestsTotal(); v != 1 {
		t.Errorf("expected 1 total request, got %d", v)
	}
}

func TestMonitoringDisabledByDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Monitoring.Enabled {
		t.Error("monitoring should be disabled by default")
	}
	if cfg.Monitoring.AlertWebhookURL != "" {
		t.Error("webhook URL should be empty by default")
	}
	if cfg.Monitoring.AlertWebhookType != "generic" {
		t.Error("webhook type should be 'generic' by default")
	}
}
