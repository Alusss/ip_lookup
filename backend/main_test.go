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
	extractor := NewIPExtractor("/dev/null", 0)
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
	extractor := NewIPExtractor("/dev/null", 0)
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
	extractor := NewIPExtractor("/dev/null", 0)
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
	extractor := NewIPExtractor("/dev/null", 0)
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

func TestRootJSONWithAd(t *testing.T) {
	cfg := DefaultConfig()
	cfg.JsonApiEnabled = true
	cfg.ApiAdEnabled = true
	extractor := NewIPExtractor("/dev/null", 0)
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
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp ipResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp.Ad == nil {
		t.Fatal("expected ad field in / JSON response, got nil")
	}
	if resp.Ad.Text == "" {
		t.Error("expected non-empty ad text")
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

func TestAllHandlerAd(t *testing.T) {
	extractor := NewIPExtractor("/dev/null", 0)
	perIP := NewPerIPRateLimiter(100, 10, time.Minute)
	global := NewGlobalRateLimiter(1000, 1000)
	metrics := NewMetrics()
	ready.Store(true)

	t.Run("ad enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AllApiEnabled = true
		cfg.ApiAdEnabled = true
		handler := allHandler(cfg, extractor, perIP, global, metrics, nil)

		req := httptest.NewRequest("GET", "/all", nil)
		req.RemoteAddr = "203.0.113.42:12345"
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp ipResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if resp.Ad == nil {
			t.Fatal("expected ad field in /all response, got nil")
		}
		if resp.Ad.Text == "" {
			t.Error("expected non-empty ad text")
		}
		if resp.Ad.URL == "" {
			t.Error("expected non-empty ad url")
		}
	})

	t.Run("ad disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AllApiEnabled = true
		cfg.ApiAdEnabled = false
		handler := allHandler(cfg, extractor, perIP, global, metrics, nil)

		req := httptest.NewRequest("GET", "/all", nil)
		req.RemoteAddr = "203.0.113.42:12345"
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp ipResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if resp.Ad != nil {
			t.Errorf("expected no ad field when api_ad_enabled=false, got %+v", resp.Ad)
		}
	})
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
		{"zh-Hans,en;q=0.8", "zh"},
		{"zh-SG,en;q=0.8", "zh"},
		{"zh", "zh"},
		{"zh-TW,zh;q=0.8", "en"},
		{"zh-HK,en;q=0.9", "en"},
		{"zh-Hant,en;q=0.9", "en"},
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
	extractor := NewIPExtractor("/dev/null", 0)
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
	extractor := NewIPExtractor("/dev/null", 0)

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
	if len(cfg.Monitoring.WebhookConfigs) != 0 {
		t.Error("webhook_configs should be empty by default")
	}
}

func TestBuildPayloadFiring(t *testing.T) {
	cfg := DefaultConfig()
	monitor := NewMonitor(cfg, NewMetrics())

	ts := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	payload := monitor.buildPayload("error_rate", "Error rate 8.50% exceeds threshold 5.00%", "0.0850", "0.0500", ts, "firing")

	if payload.Version != "4" {
		t.Errorf("expected version 4, got %s", payload.Version)
	}
	if payload.Status != "firing" {
		t.Errorf("expected status firing, got %s", payload.Status)
	}
	if payload.Receiver != "ip-lookup" {
		t.Errorf("expected receiver ip-lookup, got %s", payload.Receiver)
	}
	if len(payload.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(payload.Alerts))
	}
	alert := payload.Alerts[0]
	if alert.Status != "firing" {
		t.Errorf("expected alert status firing, got %s", alert.Status)
	}
	if alert.Labels["alertname"] != "error_rate" {
		t.Errorf("expected alertname error_rate, got %s", alert.Labels["alertname"])
	}
	if alert.Labels["severity"] != "warning" {
		t.Errorf("expected severity warning, got %s", alert.Labels["severity"])
	}
	if alert.Annotations["summary"] != "Error rate 8.50% exceeds threshold 5.00%" {
		t.Errorf("unexpected summary: %s", alert.Annotations["summary"])
	}
	if alert.StartsAt != "2026-07-24T12:00:00Z" {
		t.Errorf("expected startsAt 2026-07-24T12:00:00Z, got %s", alert.StartsAt)
	}
	if alert.EndsAt != "0001-01-01T00:00:00Z" {
		t.Errorf("expected endsAt zero time for firing, got %s", alert.EndsAt)
	}
	if alert.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}
}

