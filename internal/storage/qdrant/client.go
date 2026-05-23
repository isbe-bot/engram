package qdrant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultVectorSize = 256

type Client struct {
	baseURL    string
	collection string
	http       *http.Client
}

type Point struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type SearchResult struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

func New(baseURL, collection string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	collection = strings.TrimSpace(collection)
	if baseURL == "" {
		return nil, fmt.Errorf("qdrant base URL is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid qdrant base URL: %w", err)
	}
	if collection == "" {
		return nil, fmt.Errorf("qdrant collection is required")
	}
	return &Client{baseURL: baseURL, collection: collection, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func PointID(sourceID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceID)))
	hexValue := hex.EncodeToString(sum[:16])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[0:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:32])
}

func (c *Client) EnsureCollection(ctx context.Context, vectorSize int) error {
	if vectorSize <= 0 {
		vectorSize = DefaultVectorSize
	}
	payload := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}
	_, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/collections/%s", url.PathEscape(c.collection)), payload)
	if err != nil && strings.Contains(err.Error(), "status=409") && strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return nil
	}
	return err
}

func (c *Client) Upsert(ctx context.Context, points []Point) error {
	if len(points) == 0 {
		return nil
	}
	payload := map[string]any{"points": points}
	_, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/collections/%s/points?wait=true", url.PathEscape(c.collection)), payload)
	return err
}

func (c *Client) Search(ctx context.Context, vector []float32, limit int, filter map[string]any) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	payload := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}
	if len(filter) > 0 {
		payload["filter"] = filter
	}
	body, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/search", url.PathEscape(c.collection)), payload)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result []SearchResult `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode qdrant search response: %w", err)
	}
	return response.Result, nil
}

func (c *Client) do(ctx context.Context, method, path string, payload any) ([]byte, error) {
	if c == nil || c.http == nil {
		return nil, fmt.Errorf("qdrant client is not initialized")
	}
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode qdrant request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build qdrant request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read qdrant response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}
