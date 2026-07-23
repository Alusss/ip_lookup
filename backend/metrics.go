package main

import (
	"sync/atomic"
	"time"
)

var latencyBucketBoundaries = []int64{5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000}

type Metrics struct {
	requestsTotal    atomic.Int64
	rateLimitHits    atomic.Int64
	inflightRequests atomic.Int64
	shutdownDuration atomic.Int64

	status2xx atomic.Int64
	status3xx atomic.Int64
	status4xx atomic.Int64
	status5xx atomic.Int64

	latencyBuckets [11]atomic.Int64

	totalLatencyMs atomic.Int64

	startTime time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{startTime: time.Now()}
}

func (m *Metrics) IncRequestsTotal() {
	m.requestsTotal.Add(1)
}

func (m *Metrics) IncRateLimitHits() {
	m.rateLimitHits.Add(1)
}

func (m *Metrics) IncInflight() {
	m.inflightRequests.Add(1)
}

func (m *Metrics) DecInflight() {
	m.inflightRequests.Add(-1)
}

func (m *Metrics) SetShutdownDuration(d int64) {
	m.shutdownDuration.Store(d)
}

func (m *Metrics) ObserveRequest(statusCode int, latencyMs int64) {
	bucket := findBucket(latencyMs)
	m.latencyBuckets[bucket].Add(1)
	m.totalLatencyMs.Add(latencyMs)

	switch {
	case statusCode >= 200 && statusCode < 300:
		m.status2xx.Add(1)
	case statusCode >= 300 && statusCode < 400:
		m.status3xx.Add(1)
	case statusCode >= 400 && statusCode < 500:
		m.status4xx.Add(1)
	case statusCode >= 500:
		m.status5xx.Add(1)
	}
}

func (m *Metrics) RequestsTotal() int64 {
	return m.requestsTotal.Load()
}

func (m *Metrics) RateLimitHits() int64 {
	return m.rateLimitHits.Load()
}

func (m *Metrics) InflightRequests() int64 {
	return m.inflightRequests.Load()
}

func (m *Metrics) ShutdownDuration() int64 {
	return m.shutdownDuration.Load()
}

func (m *Metrics) Status2xx() int64  { return m.status2xx.Load() }
func (m *Metrics) Status3xx() int64  { return m.status3xx.Load() }
func (m *Metrics) Status4xx() int64  { return m.status4xx.Load() }
func (m *Metrics) Status5xx() int64  { return m.status5xx.Load() }
func (m *Metrics) TotalLatencyMs() int64 { return m.totalLatencyMs.Load() }

func (m *Metrics) StatusCodesTotal() int64 {
	return m.status2xx.Load() + m.status3xx.Load() + m.status4xx.Load() + m.status5xx.Load()
}

func (m *Metrics) LatencyCounts() [11]int64 {
	var counts [11]int64
	for i := range m.latencyBuckets {
		counts[i] = m.latencyBuckets[i].Load()
	}
	return counts
}

func (m *Metrics) Uptime() time.Duration {
	return time.Since(m.startTime)
}

func findBucket(latencyMs int64) int {
	for i, boundary := range latencyBucketBoundaries {
		if latencyMs <= boundary {
			return i
		}
	}
	return len(latencyBucketBoundaries)
}

