package retention

import (
	"context"
	"time"
)

const (
	DefaultEventRetentionDays            = 90
	DefaultDeprecatedMemoryRetentionDays = 180
	DefaultStaleMemoryDays               = 30
	DefaultMaxCandidates                 = 1000
)

type Policy struct {
	EventRetentionDays            int `json:"event_retention_days"`
	DeprecatedMemoryRetentionDays int `json:"deprecated_memory_retention_days"`
	StaleMemoryDays               int `json:"stale_memory_days"`
	MaxCandidates                 int `json:"max_candidates"`
}

type Cutoffs struct {
	RawEventsBefore        string `json:"raw_events_before"`
	DeprecatedMemoryBefore string `json:"deprecated_memory_before"`
	StaleMemoryBefore      string `json:"stale_memory_before"`
}

type Candidate struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
	AgeDays   int    `json:"age_days"`
	Action    string `json:"action"`
}

type Summary struct {
	RawEventDeleteCandidates         int `json:"raw_event_delete_candidates"`
	DeprecatedMemoryDeleteCandidates int `json:"deprecated_memory_delete_candidates"`
	StaleMemoryReviewCandidates      int `json:"stale_memory_review_candidates"`
	TotalDeleteCandidates            int `json:"total_delete_candidates"`
	TotalReviewCandidates            int `json:"total_review_candidates"`
}

type Report struct {
	GeneratedAt string      `json:"generated_at"`
	Apply       bool        `json:"apply"`
	Policy      Policy      `json:"policy"`
	Cutoffs     Cutoffs     `json:"cutoffs"`
	Summary     Summary     `json:"summary"`
	Candidates  []Candidate `json:"candidates"`
	Applied     *Applied    `json:"applied,omitempty"`
}

type Applied struct {
	DeletedRawEvents        int `json:"deleted_raw_events"`
	DeletedDeprecatedMemory int `json:"deleted_deprecated_memory"`
}

type Store interface {
	RetentionCandidates(ctx context.Context, policy Policy, now time.Time) ([]Candidate, Summary, error)
	ApplyRetention(ctx context.Context, policy Policy, now time.Time) (Applied, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }}
}

func NormalizePolicy(policy Policy) Policy {
	if policy.EventRetentionDays <= 0 {
		policy.EventRetentionDays = DefaultEventRetentionDays
	}
	if policy.DeprecatedMemoryRetentionDays <= 0 {
		policy.DeprecatedMemoryRetentionDays = DefaultDeprecatedMemoryRetentionDays
	}
	if policy.StaleMemoryDays <= 0 {
		policy.StaleMemoryDays = DefaultStaleMemoryDays
	}
	if policy.MaxCandidates <= 0 {
		policy.MaxCandidates = DefaultMaxCandidates
	}
	return policy
}

func (s *Service) Report(ctx context.Context, policy Policy, apply bool) (Report, error) {
	now := time.Now().UTC()
	if s != nil && s.now != nil {
		now = s.now().UTC()
	}
	policy = NormalizePolicy(policy)
	report := Report{
		GeneratedAt: now.Format(time.RFC3339),
		Apply:       apply,
		Policy:      policy,
		Cutoffs: Cutoffs{
			RawEventsBefore:        now.Add(-time.Duration(policy.EventRetentionDays) * 24 * time.Hour).Format(time.RFC3339),
			DeprecatedMemoryBefore: now.Add(-time.Duration(policy.DeprecatedMemoryRetentionDays) * 24 * time.Hour).Format(time.RFC3339),
			StaleMemoryBefore:      now.Add(-time.Duration(policy.StaleMemoryDays) * 24 * time.Hour).Format(time.RFC3339),
		},
		Candidates: []Candidate{},
	}
	if s == nil || s.store == nil {
		return report, nil
	}
	candidates, summary, err := s.store.RetentionCandidates(ctx, policy, now)
	if err != nil {
		return Report{}, err
	}
	report.Candidates = candidates
	report.Summary = summary
	if apply {
		applied, err := s.store.ApplyRetention(ctx, policy, now)
		if err != nil {
			return Report{}, err
		}
		report.Applied = &applied
	}
	return report, nil
}
