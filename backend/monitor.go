package main

import (
	"bytes"
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
		alerted: &alertState{lastAlertMap: make(map[string]time.Time)},
		stopCh:  make(chan struct{}),
	}
}

func (m *Monitor) Start() {
	if !m.cfg.Monitoring.Enabled {
		logInfo(nil, "self-monitoring is disabled")
		return
	}
	logInfo(nil, "self-monitoring started",
		"check_interval", m.cfg.Monitoring.CheckInterval,
		"webhook_type", m.cfg.Monitoring.AlertWebhookType,
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

	if errRate >= cfg.ErrorRateThreshold {
		m.triggerAlert("error_rate", fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", errRate*100, cfg.ErrorRateThreshold*100),
			fmt.Sprintf("%.4f", errRate), fmt.Sprintf("%.4f", cfg.ErrorRateThreshold))
	}

	if p99Ms >= cfg.P99LatencyThresholdMs {
		m.triggerAlert("p99_latency", fmt.Sprintf("P99 latency %dms exceeds threshold %dms", p99Ms, cfg.P99LatencyThresholdMs),
			fmt.Sprintf("%d", p99Ms), fmt.Sprintf("%d", cfg.P99LatencyThresholdMs))
	}

	if rlRate >= cfg.RateLimitHitRateThreshold {
		m.triggerAlert("rate_limit_hit_rate", fmt.Sprintf("Rate limit hit rate %.2f%% exceeds threshold %.2f%%", rlRate*100, cfg.RateLimitHitRateThreshold*100),
			fmt.Sprintf("%.4f", rlRate), fmt.Sprintf("%.4f", cfg.RateLimitHitRateThreshold))
	}
}

func (m *Monitor) triggerAlert(metric, message, value, threshold string) {
	now := time.Now()

	m.alerted.mu.Lock()
	lastAlert, exists := m.alerted.lastAlertMap[metric]
	if exists && now.Sub(lastAlert) < m.cfg.Monitoring.AlertCooldown {
		m.alerted.mu.Unlock()
		return
	}
	m.alerted.lastAlertMap[metric] = now
	m.alerted.mu.Unlock()

	slog.Warn("self-monitoring alert triggered",
		slog.String("metric", metric),
		slog.String("message", message),
		slog.String("value", value),
		slog.String("threshold", threshold),
	)

	go m.sendWebhook(metric, message, value, threshold, now)
}

func (m *Monitor) sendWebhook(metric, message, value, threshold string, timestamp time.Time) {
	url := m.cfg.Monitoring.AlertWebhookURL
	if url == "" {
		return
	}

	payload := m.buildPayload(metric, message, value, threshold, timestamp)

	body, err := json.Marshal(payload)
	if err != nil {
		logError(nil, "monitor: failed to marshal webhook payload", "error", err.Error())
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		logWarn(nil, "monitor: webhook request failed", "error", err.Error())
		return
	}
	resp.Body.Close()
	logInfo(nil, "monitor: webhook alert sent", "metric", metric, "status", resp.StatusCode)
}

func (m *Monitor) buildPayload(metric, message, value, threshold string, timestamp time.Time) interface{} {
	switch m.cfg.Monitoring.AlertWebhookType {
	case "dingtalk":
		return map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": fmt.Sprintf("[ip-lookup] %s\n%s\nValue: %s, Threshold: %s\nTime: %s",
					metric, message, value, threshold, timestamp.Format(time.RFC3339)),
			},
		}
	default:
		return map[string]string{
			"title":     "[ip-lookup] " + metric,
			"message":   message,
			"metric":    metric,
			"value":     value,
			"threshold": threshold,
			"severity":  "warning",
			"timestamp": timestamp.Format(time.RFC3339),
		}
	}
}
