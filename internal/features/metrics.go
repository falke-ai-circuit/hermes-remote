package features

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type MetricType int

const (
	MetricCounter MetricType = iota
	MetricGauge
	MetricHistogram
	MetricTimer
)

type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Count     int64             `json:"count"`
	Sum       float64           `json:"sum"`
	Min       float64           `json:"min"`
	Max       float64           `json:"max"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{metrics: make(map[string]*Metric)}
}

func (c *MetricsCollector) IncrementCounter(name string, delta float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.metrics[name]
	if !ok {
		m = &Metric{Name: name, Type: MetricCounter, UpdatedAt: time.Now()}
		c.metrics[name] = m
	}
	atomic.AddInt64(&m.Count, 1)
	m.Value += delta
	m.UpdatedAt = time.Now()
}

func (c *MetricsCollector) SetGauge(name string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.metrics[name]
	if !ok {
		m = &Metric{Name: name, Type: MetricGauge, UpdatedAt: time.Now()}
		c.metrics[name] = m
	}
	m.Value = value
	m.UpdatedAt = time.Now()
}

func (c *MetricsCollector) RecordHistogram(name string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.metrics[name]
	if !ok {
		m = &Metric{Name: name, Type: MetricHistogram, Min: value, Max: value, UpdatedAt: time.Now()}
		c.metrics[name] = m
	}
	m.Count++
	m.Sum += value
	if value < m.Min { m.Min = value }
	if value > m.Max { m.Max = value }
	m.Value = m.Sum / float64(m.Count)
	m.UpdatedAt = time.Now()
}

type Timer struct {
	name      string
	collector *MetricsCollector
	start     time.Time
}

func (c *MetricsCollector) StartTimer(name string) *Timer {
	return &Timer{name: name, collector: c, start: time.Now()}
}

func (t *Timer) Stop() time.Duration {
	elapsed := time.Since(t.start)
	t.collector.RecordHistogram(t.name+"_duration_ms", float64(elapsed.Milliseconds()))
	return elapsed
}

func (c *MetricsCollector) GetAllMetrics() []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()
	results := make([]Metric, 0, len(c.metrics))
	for _, m := range c.metrics { results = append(results, *m) }
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results
}

func (c *MetricsCollector) FormatMetrics() string {
	metrics := c.GetAllMetrics()
	result := fmt.Sprintf("=== Metrics Report (%d metrics) ===\n", len(metrics))
	for _, m := range metrics {
		var typeStr string
		switch m.Type {
		case MetricCounter: typeStr = "counter"
		case MetricGauge: typeStr = "gauge"
		case MetricHistogram: typeStr = "histogram"
		case MetricTimer: typeStr = "timer"
		}
		result += fmt.Sprintf("  %s [%s]: value=%.2f count=%d", m.Name, typeStr, m.Value, m.Count)
		if m.Type == MetricHistogram { result += fmt.Sprintf(" min=%.2f max=%.2f avg=%.2f", m.Min, m.Max, m.Value) }
		result += fmt.Sprintf(" updated=%s\n", m.UpdatedAt.Format(time.RFC3339))
	}
	return result
}

func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = make(map[string]*Metric)
}

func (c *MetricsCollector) GetMetric(name string) *Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if m, ok := c.metrics[name]; ok { return m }
	return nil
}
