package requestid

import "context"

type contextKey struct{}

type Details struct {
	ID           string
	Application  string
	Version      string
	PythonStatus int
}

func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, &Details{ID: id})
}

func FromContext(ctx context.Context) *Details {
	details, _ := ctx.Value(contextKey{}).(*Details)
	return details
}
