package httpstrangler

import (
	"context"
	"net/http"
	"time"
)

type Option func(*diffingOrchestrator)

func WithDiffer(differ Differ) Option {
	return func(d *diffingOrchestrator) {
		d.differ = differ
	}
}

func WithDiffReporter(reporter DiffReporter) Option {
	return func(d *diffingOrchestrator) {
		d.reporter = reporter
	}
}

func WithAlternativeTimeout(timeout time.Duration) Option {
	return func(d *diffingOrchestrator) {
		d.alternativeTimeout = timeout
	}
}

func WithAlternativeContext(ctxConstructor func(sourceCtx context.Context) context.Context) Option {
	return func(d *diffingOrchestrator) {
		d.alternativeContext = ctxConstructor
	}
}

func newDiffHandler(defaultHandler, alternativeHandler http.Handler, options []Option) *diffingOrchestrator {
	d := &diffingOrchestrator{
		defaultHandler:     defaultHandler,
		alternativeHandler: alternativeHandler,
		differ:             CMPDiffer{},
		reporter:           PrintDiffReporter{},
		alternativeTimeout: time.Second * 5,
		alternativeContext: func(ctx context.Context) context.Context {
			return context.Background()
		},
	}
	for _, opt := range options {
		opt(d)
	}
	return d
}
