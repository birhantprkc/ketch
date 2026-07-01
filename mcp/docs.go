package mcp

import (
	"context"
	"fmt"

	"github.com/1broseidon/ketch/config"
	"github.com/1broseidon/ketch/docs"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DocsInput is the input schema for the "docs" tool.
type DocsInput struct {
	Query   string `json:"query" jsonschema:"the docs search query, or a library name when resolve is true"`
	Backend string `json:"backend,omitempty" jsonschema:"docs backend: context7 or local (default: the configured backend)"`
	Library string `json:"library,omitempty" jsonschema:"Context7 library ID to fetch docs from directly, skipping the resolve step"`
	Tokens  int    `json:"tokens,omitempty" jsonschema:"Context7 token budget when library is set (default 4000)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max number of results (default: the configured limit)"`
	Resolve bool   `json:"resolve,omitempty" jsonschema:"resolve a library name to Context7 library IDs instead of searching docs"`
}

// DocsOutput is the output schema for the "docs" tool. Results is populated
// for a normal or library-scoped search; Matches is populated when Resolve
// is set. Exactly one of the two is non-empty for a given call.
type DocsOutput struct {
	Results []docs.Result       `json:"results,omitempty"`
	Matches []docs.LibraryMatch `json:"matches,omitempty"`
}

func registerDocsTool(s *mcpsdk.Server, cfg *config.Config) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "docs",
		Description: "Search library documentation using Context7 or a local FTS5 backend. Supports resolving a library name to a Context7 library ID, and fetching docs directly from a known library ID.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in DocsInput) (*mcpsdk.CallToolResult, DocsOutput, error) {
		backend := in.Backend
		if backend == "" {
			backend = cfg.DocsBackend
		}
		limit := in.Limit
		if limit <= 0 {
			limit = cfg.Limit
		}
		tokens := in.Tokens
		if tokens <= 0 {
			tokens = 4000
		}

		if in.Resolve {
			if cfg.Context7APIKey == "" {
				return nil, DocsOutput{}, fmt.Errorf("context7: API key not set (set with: ketch config set context7_api_key <key>)")
			}
			c7 := docs.NewContext7(cfg.Context7APIKey)
			matches, err := c7.ResolveLibrary(ctx, in.Query)
			if err != nil {
				return nil, DocsOutput{}, fmt.Errorf("resolve failed: %w", err)
			}
			return nil, DocsOutput{Matches: matches}, nil
		}

		if in.Library != "" && backend == "context7" {
			if cfg.Context7APIKey == "" {
				return nil, DocsOutput{}, fmt.Errorf("context7: API key not set (set with: ketch config set context7_api_key <key>)")
			}
			c7 := docs.NewContext7(cfg.Context7APIKey)
			results, err := c7.GetDocs(ctx, in.Library, in.Query, tokens)
			if err != nil {
				return nil, DocsOutput{}, fmt.Errorf("docs fetch failed: %w", err)
			}
			return nil, DocsOutput{Results: results}, nil
		}

		searcher, err := newDocSearcher(cfg, backend)
		if err != nil {
			return nil, DocsOutput{}, err
		}

		results, err := searcher.Search(ctx, in.Query, limit)
		if err != nil {
			return nil, DocsOutput{}, fmt.Errorf("docs search failed: %w", err)
		}

		return nil, DocsOutput{Results: results}, nil
	})
}

// newDocSearcher mirrors cmd.newDocSearcher's backend switch so the MCP tool
// picks backends and API keys exactly as the `ketch docs` CLI command does.
func newDocSearcher(cfg *config.Config, backend string) (docs.Searcher, error) {
	switch backend {
	case "context7":
		if cfg.Context7APIKey == "" {
			return nil, fmt.Errorf("context7: API key not set (set with: ketch config set context7_api_key <key>)")
		}
		return docs.NewContext7(cfg.Context7APIKey), nil
	case "local":
		return docs.NewFTS5Local(), nil
	default:
		return nil, fmt.Errorf("unknown docs backend: %s", backend)
	}
}