func TestBuildPayloadResolved(t *testing.T) {
	cfg := DefaultConfig()
	monitor := NewMonitor(cfg, NewMetrics())

	ts := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	payload := monitor.buildPayload("error_rate", "Error rate resolved", "0.01", "0.05", ts, "resolved")

	if payload.Status != "resolved" {
		t.Errorf("expected status resolved, got %s", payload.Status)
	}
	if len(payload.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(payload.Alerts))
	}
	alert := payload.Alerts[0]
	if alert.Status != "resolved" {
		t.Errorf("expected alert status resolved, got %s", alert.Status)
	}
	if alert.EndsAt == "0001-01-01T00:00:00Z" {
		t.Error("expected non-zero endsAt for resolved alert")
	}
}

func TestMonitorFiringAndResolved(t *testing.T) {
	receivedCh := make(chan alertmanagerPayload, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p alertmanagerPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		receivedCh <- p
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Monitoring.WebhookConfigs = []WebhookConfig{
		{URL: server.URL},
	}
	monitor := NewMonitor(cfg, NewMetrics())

	monitor.checkThreshold("error_rate", true, "msg", "0.1", "0.05")

	select {
	case p := <-receivedCh:
		if p.Status != "firing" {
			t.Errorf("expected firing, got %s", p.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for firing webhook")
	}

	monitor.checkThreshold("error_rate", false, "msg", "0.01", "0.05")

	select {
	case p := <-receivedCh:
		if p.Status != "resolved" {
			t.Errorf("expected resolved, got %s", p.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for resolved webhook")
	}
}

func TestMonitorSendResolvedFalse(t *testing.T) {
	receivedCh := make(chan alertmanagerPayload, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p alertmanagerPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		receivedCh <- p
	}))
	defer server.Close()

	sendResolved := false
	cfg := DefaultConfig()
	cfg.Monitoring.WebhookConfigs = []WebhookConfig{
		{URL: server.URL, SendResolved: &sendResolved},
	}
	monitor := NewMonitor(cfg, NewMetrics())

	monitor.checkThreshold("error_rate", true, "msg", "0.1", "0.05")

	select {
	case p := <-receivedCh:
		if p.Status != "firing" {
			t.Errorf("expected firing, got %s", p.Status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for firing webhook")
	}

	monitor.checkThreshold("error_rate", false, "msg", "0.01", "0.05")

	select {
	case p := <-receivedCh:
		t.Errorf("expected no resolved webhook, but got: %+v", p)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestMonitorMultipleWebhookTargets(t *testing.T) {
	ch1 := make(chan alertmanagerPayload, 5)
	ch2 := make(chan alertmanagerPayload, 5)

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p alertmanagerPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		ch1 <- p
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p alertmanagerPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		ch2 <- p
	}))
	defer server2.Close()

	cfg := DefaultConfig()
	cfg.Monitoring.WebhookConfigs = []WebhookConfig{
		{URL: server1.URL},
		{URL: server2.URL},
	}
	monitor := NewMonitor(cfg, NewMetrics())

	monitor.checkThreshold("error_rate", true, "msg", "0.1", "0.05")

	for i, ch := range []chan alertmanagerPayload{ch1, ch2} {
		select {
		case p := <-ch:
			if p.Status != "firing" {
				t.Errorf("server %d: expected firing, got %s", i, p.Status)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("server %d: timeout waiting for webhook", i)
		}
	}
}

func TestMonitorAuthHeader(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Monitoring.WebhookConfigs = []WebhookConfig{
		{
			URL: server.URL,
			HTTPConfig: &WebhookHTTPConfig{
				Authorization: &WebhookAuthConfig{
					Type:        "Bearer",
					Credentials: "my-secret-token",
				},
			},
		},
	}
	monitor := NewMonitor(cfg, NewMetrics())

	monitor.checkThreshold("error_rate", true, "msg", "0.1", "0.05")

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for webhook")
		default:
		}
		if authHeader != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if authHeader != "Bearer my-secret-token" {
		t.Errorf("expected 'Bearer my-secret-token', got %q", authHeader)
	}
}

func TestMonitorCooldown(t *testing.T) {
	receivedCh := make(chan alertmanagerPayload, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p alertmanagerPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		receivedCh <- p
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Monitoring.AlertCooldown = 1 * time.Hour
	cfg.Monitoring.WebhookConfigs = []WebhookConfig{
		{URL: server.URL},
	}
	monitor := NewMonitor(cfg, NewMetrics())

	monitor.checkThreshold("error_rate", true, "msg", "0.1", "0.05")

	select {
	case <-receivedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first firing")
	}

	monitor.checkThreshold("error_rate", true, "msg", "0.15", "0.05")

	select {
	case p := <-receivedCh:
		t.Errorf("expected no repeat within cooldown, got: %+v", p)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestWebhookConfigValidation(t *testing.T) {
	t.Run("empty URL", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Monitoring.WebhookConfigs = []WebhookConfig{{URL: ""}}
		err := cfg.validate()
		if err == nil {
			t.Error("expected error for empty URL")
		}
	})

	t.Run("invalid URL scheme", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Monitoring.WebhookConfigs = []WebhookConfig{{URL: "ftp://example.com"}}
		err := cfg.validate()
		if err == nil {
			t.Error("expected error for non-http(s) URL")
		}
	})

	t.Run("invalid auth type", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Monitoring.WebhookConfigs = []WebhookConfig{
			{
				URL: "https://example.com/hook",
				HTTPConfig: &WebhookHTTPConfig{
					Authorization: &WebhookAuthConfig{
						Type:        "Basic",
						Credentials: "token",
					},
				},
			},
		}
		err := cfg.validate()
		if err == nil {
			t.Error("expected error for non-Bearer auth type")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Monitoring.WebhookConfigs = []WebhookConfig{
			{
				URL: "https://example.com/hook",
				HTTPConfig: &WebhookHTTPConfig{
					Authorization: &WebhookAuthConfig{
						Type:        "Bearer",
						Credentials: "token",
					},
				},
			},
		}
		err := cfg.validate()
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("empty auth type defaults to Bearer", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Monitoring.WebhookConfigs = []WebhookConfig{
			{
				URL: "https://example.com/hook",
				HTTPConfig: &WebhookHTTPConfig{
					Authorization: &WebhookAuthConfig{
						Type:        "",
						Credentials: "token",
					},
				},
			},
		}
		err := cfg.validate()
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if cfg.Monitoring.WebhookConfigs[0].HTTPConfig.Authorization.Type != "Bearer" {
			t.Errorf("expected type defaulted to Bearer, got %s", cfg.Monitoring.WebhookConfigs[0].HTTPConfig.Authorization.Type)
		}
	})
}

func TestWebhookSendResolvedDefault(t *testing.T) {
	t.Run("nil defaults to true", func(t *testing.T) {
		wc := WebhookConfig{URL: "https://example.com"}
		if !wc.ShouldSendResolved() {
			t.Error("expected default true when SendResolved is nil")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		v := true
		wc := WebhookConfig{URL: "https://example.com", SendResolved: &v}
		if !wc.ShouldSendResolved() {
			t.Error("expected true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		v := false
		wc := WebhookConfig{URL: "https://example.com", SendResolved: &v}
		if wc.ShouldSendResolved() {
			t.Error("expected false")
		}
	})
}
