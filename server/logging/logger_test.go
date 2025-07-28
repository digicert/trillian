package logging

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestGenerateTraceID(t *testing.T) {
	// Test that generateTraceID returns a non-empty string
	traceID1 := generateTraceID()
	traceID2 := generateTraceID()

	if traceID1 == "" {
		t.Error("generateTraceID returned empty string")
	}

	if traceID2 == "" {
		t.Error("generateTraceID returned empty string")
	}

	// Test that two trace IDs are different
	if traceID1 == traceID2 {
		t.Error("generateTraceID returned the same trace ID twice, should be unique")
	}

	// Test that trace ID has expected format (32 char hex)
	if len(traceID1) != 32 {
		t.Error("generateTraceID didn't return expected 32 character hex format")
	}
}

func TestWithContext(t *testing.T) {
	// Create a test HTTP request
	req := httptest.NewRequest("GET", "/test", nil)

	// Test case 1: Request without existing trace ID
	ctx := WithContext(req)

	// Check that trace ID was added
	traceID := ctx.Value(ctxKeyTraceID)
	if traceID == nil {
		t.Error("WithContext didn't add trace ID to context")
	}

	// Check that span ID was added
	spanID := ctx.Value(ctxKeySpanID)
	if spanID == nil {
		t.Error("WithContext didn't add span ID to context")
	}

	// Check that both IDs are strings
	if _, ok := traceID.(string); !ok {
		t.Error("Trace ID is not a string")
	}
	if _, ok := spanID.(string); !ok {
		t.Error("Span ID is not a string")
	}
}

func TestWithContextExistingTraceID(t *testing.T) {
	// Note: WithContext for Trillian doesn't read headers - it always generates new IDs
	// This is because Trillian's HTTP endpoints are just for metrics/health
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")

	ctx := WithContext(req)

	// Check that new trace ID was generated (not from header)
	traceID := ctx.Value(ctxKeyTraceID)
	if traceID == nil {
		t.Error("WithContext didn't generate trace ID")
	}
	if traceID == "0123456789abcdef0123456789abcdef" {
		t.Error("WithContext should generate new trace ID, not use header")
	}

	// Check that new span ID was generated
	spanID := ctx.Value(ctxKeySpanID)
	if spanID == nil {
		t.Error("WithContext didn't generate span ID")
	}
}

func TestWithGRPCContext(t *testing.T) {
	// Test case 1: Empty context
	ctx := context.Background()
	newCtx := WithGRPCContext(ctx)

	traceID := newCtx.Value(ctxKeyTraceID)
	spanID := newCtx.Value(ctxKeySpanID)

	if traceID == nil || spanID == nil {
		t.Error("WithGRPCContext didn't add IDs to empty context")
	}

	// Test case 2: Context with existing values
	existingCtx := context.WithValue(context.Background(), ctxKeyTraceID, "existing-trace")
	existingCtx = context.WithValue(existingCtx, ctxKeySpanID, "existing-span")

	newCtx2 := WithGRPCContext(existingCtx)

	if newCtx2.Value(ctxKeyTraceID) != "existing-trace" {
		t.Error("WithGRPCContext didn't preserve existing trace ID")
	}
	// Note: span ID should NOT be preserved - each service generates its own
	newSpanID := newCtx2.Value(ctxKeySpanID)
	if newSpanID == "existing-span" {
		t.Error("WithGRPCContext should generate new span ID, not preserve existing one")
	}
}

func TestWithGRPCContextFromMetadata(t *testing.T) {
	// Create context with gRPC metadata
	md := metadata.Pairs("x-trace-id", "metadata-trace-id")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := WithGRPCContext(ctx)

	traceID := newCtx.Value(ctxKeyTraceID)
	spanID := newCtx.Value(ctxKeySpanID)

	if traceID != "metadata-trace-id" {
		t.Errorf("Expected trace ID from metadata 'metadata-trace-id', got %v", traceID)
	}
	// Span ID should be newly generated, not from metadata (each service gets its own span)
	if spanID == "" {
		t.Error("Expected new span ID to be generated, but got empty string")
	}
}

