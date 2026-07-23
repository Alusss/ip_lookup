package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ipResponse struct {
	IP      string `json:"ip"`
	Version string `json:"version,omitempty"`
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
	ISP     string `json:"isp,omitempty"`
	ASN     string `json:"asn,omitempty"`
}

type webAdConfig struct {
	Enabled bool   `json:"enabled"`
	Text    string `json:"text"`
	URL     string `json:"url"`
}

type adConfigResponse struct {
	Web webAdConfig `json:"web"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func readyzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func metricsHandler(metrics *Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "# HELP http_requests_total Total number of HTTP requests\n")
		fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
		fmt.Fprintf(w, "http_requests_total %d\n", metrics.RequestsTotal())

		fmt.Fprintf(w, "# HELP rate_limit_hits_total Total number of rate limit hits\n")
		fmt.Fprintf(w, "# TYPE rate_limit_hits_total counter\n")
		fmt.Fprintf(w, "rate_limit_hits_total %d\n", metrics.RateLimitHits())

		fmt.Fprintf(w, "# HELP inflight_requests Current number of inflight requests\n")
		fmt.Fprintf(w, "# TYPE inflight_requests gauge\n")
		fmt.Fprintf(w, "inflight_requests %d\n", metrics.InflightRequests())

		fmt.Fprintf(w, "# HELP shutdown_duration_seconds Duration of graceful shutdown in seconds\n")
		fmt.Fprintf(w, "# TYPE shutdown_duration_seconds gauge\n")
		fmt.Fprintf(w, "shutdown_duration_seconds %d\n", metrics.ShutdownDuration())

		fmt.Fprintf(w, "# HELP http_requests_2xx_total Total number of 2xx responses\n")
		fmt.Fprintf(w, "# TYPE http_requests_2xx_total counter\n")
		fmt.Fprintf(w, "http_requests_2xx_total %d\n", metrics.Status2xx())
		fmt.Fprintf(w, "# HELP http_requests_3xx_total Total number of 3xx responses\n")
		fmt.Fprintf(w, "# TYPE http_requests_3xx_total counter\n")
		fmt.Fprintf(w, "http_requests_3xx_total %d\n", metrics.Status3xx())
		fmt.Fprintf(w, "# HELP http_requests_4xx_total Total number of 4xx responses\n")
		fmt.Fprintf(w, "# TYPE http_requests_4xx_total counter\n")
		fmt.Fprintf(w, "http_requests_4xx_total %d\n", metrics.Status4xx())
		fmt.Fprintf(w, "# HELP http_requests_5xx_total Total number of 5xx responses\n")
		fmt.Fprintf(w, "# TYPE http_requests_5xx_total counter\n")
		fmt.Fprintf(w, "http_requests_5xx_total %d\n", metrics.Status5xx())

		counts := metrics.LatencyCounts()
		bucketNames := []string{"5", "10", "25", "50", "100", "250", "500", "1000", "2000", "5000", "+Inf"}
		for i, name := range bucketNames {
			fmt.Fprintf(w, "http_request_duration_ms_bucket{le=%q} %d\n", name, cumulativeSum(counts, i))
		}
		fmt.Fprintf(w, "# HELP http_request_duration_ms_sum Total latency in milliseconds\n")
		fmt.Fprintf(w, "# TYPE http_request_duration_ms_sum counter\n")
		fmt.Fprintf(w, "http_request_duration_ms_sum %d\n", metrics.TotalLatencyMs())

		fmt.Fprintf(w, "# HELP uptime_seconds Uptime of the service in seconds\n")
		fmt.Fprintf(w, "# TYPE uptime_seconds gauge\n")
		fmt.Fprintf(w, "uptime_seconds %.0f\n", metrics.Uptime().Seconds())
	}
}

func cumulativeSum(counts [11]int64, idx int) int64 {
	var sum int64
	for i := 0; i <= idx; i++ {
		sum += counts[i]
	}
	return sum
}

func adConfigHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wc := cfg.GetWebAdConfig(r)
		resp := adConfigResponse{
			Web: webAdConfig{Enabled: false},
		}
		if wc != nil {
			resp.Web = webAdConfig{
				Enabled: true,
				Text:    wc.Text,
				URL:     wc.URL,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logError(r, "failed to encode ad-config response", "error", err.Error())
		}
	}
}

func rootHandler(cfg *Config, extractor *IPExtractor, perIPLimiter *PerIPRateLimiter, globalLimiter *GlobalRateLimiter, metrics *Metrics, geo *GeoIP) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		if cfg.GetCfOnly() && !extractor.IsSourceTrusted(r) {
			logWarn(r, "cf_only: rejected direct (non-cf/proxy) source")
			http.Error(w, errForbidden, http.StatusForbidden)
			return
		}

		realIP, err := getRealIP(r, extractor)
		if err != nil {
			http.Error(w, errBadRequest, http.StatusBadRequest)
			return
		}

		ipStr := realIP.String()

		if !applyRateLimit(cfg, perIPLimiter, globalLimiter, ipStr, metrics, w) {
			return
		}

		acceptsJSON := cfg.JsonApiEnabled && strings.Contains(r.Header.Get("Accept"), "application/json")

		if acceptsJSON {
			resp := buildGeoResponse(cfg, geo, ipStr, r)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				logError(r, "failed to encode ip response", "error", err.Error())
			}
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if isWebClient(r) || !cfg.ApiAdEnabled {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			if wc := cfg.GetWebAdConfig(r); wc != nil {
				w.Header().Set("X-Ad-Enabled", "true")
				w.Header().Set("X-Ad-Text", wc.Text)
				w.Header().Set("X-Ad-URL", wc.URL)
			} else {
				w.Header().Set("X-Ad-Enabled", "false")
			}
			w.Write([]byte(ipStr))
		} else {
			text, url := cfg.GetApiAdText(r)
			response := ipStr
			if text != "" {
				response = fmt.Sprintf("%s (%s)\n%s", text, url, ipStr)
			}
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			w.Write([]byte(response))
		}
	}
}

// allHandler mirrors the / route but, when all_api_enabled is on, always returns
// a JSON document enriched with GeoIP + ASN. When the switch is off it delegates
// to the root handler so /all behaves identically to /.
func allHandler(cfg *Config, extractor *IPExtractor, perIPLimiter *PerIPRateLimiter, globalLimiter *GlobalRateLimiter, metrics *Metrics, geo *GeoIP) http.HandlerFunc {
	root := rootHandler(cfg, extractor, perIPLimiter, globalLimiter, metrics, geo)
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.GetAllApiEnabled() {
			root.ServeHTTP(w, r)
			return
		}
		if !ready.Load() {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		if cfg.GetCfOnly() && !extractor.IsSourceTrusted(r) {
			http.Error(w, errForbidden, http.StatusForbidden)
			return
		}
		realIP, err := getRealIP(r, extractor)
		if err != nil {
			http.Error(w, errBadRequest, http.StatusBadRequest)
			return
		}
		ipStr := realIP.String()
		if !applyRateLimit(cfg, perIPLimiter, globalLimiter, ipStr, metrics, w) {
			return
		}
		resp := buildGeoResponse(cfg, geo, ipStr, r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logError(r, "failed to encode /all response", "error", err.Error())
		}
	}
}

// applyRateLimit enforces the global / per-IP limiters according to the
// rate_enabled switch and rate_mode. It writes a 429 response and returns false
// when the request is rejected.
func applyRateLimit(cfg *Config, perIPLimiter *PerIPRateLimiter, globalLimiter *GlobalRateLimiter, ipStr string, metrics *Metrics, w http.ResponseWriter) bool {
	if !cfg.GetRateEnabled() {
		return true
	}
	mode := cfg.GetRateMode()
	if mode == "global" || mode == "both" {
		if !globalLimiter.Allow() {
			metrics.IncRateLimitHits()
			w.Header().Set("Retry-After", "1")
			http.Error(w, errTooManyRequests, http.StatusTooManyRequests)
			return false
		}
	}
	if mode == "per_ip" || mode == "both" {
		if !perIPLimiter.Allow(ipStr) {
			metrics.IncRateLimitHits()
			w.Header().Set("Retry-After", "6")
			http.Error(w, errTooManyRequests, http.StatusTooManyRequests)
			return false
		}
	}
	return true
}

// buildGeoResponse assembles an ipResponse, attaching localized GeoIP fields
// (city/country/isp/asn) when a GeoIP reader is configured.
func buildGeoResponse(cfg *Config, geo *GeoIP, ipStr string, r *http.Request) ipResponse {
	resp := ipResponse{
		IP:      ipStr,
		Version: ipVersion(ipStr),
	}
	if geo != nil {
		lang := cfg.detectLanguage(r)
		if loc := geo.Lookup(ipStr, lang); loc != nil {
			resp.City = loc.City
			resp.Country = loc.Country
			resp.ISP = loc.ISP
			resp.ASN = loc.ASN
		}
	}
	return resp
}

func ipVersion(ip string) string {
	if strings.Contains(ip, ":") {
		return "IPv6"
	}
	return "IPv4"
}
