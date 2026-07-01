package mcp

import (
	"context"
	"fmt"

	"github.com/1broseidon/ketch/config"
	"github.com/1broseidon/ketch/search"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchInput is the input schema for the "search" tool.
type SearchInput struct {
	Query   string `json:"query" jsonschema:"the search query"`
	Backend string `json:"backend,omitempty" jsonschema:"search backend: brave, ddg, searxng, or exa (default: the configured backend)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max number of results (default: the configured limit)"`
}

// SearchOutput is the output schema for the "search" tool. It mirrors the
// CLI's `ketch search --json` output shape.
type SearchOutput struct {
	Results []search.Result `json:"results"`
}

func registerSearchTool(s *mcpsdk.Server, cfg *config.Config) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "search",
		Description: "Search the web using Brave (default), DuckDuckGo, SearXNG, or Exa and return results (title, url, description).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in SearchInput) (*mcpsdk.CallToolResult, SearchOutput, error) {
		backend := in.Backend
		if backend == "" {
			backend = cfg.Backend
		}
		limit := in.Limit
		if limit <= 0 {
			limit = cfg.Limit
		}

		searcher, err := newSearcher(cfg, backend)
		if err != nil {
			return nil, SearchOutput{}, err
		}

		results, err := searcher.Search(ctx, in.Query, limit)
		if err != nil {
			return nil, SearchOutput{}, fmt.Errorf("search failed: %w", err)
		}

		return nil, SearchOutput{Results: results}, nil
	})
}

// newSearcher mirrors cmd.newSearcher's backend switch so the MCP tool picks
// backends and API keys exactly as the `ketch search` CLI command does.
func newSearcher(cfg *config.Config, backend string) (search.Searcher, error) {
	switch backend {
	case "brave":
		if cfg.BraveAPIKey == "" {
			return nil, fmt.Errorf("brave: API key not set (set with: ketch config set brave_api_key <key>)")
		}
		return search.NewBrave(cfg.BraveAPIKey), nil
	case "searxng":
		return search.NewSearXNG(cfg.SearxngURL), nil
	case "ddg":
		return search.NewDDG(), nil
	case "exa":
		var apiKey *string
		if cfg.ExaAPIKey != "" {
			apiKey = &cfg.ExaAPIKey
		}
		return search.NewEXA(apiKey), nil
	default:
		return nil, fmt.Errorf("unknown backend: %s", backend)
	}
}
