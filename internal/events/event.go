package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/policy"
)

type Envelope struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"`
	EnvironmentID string         `json:"environment_id"`
	OccurredAt    string         `json:"occurred_at"`
	Data          map[string]any `json:"data"`
}

func (e *Envelope) NormalizeAndValidate(now time.Time) error {
	e.EventID = strings.TrimSpace(e.EventID)
	e.EventType = strings.TrimSpace(e.EventType)
	e.EnvironmentID = strings.TrimSpace(e.EnvironmentID)
	e.OccurredAt = strings.TrimSpace(e.OccurredAt)

	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if e.EnvironmentID == "" {
		return fmt.Errorf("environment_id is required")
	}
	if e.Data == nil {
		e.Data = map[string]any{}
	}
	dataJSON, err := json.Marshal(e.Data)
	if err != nil {
		return fmt.Errorf("data must be JSON serializable: %w", err)
	}
	if err := policy.EnsureNoSecretLikeText(string(dataJSON)); err != nil {
		return err
	}

	if e.OccurredAt == "" {
		e.OccurredAt = now.UTC().Format(time.RFC3339)
		return nil
	}

	if _, err := time.Parse(time.RFC3339, e.OccurredAt); err != nil {
		return fmt.Errorf("occurred_at must be RFC3339: %w", err)
	}
	return nil
}