func TestLogWithContext(t *testing.T) {
	// Create a test hook and attach it to our global logger
	hook := test.NewLocal(log)

	// Create context with IDs
	ctx := context.WithValue(context.Background(), ctxKeyTraceID, "test-trace-id")
	ctx = context.WithValue(ctx, ctxKeySpanID, "test-span-id")

	// Test logging
	LogWithContext(ctx, "test-event", "test message", map[string]interface{}{
		"custom_field": "custom_value",
	})

	// Check that a log entry was created
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
		return
	}

	entry := hook.LastEntry()
	if entry == nil {
		t.Error("LastEntry() returned nil")
		return
	}

	// Check log level
	if entry.Level != logrus.InfoLevel {
		t.Errorf("Expected INFO level, got %v", entry.Level)
	}

	// Check message
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %v", entry.Message)
	}

	// Check fields
	if entry.Data["event_id"] != "test-event" {
		t.Errorf("Expected event_id 'test-event', got %v", entry.Data["event_id"])
	}
	if entry.Data["trace_id"] != "test-trace-id" {
		t.Errorf("Expected trace_id 'test-trace-id', got %v", entry.Data["trace_id"])
	}
	if entry.Data["span_id"] != "test-span-id" {
		t.Errorf("Expected span_id 'test-span-id', got %v", entry.Data["span_id"])
	}
	if entry.Data["custom_field"] != "custom_value" {
		t.Errorf("Expected custom_field 'custom_value', got %v", entry.Data["custom_field"])
	}
}

func TestLogTiming(t *testing.T) {
	// Create a test hook and attach it to our global logger
	hook := test.NewLocal(log)

	// Create context with IDs
	ctx := context.WithValue(context.Background(), ctxKeyTraceID, "timing-trace-id")
	ctx = context.WithValue(ctx, ctxKeySpanID, "timing-span-id")

	// Create a test request
	req := httptest.NewRequest("GET", "/api/test", nil)

	// Test timing log
	elapsed := 250 * time.Millisecond
	LogTiming(ctx, req, 200, elapsed)

	// Check that a log entry was created
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
		return
	}

	entry := hook.LastEntry()
	if entry == nil {
		t.Error("LastEntry() returned nil")
		return
	}

	// Check message
	if entry.Message != "request completed" {
		t.Errorf("Expected message 'request completed', got %v", entry.Message)
	}

	// Check timing fields
	if entry.Data["event_id"] != "timing" {
		t.Error("Expected event_id to be 'timing'")
	}
	if entry.Data["path"] != "/api/test" {
		t.Errorf("Expected path '/api/test', got %v", entry.Data["path"])
	}
	if entry.Data["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", entry.Data["method"])
	}
	if entry.Data["status"] != 200 {
		t.Errorf("Expected status 200, got %v", entry.Data["status"])
	}
	if entry.Data["elapsed"] != "250ms" {
		t.Errorf("Expected elapsed '250ms', got %v", entry.Data["elapsed"])
	}
}

func TestStatusCodeFromError(t *testing.T) {
	// Test with no error
	statusStr := statusCodeFromError(nil)
	if statusStr != "OK" {
		t.Errorf("Expected 'OK' for nil error, got %v", statusStr)
	}

	// Test with gRPC status error
	grpcErr := status.Errorf(codes.NotFound, "not found")
	statusStr = statusCodeFromError(grpcErr)
	if statusStr != "NotFound" {
		t.Errorf("Expected 'NotFound' for gRPC error, got %v", statusStr)
	}

	// Test with regular error
	regularErr := &testError{msg: "test error"}
	statusStr = statusCodeFromError(regularErr)
	if statusStr != "test error" {
		t.Errorf("Expected 'test error', got %v", statusStr)
	}
}

// Helper type for testing errors
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestPropagateToGRPC(t *testing.T) {
	// Test with context containing IDs
	ctx := context.WithValue(context.Background(), ctxKeyTraceID, "propagate-trace-id")
	ctx = context.WithValue(ctx, ctxKeySpanID, "propagate-span-id")

	newCtx := PropagateToGRPC(ctx)

	// Extract metadata from the new context
	md, ok := metadata.FromOutgoingContext(newCtx)
	if !ok {
		t.Error("PropagateToGRPC didn't add metadata to context")
	}

	// Check that trace ID was added to metadata
	traceIDs := md.Get("x-trace-id")
	if len(traceIDs) != 1 || traceIDs[0] != "propagate-trace-id" {
		t.Errorf("Expected x-trace-id 'propagate-trace-id', got %v", traceIDs)
	}

	// Check that span ID was NOT added to metadata (new design)
	spanIDs := md.Get("x-span-id")
	if len(spanIDs) != 0 {
		t.Errorf("Expected no x-span-id in metadata, but got %v", spanIDs)
	}

	// Test with empty context (should return same context)
	emptyCtx := context.Background()
	sameCtx := PropagateToGRPC(emptyCtx)

	if sameCtx != emptyCtx {
		t.Error("PropagateToGRPC should return same context when no IDs present")
	}
}
