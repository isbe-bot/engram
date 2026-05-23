package events

import (
	"strings"
	"testing"
	"time"
)

func TestEnvelopeRejectsSecretLikeData(t *testing.T) {
	env := Envelope{
		EventID:       "evt-secret",
		EventType:     "task.completed",
		EnvironmentID: "test",
		Data: map[string]any{
			"token": "ghp_" + strings.Repeat("a", 24),
		},
	}
	if err := env.NormalizeAndValidate(time.Now()); err == nil {
		t.Fatal("expected secret-like event data to be rejected")
	}
}
