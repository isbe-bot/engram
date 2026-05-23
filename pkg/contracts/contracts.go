package contracts

type MutationEnvelope struct {
	ActorID    string `json:"actor_id"`
	MutationID string `json:"mutation_id"`
	Signature  string `json:"signature"`
}

type MemoryWriteRequest struct {
	ObjectID       string           `json:"object_id,omitempty"`
	Type           string           `json:"type"`
	Content        string           `json:"content"`
	SourceRefs     []string         `json:"source_refs"`
	Confidence     float64          `json:"confidence,omitempty"`
	Classification string           `json:"classification,omitempty"`
	SchemaVersion  string           `json:"schema_version,omitempty"`
	Envelope       MutationEnvelope `json:"envelope,omitempty"`
}

type MemoryCorrectRequest struct {
	Content    string           `json:"content"`
	Reason     string           `json:"reason"`
	SourceRefs []string         `json:"source_refs,omitempty"`
	Force      bool             `json:"force,omitempty"`
	Envelope   MutationEnvelope `json:"envelope"`
}

type MemoryDeprecateRequest struct {
	Reason   string           `json:"reason"`
	Force    bool             `json:"force,omitempty"`
	Envelope MutationEnvelope `json:"envelope"`
}

type MemoryHistoryRequest struct {
	Action string
	Before int
	Limit  int
}

type SearchResponse struct {
	Results []map[string]any `json:"results"`
}
