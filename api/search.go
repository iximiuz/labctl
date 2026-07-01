package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// SearchItem is a single hit returned by GET /api/search. The endpoint returns a
// flat, relevance-ranked list mixing multiple content kinds, so every item is
// tagged with its Kind. Only the fields shared across kinds are decoded here.
type SearchItem struct {
	Kind        string   `json:"kind" yaml:"kind"`
	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	PageURL     string   `json:"pageUrl" yaml:"pageUrl"`
	Difficulty  string   `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Categories  []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Official    bool     `json:"official,omitempty" yaml:"official,omitempty"`

	AttemptCount    int `json:"attemptCount,omitempty" yaml:"attemptCount,omitempty"`
	CompletionCount int `json:"completionCount,omitempty" yaml:"completionCount,omitempty"`
}

type SearchOptions struct {
	Search       string
	Kinds        []string
	Categories   []string
	Tags         []string
	Difficulties []string
	Limit        int
	Offset       int
}

// SearchResult is a page of hits plus the total number of matches (from the
// X-Total-Count header) so callers can render "showing X of N".
type SearchResult struct {
	Items []SearchItem
	Total int
}

func (c *Client) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	query := url.Values{}
	if opts.Search != "" {
		query.Set("search", opts.Search)
	}
	for _, kind := range opts.Kinds {
		query.Add("kind", kind)
	}
	for _, category := range opts.Categories {
		query.Add("category", category)
	}
	for _, tag := range opts.Tags {
		query.Add("tag", tag)
	}
	for _, difficulty := range opts.Difficulties {
		query.Add("difficulty", difficulty)
	}
	if opts.Limit > 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		query.Set("offset", strconv.Itoa(opts.Offset))
	}

	resp, err := c.Get(ctx, "/search", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Items []SearchItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	// The endpoint paginates via X-Total-Count; fall back to the page size when
	// the header is missing so "showing X of N" still reads sensibly.
	total := len(body.Items)
	if v := resp.Header.Get("X-Total-Count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			total = n
		}
	}

	return &SearchResult{Items: body.Items, Total: total}, nil
}
