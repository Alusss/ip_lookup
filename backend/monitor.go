package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type alertState struct {
	mu           sync.Mutex
	lastAlertMap map[string]time.Time
	isActive     map[string]bool
	startedAt    map[string]time.Time
}

type Monitor struct {
	cfg     *Config
	metrics *Metrics
	alerted *alertState
	stopCh  chan struct{}
}

func NewMonitor(cfg *Config, metrics *Metrics) *Monitor {
	return &Monitor{
		cfg:     cfg,
		metrics: metrics,
		alerted: &alertState{
			lastAlertMap: make(map[string]time.Time),
			isActive:     make(map[string]bool),
			startedAt:    make(map[string]time.Time),
		},
		stopCh: make(chan struct{}),
	}
}

func (m *Monitor) Start() {
	if !m.cfg.Monitoring.Enabled {
		logInfo(nil, "self-monitoring is disabled")
		return
	}
	logInfo(nil, "self-monitoring started",
		"check_interval", m.cfg.Monitoring.CheckInterval,
		"webhook_targets", len(m.cfg.Monitoring.WebhookConfigs),
	)
	go m.loop()
}

func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) loop() {
	ticker := time.NewTicker(m.cfg.Monitoring.CheckInterval)
	defer ticker.Stop()

	var prevTotal, prev5xx, prevRLHits int64
	var prevLatencyCounts [11]int64

	for {
		select {
		case <-ticker.C:
			total := m.metrics.RequestsTotal()
			counts5xx := m.metrics.Status5xx()
			rlHits := m.metrics.RateLimitHits()
			latencyCounts := m.metrics.LatencyCounts()

			deltaTotal := total - prevTotal
			delta5xx := counts5xx - prev5xx
			deltaRL := rlHits - prevRLHits

			var errRate, rlRate float64
			var p99Ms int64

			if deltaTotal > 0 {
				errRate = float64(delta5xx) / float64(deltaTotal)
				rlRate = float64(deltaRL) / float64(deltaTotal)
				p99Ms = computeP99(latencyCounts, prevLatencyCounts, deltaTotal)
			}

			prevTotal = total
			prev5xx = counts5xx
			prevRLHits = rlHits
			prevLatencyCounts = latencyCounts

			m.evaluateThresholds(errRate, p99Ms, rlRate)

		case <-m.stopCh:
			return
		}
	}
}

func computeP99(curr, prev [11]int64, deltaTotal int64) int64 {
	var deltaBuckets [11]int64
	for i := range curr {
		deltaBuckets[i] = curr[i] - prev[i]
	}

	target := int64(float64(deltaTotal) * 0.99)
	if target <= 0 {
		return 0
	}

	var cum int64
	for i := 0; i < len(deltaBuckets); i++ {
		cum += deltaBuckets[i]
		if cum >= target {
			if i < len(latencyBucketBoundaries) {
				return latencyBucketBoundaries[i]
			}
			return 10000
		}
	}
	return 0
}

func (m *Monitor) evaluateThresholds(errRate float64, p99Ms int64, rlRate float64) {
	cfg := &m.cfg.Monitoring

	m.checkThreshold("error_rate", errRate >= cfg.ErrorRateThreshold,
		fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", errRate*100, cfg.ErrorRateThreshold*100),
		fmt.Sprintf("%.4f", errRate), fmt.Sprintf("%.4f", cfg.ErrorRateThreshold))

	m.checkThreshold("p99_latency", p99Ms >= cfg.P99LatencyThresholdMs,
		fmt.Sprintf("P99 latency %dms exceeds threshold %dms", p99Ms, cfg.P99LatencyThresholdMs),
		fmt.Sprintf("%d", p99Ms), fmt.Sprintf("%d", cfg.P99LatencyThresholdMs))

	m.checkThreshold("rate_limit_hit_rate", rlRate >= cfg.RateLimitHitRateThreshold,
		fmt.Sprintf("Rate limit hit rate %.2f%% exceeds threshold %.2f%%", rlRate*100, cfg.RateLimitHitRateThreshold*100),
		fmt.Sprintf("%.4f", rlRate), fmt.Sprintf("%.4f", cfg.RateLimitHitRateThreshold))
}

