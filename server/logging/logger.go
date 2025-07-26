package logging

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	ctxKeyTxID   contextKey = "transaction_id"
	ctxKeySpanID contextKey = "span_id"
)

var log = logrus.New()

func init() {
	log.Formatter = &logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

func generateUUID() string {
	return uuid.New().String()
}

func WithContext(r *http.Request) context.Context {
	txID := r.Header.Get("X-Transaction-ID")
	if txID == "" {
		txID = generateUUID()
	}

	spanID := generateUUID()
	ctx := context.WithValue(r.Context(), ctxKeyTxID, txID)
	ctx = context.WithValue(ctx, ctxKeySpanID, spanID)
	return ctx
}

func WithGRPCContext(ctx context.Context) context.Context {
	// First check if there's already a transaction_id in the context (from HTTP)
	txID, ok := ctx.Value(ctxKeyTxID).(string)
	if !ok || txID == "" {
		// If not, try to get it from gRPC metadata
		txID = getFromMetadata(ctx, "X-Transaction-ID")
		if txID == "" {
			txID = generateUUID()
		}
	}

	// Check for span_id in context first, then metadata
	spanID, ok := ctx.Value(ctxKeySpanID).(string)
	if !ok || spanID == "" {
		// Always generate a new span_id for this service, regardless of metadata
		// This ensures each service has its own span while maintaining trace correlation
		spanID = generateUUID()
	}

	ctx = context.WithValue(ctx, ctxKeyTxID, txID)
	ctx = context.WithValue(ctx, ctxKeySpanID, spanID)
	return ctx
}

func getFromMetadata(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// PropagateToGRPC adds the transaction_id and span_id from the context to gRPC metadata
// This should be called when making outgoing gRPC calls from HTTP handlers
func PropagateToGRPC(ctx context.Context) context.Context {
	txID := ctx.Value(ctxKeyTxID)
	spanID := ctx.Value(ctxKeySpanID)

	if txID == nil && spanID == nil {
		return ctx
	}

	mdMap := make(map[string]string)
	if txID != nil {
		mdMap["X-Transaction-ID"] = txID.(string)
	}
	if spanID != nil {
		mdMap["X-Span-ID"] = spanID.(string)
	}

	md := metadata.New(mdMap)
	return metadata.NewOutgoingContext(ctx, md)
}

func LogWithContext(ctx context.Context, eventID string, msg string, fields map[string]interface{}) {
	lf := logrus.Fields{
		"event_id": eventID,
	}

	// Safely extract transaction_id and span_id
	if txID := ctx.Value(ctxKeyTxID); txID != nil {
		sanitizedTxID := strings.ReplaceAll(txID.(string), "\n", "")
		sanitizedTxID = strings.ReplaceAll(sanitizedTxID, "\r", "")
		lf["transaction_id"] = sanitizedTxID
	}
	if spanID := ctx.Value(ctxKeySpanID); spanID != nil {
		sanitizedSpanID := strings.ReplaceAll(spanID.(string), "\n", "")
		sanitizedSpanID = strings.ReplaceAll(sanitizedSpanID, "\r", "")
		lf["span_id"] = sanitizedSpanID
	}

	for k, v := range fields {
		lf[k] = v
	}
	log.WithFields(lf).Info(msg)
}

func LogTiming(ctx context.Context, r *http.Request, status int, elapsed time.Duration) {
	elapsedInMsStr := fmt.Sprintf("%dms", elapsed.Milliseconds())
	LogWithContext(ctx, "timing", "request completed", map[string]interface{}{
		"path":    r.URL.Path,
		"method":  r.Method,
		"status":  status,
		"elapsed": elapsedInMsStr,
	})
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx = WithGRPCContext(ctx)
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)
		LogWithContext(ctx, "timing", "gRPC call completed", map[string]interface{}{
			"method":  info.FullMethod,
			"status":  statusCodeFromError(err),
			"elapsed": elapsed.Milliseconds(),
		})
		return resp, err
	}
}

func statusCodeFromError(err error) string {
	if err == nil {
		return "OK"
	}
	// Try to extract gRPC status code if available
	if status, ok := status.FromError(err); ok {
		return status.Code().String()
	}
	return err.Error()
}
