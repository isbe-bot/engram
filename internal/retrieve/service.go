package retrieve

import "context"

type SearchResult = map[string]any

type Query struct {
	Text          string
	Status        string
	MinConfidence float64
	Limit         int
}

type memorySearcher interface {
	SearchMemoryObjects(ctx context.Context, q Query) ([]SearchResult, error)
}

type Service struct {
	searcher memorySearcher
}

func NewService(searcher memorySearcher) *Service {
	return &Service{searcher: searcher}
}

func (s *Service) Search(ctx context.Context, q Query) ([]SearchResult, error) {
	if s == nil || s.searcher == nil {
		return []SearchResult{}, nil
	}
	return s.searcher.SearchMemoryObjects(ctx, q)
}
