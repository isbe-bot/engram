package events

type Envelope struct {
	EventID      string         `json:"event_id"`
	EventType    string         `json:"event_type"`
	EnvironmentID string        `json:"environment_id"`
	OccurredAt   string         `json:"occurred_at"`
	Data         map[string]any `json:"data"`
}
