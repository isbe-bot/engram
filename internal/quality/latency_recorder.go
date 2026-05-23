package quality

import (
	"context"
	"time"
)

type latencySampleWriter interface {
	RecordLatencySample(ctx context.Context, operation string, latency time.Duration, now time.Time) error
}

type LatencyRecorder struct {
	store latencySampleWriter
	now   func() time.Time
}

func NewLatencyRecorder(store latencySampleWriter) *LatencyRecorder {
	return &LatencyRecorder{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func (r *LatencyRecorder) RecordLatency(ctx context.Context, operation string, latency time.Duration) {
	if r == nil || r.store == nil {
		return
	}
	_ = r.store.RecordLatencySample(ctx, operation, latency, r.now())
}
