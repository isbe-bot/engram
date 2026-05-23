package contracts

type MemoryWriteRequest struct {
	Type       string   `json:"type"`
	Content    string   `json:"content"`
	SourceRefs []string `json:"source_refs"`
}

type SearchResponse struct {
	Results []map[string]any `json:"results"`
}
