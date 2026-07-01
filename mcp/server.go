// Package mcp exposes ketch's search, code, docs, and scrape capabilities as
// Model Context Protocol (MCP) tools. Each tool adapter calls the same
// underlying packages (search, code, docs, scrape) the Cobra commands in
// cmd/ call, and resolves backends from the same *config.Config an agent's
// human counterpart configures via `ketch config set`.
package mcp

import (
	"github.com/1broseidon/ketch/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer builds an MCP server named "ketch" exposing the search, code,
// docs, and scrape tools, backed by cfg for backend selection and API keys.
// Crawl, cache, and config are intentionally out of scope for this server.
func NewServer(cfg *config.Config, version string) *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "ketch",
		Version: version,
	}, nil)

	registerSearchTool(server, cfg)
	registerCodeTool(server, cfg)
	registerDocsTool(server, cfg)
	registerScrapeTool(server, cfg)

	return server
}
