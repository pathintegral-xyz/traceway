package otelcontrollers

import (
	"context"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"strings"
	"testing"
	"time"
)

func failureTestSpan(id byte) *tracepb.Span {
	now := uint64(time.Now().UnixNano())
	return &tracepb.Span{TraceId: []byte("failuretraceabcd"), SpanId: []byte{id, 1, 2, 3, 4, 5, 6, 7}, Name: "test-operation", Kind: tracepb.Span_SPAN_KIND_INTERNAL, StartTimeUnixNano: now, EndTimeUnixNano: now + 1000000}
}

func failureTestRequest(spans ...*tracepb.Span) *coltracepb.ExportTraceServiceRequest {
	return &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{ScopeSpans: []*tracepb.ScopeSpans{{Spans: spans}}}}}
}

func TestFailureSignalsBecomeExceptions(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "modern"}[modern], func(t *testing.T) {
			sql := failureTestSpan(1)
			sql.Status = &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "constraint rejected"}
			dbKey, httpKey := "db.system", "http.status_code"
			if modern {
				dbKey, httpKey = "db.system.name", "http.response.status_code"
			}
			sql.Attributes = append(sql.Attributes, strKV(dbKey, "sqlite"))
			_, _, _, exceptions, _, _ := convertTraces(context.Background(), nil, testProjectId, failureTestRequest(sql))
			if len(exceptions) != 1 || !strings.HasPrefix(exceptions[0].StackTrace, "SQLQueryError:") {
				t.Fatalf("SQL failure not captured: %+v", exceptions)
			}
			http := failureTestSpan(2)
			http.Kind = tracepb.Span_SPAN_KIND_SERVER
			http.Attributes = append(http.Attributes, intKV(httpKey, 500))
			_, _, _, exceptions, _, _ = convertTraces(context.Background(), nil, testProjectId, failureTestRequest(http))
			if len(exceptions) != 1 || !strings.HasPrefix(exceptions[0].StackTrace, "HTTPServerError:") {
				t.Fatalf("HTTP failure not captured: %+v", exceptions)
			}
			http.Attributes = []*commonpb.KeyValue{intKV(httpKey, 404)}
			if got := failedSpanExceptions(context.Background(), nil, testProjectId, failureTestRequest(http)); len(got) != 0 {
				t.Fatalf("404 must not become a server error: %+v", got)
			}
			sql.Status = nil
			if got := failedSpanExceptions(context.Background(), nil, testProjectId, failureTestRequest(sql)); len(got) != 0 {
				t.Fatalf("successful SQL must not alert: %+v", got)
			}
		})
	}
}

func TestFailureSignalsPreferRootCauseAcrossResources(t *testing.T) {
	sql, http := failureTestSpan(1), failureTestSpan(2)
	sql.Status = &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR}
	sql.Attributes = append(sql.Attributes, strKV("db.system.name", "sqlite"))
	http.Attributes = append(http.Attributes, intKV("http.response.status_code", 500))
	req := failureTestRequest(http)
	req.ResourceSpans = append(req.ResourceSpans, failureTestRequest(sql).ResourceSpans...)
	got := failedSpanExceptions(context.Background(), nil, testProjectId, req)
	if len(got) != 1 || !strings.HasPrefix(got[0].StackTrace, "SQLQueryError:") {
		t.Fatalf("expected one SQL root cause: %+v", got)
	}
	sql.Events = []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: sql.StartTimeUnixNano, Attributes: []*commonpb.KeyValue{strKV("exception.type", "ConstraintError")}}}
	if got := failedSpanExceptions(context.Background(), nil, testProjectId, req); len(got) != 0 {
		t.Fatalf("real exception must suppress synthetic failures: %+v", got)
	}
}
