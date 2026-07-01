// Package mcp exposes ketch's search, code, docs, scrape, and crawl
// capabilities as Model Context Protocol (MCP) tools. Each tool adapter calls
// the same underlying packages (search, code, docs, scrape, crawl) the Cobra
// commands in cmd/ call, through the same config-driven constructors
// (search.NewFromConfig etc.), and resolves backends from the same
// *config.Config an agent's human counterpart configures via
// `ketch config set`.
//
// The go-sdk dispatches tool calls concurrently, so everything with process
// lifetime (the headless-browser scraper, the bbolt page cache, the compiled
// URL rewriter) is constructed once in NewServer, shared by all calls, and
// released in Close — never per call.
package mcp

import (
	"context"

	"github.com/1broseidon/ketch/cache"
	"github.com/1broseidon/ketch/config"
	"github.com/1broseidon/ketch/scrape"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverInstructions is returned in the initialize result so MCP clients can
// inject it into the calling agent's context. Keep it short: tool routing,
// where defaults come from, the error taxonomy, and output-size advice.
const serverInstructions = `ketch provides five read-only research tools: search (web search), code (grep public OSS repos for real-world usage), docs (curated library/API documentation via Context7), scrape (fetch URLs as clean markdown), and crawl (bounded same-host multi-page crawl).
Prefer search for the open web, code for code examples, docs for library references, scrape when you already have the URL, and crawl only when one page is not enough.
Backend defaults and API keys come from the operator's ketch config; omit the backend argument to use them (on the CLI, ` + "`ketch config`" + ` shows the effective settings).
Tool errors start with a stable prefix: [validation] and [not_found] mean fix your input (retrying unchanged will not help); [upstream] is a backend/network failure where retrying may help; [precondition] means the operator must configure something (e.g. an API key or browser); [cancelled] means the call was cancelled or timed out.
When scraping unknown or potentially large pages, set max_chars (and optionally trim) to bound the response size.`

// Server bundles the SDK server with the shared, server-lifetime resources
// the tool handlers use. Construct with NewServer, run with Run, and always
// Close when done (it shuts down the headless browser and releases the cache
// file lock).
type Server struct {
	cfg     *config.Config
	mcp     *mcpsdk.Server
	scraper *scrape.Scraper // one scraper (and lazy browser conn) for all calls
	cache   *cache.Cache    // one bbolt handle for all calls; nil if unavailable
}

// NewServer builds an MCP server named "ketch" exposing the search, code,
// docs, scrape, and crawl tools, backed by cfg for backend selection and API
// keys. Background crawls, cache admin, and config stay CLI-only.
//
// The returned error is a precondition failure (invalid url_rewrites config).
// A nil cache (e.g. another long-lived process holds the bbolt lock) is not
// an error: the server runs with caching disabled, exactly like the CLI.
func NewServer(cfg *config.Config, version string) (*Server, error) {
	scraper, err := scrape.NewFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:     cfg,
		scraper: scraper,
		cache:   cache.NewFromConfig(cfg),
		mcp: mcpsdk.NewServer(&mcpsdk.Implementation{
			Name:    "ketch",
			Version: version,
		}, &mcpsdk.ServerOptions{
			Instructions: serverInstructions,
		}),
	}

	s.registerSearchTool()
	s.registerCodeTool()
	s.registerDocsTool()
	s.registerScrapeTool()
	s.registerCrawlTool()

	return s, nil
}

// Run runs the server over the given transport until the client disconnects
// or ctx is cancelled. Call Close afterwards to release shared resources.
func (s *Server) Run(ctx context.Context, t mcpsdk.Transport) error {
	return s.mcp.Run(ctx, t)
}

// Close releases the server-lifetime resources: kills the headless browser
// (if one was launched) and closes the page cache. Safe to call once Run has
// returned; both underlying Closes are nil-safe.
func (s *Server) Close() {
	s.scraper.Close()
	s.cache.Close()
}

// pageCache returns the shared cache handle, or nil when the caller asked to
// bypass caching for this call.
func (s *Server) pageCache(noCache bool) *cache.Cache {
	if noCache {
		return nil
	}
	return s.cache
}

// readOnlyOpenWorld marks a tool as a non-mutating fetcher that talks to the
// open web. All ketch tools are read-only network fetchers.
func readOnlyOpenWorld() *mcpsdk.ToolAnnotations {
	openWorld := true
	return &mcpsdk.ToolAnnotations{
		ReadOnlyHint:  true,
		OpenWorldHint: &openWorld,
	}
}
