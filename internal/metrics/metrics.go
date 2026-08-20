package metrics

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registerOnce sync.Once

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests handled by the CRM service.",
		},
		[]string{"method", "route", "status"},
	)
	HTTPRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests handled by the CRM service.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "http_requests_in_flight",
			Help:      "Current number of in-flight HTTP requests handled by the CRM service.",
		},
	)
	CRMRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "api_requests_total",
			Help:      "Total number of CRM API requests by operation and status.",
		},
		[]string{"operation", "status"},
	)
	CRMRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "api_request_duration_seconds",
			Help:      "Duration of CRM API requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	CRMApiErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "api_errors_total",
			Help:      "Total number of CRM API errors by operation and reason.",
		},
		[]string{"operation", "reason"},
	)
	EntityOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "entity_operations_total",
			Help:      "Total number of CRM entity operations by entity, operation, and status.",
		},
		[]string{"entity", "operation", "status"},
	)
	OAuthFlowsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "oauth_flows_total",
			Help:      "Total number of OAuth flow attempts by provider, stage, and status.",
		},
		[]string{"provider", "stage", "status"},
	)
	ProviderRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "provider_requests_total",
			Help:      "Total number of provider HTTP requests by provider, method, and status.",
		},
		[]string{"provider", "method", "status"},
	)
	ProviderRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "air",
			Subsystem: "crm",
			Name:      "provider_request_duration_seconds",
			Help:      "Duration of provider HTTP requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"provider", "method"},
	)
)

func Register() {
	registerOnce.Do(func() {
		registerCollector(collectors.NewGoCollector())
		registerCollector(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		registerCollector(HTTPRequestsTotal)
		registerCollector(HTTPRequestDurationSeconds)
		registerCollector(HTTPRequestsInFlight)
		registerCollector(CRMRequestsTotal)
		registerCollector(CRMRequestDurationSeconds)
		registerCollector(CRMApiErrorsTotal)
		registerCollector(EntityOperationsTotal)
		registerCollector(OAuthFlowsTotal)
		registerCollector(ProviderRequestsTotal)
		registerCollector(ProviderRequestDurationSeconds)
	})
}

func registerCollector(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		var alreadyRegisteredError prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegisteredError) {
			return
		}
		panic(err)
	}
}

func Handler() http.Handler {
	Register()
	return promhttp.Handler()
}

func ObserveHTTP(method, route, status string, startedAt time.Time) {
	normalizedMethod := normalizeLabel(method, "unknown")
	normalizedRoute := NormalizeRoute(route)
	normalizedStatus := normalizeLabel(status, "unknown")

	HTTPRequestDurationSeconds.WithLabelValues(normalizedMethod, normalizedRoute).Observe(time.Since(startedAt).Seconds())
	HTTPRequestsTotal.WithLabelValues(normalizedMethod, normalizedRoute, normalizedStatus).Inc()
}

func ObserveHTTPInFlight(delta float64) {
	HTTPRequestsInFlight.Add(delta)
}

func ObserveAPIRequest(operation, status string, startedAt time.Time) {
	normalizedOperation := normalizeLabel(operation, "unknown")
	normalizedStatus := normalizeLabel(status, "unknown")

	CRMRequestDurationSeconds.WithLabelValues(normalizedOperation).Observe(time.Since(startedAt).Seconds())
	CRMRequestsTotal.WithLabelValues(normalizedOperation, normalizedStatus).Inc()
}

func ObserveAPIErrors(operation, reason string) {
	CRMApiErrorsTotal.WithLabelValues(normalizeLabel(operation, "unknown"), normalizeLabel(reason, "unknown")).Inc()
}

func ObserveEntityOperation(entity, operation, status string) {
	normalizedEntity := normalizeLabel(entity, "unknown")
	normalizedOperation := normalizeLabel(operation, "unknown")
	normalizedStatus := normalizeLabel(status, "unknown")

	EntityOperationsTotal.WithLabelValues(normalizedEntity, normalizedOperation, normalizedStatus).Inc()
}

func ObserveOAuthFlow(provider, stage, status string) {
	normalizedProvider := normalizeLabel(provider, "unknown")
	normalizedStage := normalizeLabel(stage, "unknown")
	normalizedStatus := normalizeLabel(status, "unknown")

	OAuthFlowsTotal.WithLabelValues(normalizedProvider, normalizedStage, normalizedStatus).Inc()
}

func ObserveProviderRequest(provider, method, status string, startedAt time.Time) {
	normalizedProvider := normalizeLabel(provider, "unknown")
	normalizedMethod := normalizeLabel(method, "unknown")
	normalizedStatus := normalizeLabel(status, "unknown")

	ProviderRequestDurationSeconds.WithLabelValues(normalizedProvider, normalizedMethod).Observe(time.Since(startedAt).Seconds())
	ProviderRequestsTotal.WithLabelValues(normalizedProvider, normalizedMethod, normalizedStatus).Inc()
}

func NormalizeRoute(path string) string {
	if path == "" {
		return "unknown"
	}
	if path == "/metrics" {
		return path
	}
	trimmed := strings.TrimSuffix(path, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func normalizeLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
