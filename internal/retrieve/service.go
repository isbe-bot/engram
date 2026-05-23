package retrieve

import "context"

type SearchResult = map[string]any

type eventSearcher interface {
	SearchEvents(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

type Service struct {
	searcher eventSearcher
}

func NewService(searcher eventSearcher) *Service {
	return &Service{searcher: searcher}
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if s == nil || s.searcher == nil {
		return []SearchResult{}, nil
	}
	return s.searcher.SearchEvents(ctx, query, limit)
}
