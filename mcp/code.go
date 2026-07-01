package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/1broseidon/ketch/code"
	"github.com/1broseidon/ketch/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// CodeInput is the input schema for the "code" tool.
type CodeInput struct {
	Query   string `json:"query" jsonschema:"the code search query"`
	Backend string `json:"backend,omitempty" jsonschema:"code search backend: grepapp, sourcegraph, or github (default: the configured backend)"`
	Lang    string `json:"lang,omitempty" jsonschema:"language filter appended to the query"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max number of results (default: the configured limit)"`
	Regexp  bool   `json:"regexp,omitempty" jsonschema:"interpret query as a regular expression (grepapp, sourcegraph only)"`
}

// CodeOutput is the output schema for the "code" tool. It mirrors the CLI's
// `ketch code --json` output shape.
type CodeOutput struct {
	Results []code.Result `json:"results"`
}

func registerCodeTool(s *mcpsdk.Server, cfg *config.Config) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "code",
		Description: "Search code across open-source repositories using Grep (default; mcp.grep.app), Sourcegraph, or GitHub Code Search.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in CodeInput) (*mcpsdk.CallToolResult, CodeOutput, error) {
		backend := in.Backend
		if backend == "" {
			backend = cfg.CodeBackend
		}
		limit := in.Limit
		if limit <= 0 {
			limit = cfg.Limit
		}

		searcher, err := newCodeSearcher(cfg, backend)
		if err != nil {
			return nil, CodeOutput{}, err
		}

		results, err := searcher.Search(ctx, code.Query{
			Term:   in.Query,
			Lang:   in.Lang,
			Limit:  limit,
			Regexp: in.Regexp,
		})
		if err != nil {
			if errors.Is(err, code.ErrRegexpUnsupported) {
				return nil, CodeOutput{}, fmt.Errorf("backend %q does not support regexp (try backend=grepapp or backend=sourcegraph)", backend)
			}
			return nil, CodeOutput{}, fmt.Errorf("code search failed: %w", err)
		}

		return nil, CodeOutput{Results: results}, nil
	})
}

// newCodeSearcher mirrors cmd.newCodeSearcher's backend switch so the MCP
// tool picks backends and tokens exactly as the `ketch code` CLI command
// does.
func newCodeSearcher(cfg *config.Config, backend string) (code.Searcher, error) {
	switch backend {
	case "sourcegraph":
		return code.NewSourcegraph(cfg.SourcegraphURL), nil
	case "grepapp":
		return code.NewGrepApp(), nil
	case "github":
		token, _ := cfg.ResolveGithubToken()
		if token == "" {
			return nil, fmt.Errorf(`github code search: no token found.
  - explicit:   ketch config set github_token <token>
  - env var:    export GITHUB_TOKEN=<token>
  - or run:     gh auth login`)
		}
		return code.NewGitHub(token), nil
	default:
		return nil, fmt.Errorf("unknown code backend: %s", backend)
	}
}
