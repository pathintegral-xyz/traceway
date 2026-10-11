package otelcontrollers

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type failedSpan struct {
	span      *tracepb.Span
	resource  *tracepb.ResourceSpans
	scope     string
	errorType string
	message   string
	priority  int
}

func failedSpanExceptions(ctx context.Context, project *models.Project, projectID uuid.UUID, req *coltracepb.ExportTraceServiceRequest) []models.ExceptionStackTrace {
	explicit := make(map[string]bool)
	candidates := make(map[string]failedSpan)
	var order []string
	for _, resource := range req.ResourceSpans {
		for _, scope := range resource.ScopeSpans {
			for _, span := range scope.Spans {
				if len(span.TraceId) != 16 || len(span.SpanId) != 8 {
					continue
				}
				key := hex.EncodeToString(span.TraceId)
				if spanCarriesException(span) {
					explicit[key] = true
				}
				errorType, message, priority := spanFailure(span)
				if priority == 0 {
					continue
				}
				previous, exists := candidates[key]
				if !exists {
					order = append(order, key)
				}
				if !exists || priority > previous.priority {
					candidates[key] = failedSpan{span, resource, scope.GetScope().GetName(), errorType, message, priority}
				}
			}
		}
	}
	var exceptions []models.ExceptionStackTrace
	for _, key := range order {
		if explicit[key] {
			continue
		}
		candidate := candidates[key]
		span := candidate.span
		resourceAttrs := candidate.resource.GetResource().GetAttributes()
		attrs := extractAttributes(span.Attributes)
		attrs["traceway.failure.source"] = candidate.errorType
		exceptionAttrs := []*commonpb.KeyValue{
			{Key: "exception.type", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: candidate.errorType}}},
			{Key: "exception.message", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: candidate.message}}},
		}
		traceID := otelTraceIDToUUID(span.TraceId)
		traceType := "task"
		if candidate.errorType == "HTTPServerError" {
			traceType = "endpoint"
		}
		exception := buildException(ctx, project, projectID, traceID, traceType, exceptionAttrs, span.StartTimeUnixNano, attrs,
			getStringAttribute(resourceAttrs, "service.name"), getStringAttribute(resourceAttrs, "service.version"),
			getStringAttribute(resourceAttrs, "telemetry.sdk.language"), getStringAttribute(resourceAttrs, "app.debug.proguard_uuid"), candidate.scope)
		exception.DistributedTraceId = &traceID
		exceptions = append(exceptions, exception)
	}
	return exceptions
}

func spanFailure(span *tracepb.Span) (string, string, int) {
	attrs := span.Attributes
	database := getStringAttribute(attrs, "db.system.name")
	if database == "" {
		database = getStringAttribute(attrs, "db.system")
	}
	sql := database == "" && (getStringAttribute(attrs, "db.query.text") != "" || getStringAttribute(attrs, "db.statement") != "")
	switch database {
	case "sqlite", "postgresql", "mysql", "mariadb", "mssql", "oracle", "db2", "cockroachdb", "duckdb", "clickhouse", "other_sql":
		sql = true
	}
	if sql && (span.GetStatus().GetCode() == tracepb.Status_STATUS_CODE_ERROR || getStringAttribute(attrs, "error.type") != "") {
		reason := span.GetStatus().GetMessage()
		if reason == "" {
			reason = getStringAttribute(attrs, "error.type")
		}
		if reason == "" {
			reason = "database operation failed"
		}
		return "SQLQueryError", fmt.Sprintf("%s: %s", span.Name, reason), 2
	}
	status, ok := getIntAttribute(attrs, "http.response.status_code")
	if !ok {
		status, ok = getIntAttribute(attrs, "http.status_code")
	}
	if ok && status >= 500 && status <= 599 {
		route := getStringAttribute(attrs, "http.route")
		if route == "" {
			route = span.Name
		}
		return "HTTPServerError", fmt.Sprintf("%s returned HTTP %d", route, status), 1
	}
	return "", "", 0
}
