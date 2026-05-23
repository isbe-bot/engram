package models

type MemoryObject struct {
	ObjectID     string   `json:"object_id"`
	Type         string   `json:"type"`
	SchemaVer    string   `json:"schema_version"`
	Content      string   `json:"content"`
	SourceRefs   []string `json:"source_refs"`
	Confidence   float64  `json:"confidence"`
	Classification string `json:"classification"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}
