package handlers

import (
	"fmt"
	"net/http"
	"time"

	"Marusia_CRM/internal/metrics"
)

func ObserveHandlerMetric(operation string, startedAt time.Time, statusCode int, reason string) {
	status := classifyStatusCode(statusCode)
	metrics.ObserveAPIRequest(operation, status, startedAt)
	if statusCode >= http.StatusBadRequest {
		if reason == "" {
			reason = status
		}
		metrics.ObserveAPIErrors(operation, reason)
	}
}

func observeEntityMetric(entity, operation string, statusCode int, reason string) {
	status := classifyStatusCode(statusCode)
	metrics.ObserveEntityOperation(entity, operation, status)
	if statusCode >= http.StatusBadRequest {
		if reason == "" {
			reason = status
		}
		metrics.ObserveAPIErrors(fmt.Sprintf("%s.%s", entity, operation), reason)
	}
}

func observeOAuthMetric(provider, stage string, statusCode int, reason string) {
	status := classifyStatusCode(statusCode)
	metrics.ObserveOAuthFlow(provider, stage, status)
	if statusCode >= http.StatusBadRequest {
		if reason == "" {
			reason = status
		}
		metrics.ObserveAPIErrors(fmt.Sprintf("oauth.%s.%s", provider, stage), reason)
	}
}

func classifyStatusCode(statusCode int) string {
	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices:
		return "2xx"
	case statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest:
		return "3xx"
	case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
		return "4xx"
	case statusCode >= http.StatusInternalServerError:
		return "5xx"
	default:
		return "unknown"
	}
}
