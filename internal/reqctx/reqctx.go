// Package reqctx carries request-scoped values (request ID, caller) on
// context.Context so services can read them without depending on gin.
package reqctx

import "context"

type requestIDKey struct{}

// WithRequestID returns a context carrying the request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the request ID, or "" if none is set.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
