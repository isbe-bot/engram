package contracts

type MemoryWriteRequest struct {
	ObjectID       string   `json:"object_id,omitempty"`
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	SourceRefs     []string `json:"source_refs"`
	Confidence     float64  `json:"confidence,omitempty"`
	Classification string   `json:"classification,omitempty"`
	SchemaVersion  string   `json:"schema_version,omitempty"`
}

type MemoryCorrectRequest struct {
	Content    string   `json:"content"`
	Reason     string   `json:"reason"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type MemoryDeprecateRequest struct {
	Reason string `json:"reason"`
}

type SearchResponse struct {
	Results []map[string]any `json:"results"`
}
