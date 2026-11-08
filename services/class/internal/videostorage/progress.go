package videostorage

import "context"

type publishProgressKey struct{}

// WithPublishProgress reports only segments whose writes/checksums succeeded.
func WithPublishProgress(ctx context.Context, report func(completed, total int)) context.Context {
	return context.WithValue(ctx, publishProgressKey{}, report)
}
func reportPublishProgress(ctx context.Context, completed, total int) {
	if report, ok := ctx.Value(publishProgressKey{}).(func(int, int)); ok && report != nil {
		report(completed, total)
	}
}