func (m *Monitor) checkThreshold(metric string, exceeded bool, message, value, threshold string) {
	now := time.Now()

	m.alerted.mu.Lock()
	wasActive := m.alerted.isActive[metric]

	if exceeded {
		if !wasActive {
			m.alerted.isActive[metric] = true
			m.alerted.startedAt[metric] = now
			m.alerted.lastAlertMap[metric] = now
			m.alerted.mu.Unlock()

			slog.Warn("self-monitoring alert triggered",
				slog.String("metric", metric),
				slog.String("message", message),
				slog.String("value", value),
				slog.String("threshold", threshold),
			)
			go m.sendWebhook(metric, message, value, threshold, now, "firing")
		} else {
			lastAlert := m.alerted.lastAlertMap[metric]
			if now.Sub(lastAlert) < m.cfg.Monitoring.AlertCooldown {
				m.alerted.mu.Unlock()
				return
			}
			m.alerted.lastAlertMap[metric] = now
			startedAt := m.alerted.startedAt[metric]
			m.alerted.mu.Unlock()

			slog.Warn("self-monitoring alert repeated",
				slog.String("metric", metric),
				slog.String("message", message),
				slog.String("value", value),
				slog.String("threshold", threshold),
			)
			go m.sendWebhook(metric, message, value, threshold, startedAt, "firing")
		}
	} else if wasActive {
		startedAt := m.alerted.startedAt[metric]
		m.alerted.isActive[metric] = false
		delete(m.alerted.startedAt, metric)
		delete(m.alerted.lastAlertMap, metric)
		m.alerted.mu.Unlock()

		slog.Info("self-monitoring alert resolved",
			slog.String("metric", metric),
			slog.String("message", message),
			slog.String("value", value),
			slog.String("threshold", threshold),
		)
		go m.sendWebhook(metric, message, value, threshold, startedAt, "resolved")
	} else {
		m.alerted.mu.Unlock()
	}
}

type alertmanagerPayload struct {
	Version           string              `json:"version"`
	GroupKey          string              `json:"groupKey"`
	Status            string              `json:"status"`
	Receiver          string              `json:"receiver"`
	GroupLabels       map[string]string   `json:"groupLabels"`
	CommonLabels      map[string]string   `json:"commonLabels"`
	CommonAnnotations map[string]string   `json:"commonAnnotations"`
	ExternalURL       string              `json:"externalURL"`
	Alerts            []alertmanagerAlert `json:"alerts"`
}

type alertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

func (m *Monitor) sendWebhook(metric, message, value, threshold string, startedAt time.Time, status string) {
	webhooks := m.cfg.Monitoring.WebhookConfigs
	if len(webhooks) == 0 {
		return
	}

	payload := m.buildPayload(metric, message, value, threshold, startedAt, status)

	body, err := json.Marshal(payload)
	if err != nil {
		logError(nil, "monitor: failed to marshal webhook payload", "error", err.Error())
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, wc := range webhooks {
		if status == "resolved" && !wc.ShouldSendResolved() {
			continue
		}

		req, err := http.NewRequest("POST", wc.URL, bytes.NewReader(body))
		if err != nil {
			logWarn(nil, "monitor: failed to create webhook request", "url", wc.URL, "error", err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		if wc.HTTPConfig != nil && wc.HTTPConfig.Authorization != nil {
			auth := wc.HTTPConfig.Authorization
			req.Header.Set("Authorization", auth.Type+" "+auth.Credentials)
		}

		resp, err := client.Do(req)
		if err != nil {
			logWarn(nil, "monitor: webhook request failed", "url", wc.URL, "error", err.Error())
			continue
		}
		resp.Body.Close()
		logInfo(nil, "monitor: webhook alert sent",
			"metric", metric,
			"status", status,
			"url", wc.URL,
			"response_status", resp.StatusCode,
		)
	}
}

func (m *Monitor) buildPayload(metric, message, value, threshold string, startedAt time.Time, status string) alertmanagerPayload {
	labels := map[string]string{
		"alertname": metric,
		"severity":  "warning",
		"instance":  "ip-lookup",
	}
	annotations := map[string]string{
		"summary":   message,
		"value":     value,
		"threshold": threshold,
	}

	endsAt := "0001-01-01T00:00:00Z"
	if status == "resolved" {
		endsAt = time.Now().Format(time.RFC3339)
	}

	alert := alertmanagerAlert{
		Status:       status,
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     startedAt.Format(time.RFC3339),
		EndsAt:       endsAt,
		GeneratorURL: "",
		Fingerprint:  fingerprint(metric),
	}

	return alertmanagerPayload{
		Version:           "4",
		GroupKey:          fmt.Sprintf("{}:{alertname=%q}", metric),
		Status:            status,
		Receiver:          "ip-lookup",
		GroupLabels:       map[string]string{},
		CommonLabels:      labels,
		CommonAnnotations: annotations,
		ExternalURL:       "",
		Alerts:            []alertmanagerAlert{alert},
	}
}

func fingerprint(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}
