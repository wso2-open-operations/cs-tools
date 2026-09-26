package entityclient

import "context"

// The BFF's middleware stores the correlation id under its own unexported key,
// so this package cannot read it directly. WithCorrelationID is called by the
// middleware to put a copy where the client can find it.
//
// Duplicating the value rather than exporting the middleware's key keeps the
// dependency pointing one way: middleware knows about this package, this
// package knows nothing about middleware.

const correlationIDHeader = "X-CSM-Correlation-ID"

type correlationIDKey struct{}

// WithCorrelationID returns a context carrying the id for outgoing requests.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

func correlationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey{}).(string)
	return v
}
