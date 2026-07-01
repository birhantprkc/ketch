package mcp

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/1broseidon/ketch/cache"
	"github.com/1broseidon/ketch/config"
	"github.com/1broseidon/ketch/extract"
	"github.com/1broseidon/ketch/scrape"
	"github.com/1broseidon/ketch/urlrewrite"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ScrapeInput is the input schema for the "scrape" tool. Phase 1 supports a
// single URL per call; the CLI's multi-URL/file/stdin input detection is not
// exposed here.
type ScrapeInput struct {
	URL      string `json:"url" jsonschema:"the URL to scrape"`
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector to extract specific elements, skips readability"`
	Trim     bool   `json:"trim,omitempty" jsonschema:"strip markdown formatting, keep content text only"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"truncate markdown output to N characters (0 = disabled)"`
	NoCache  bool   `json:"no_cache,omitempty" jsonschema:"bypass the page cache"`
}

// ScrapeOutput is the output schema for the "scrape" tool. It mirrors the
// CLI's `ketch scrape --json` page shape.
type ScrapeOutput struct {
	URL        string `json:"url"`
	FetchedURL string `json:"fetched_url,omitempty"`
	Title      string `json:"title"`
	Markdown   string `json:"markdown"`
}

func registerScrapeTool(s *mcpsdk.Server, cfg *config.Config) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "scrape",
		Description: "Fetch a single URL, extract the main content, and convert it to clean markdown.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in ScrapeInput) (*mcpsdk.CallToolResult, ScrapeOutput, error) {
		if in.URL == "" {
			return nil, ScrapeOutput{}, fmt.Errorf("url is required")
		}

		scraper, err := newMCPScraper(cfg)
		if err != nil {
			return nil, ScrapeOutput{}, err
		}
		defer scraper.Close()

		pc := newMCPPageCache(cfg, in.NoCache)
		defer pc.Close()

		var page *scrape.Page
		if in.Selector != "" {
			page, err = scrapeWithSelector(ctx, scraper, in.URL, in.Selector)
		} else {
			page, err = cachedScrape(ctx, scraper, pc, in.URL)
		}
		if err != nil {
			return nil, ScrapeOutput{}, fmt.Errorf("scrape failed: %w", err)
		}

		return nil, ScrapeOutput{
			URL:        page.URL,
			FetchedURL: page.FetchedURL,
			Title:      page.Title,
			Markdown:   postProcess(page.Markdown, in.Trim, in.MaxChars),
		}, nil
	})
}

// newMCPScraper mirrors cmd.newScraper: builds a Scraper from cfg's compiled
// URL rewriter and optional browser binary. Returned scraper must be Closed
// by the caller.
func newMCPScraper(cfg *config.Config) (*scrape.Scraper, error) {
	rw, err := urlrewrite.NewRewriter(cfg.URLRewrites)
	if err != nil {
		return nil, fmt.Errorf("invalid url_rewrites: %w", err)
	}
	return scrape.NewWithConfig(cfg.Browser, rw, cfg.SPAMarkers), nil
}

// newMCPPageCache mirrors cmd.newPageCache: creates a cache from cfg, or nil
// if disabled.
func newMCPPageCache(cfg *config.Config, noCache bool) *cache.Cache {
	if noCache {
		return nil
	}
	ttl, err := time.ParseDuration(cfg.CacheTTL)
	if err != nil {
		ttl = time.Hour
	}
	return cache.New(ttl)
}

// cachedScrape mirrors cmd.cachedScrape: checks the cache first, falls back
// to fetch+extract.
func cachedScrape(ctx context.Context, s *scrape.Scraper, pc *cache.Cache, url string) (*scrape.Page, error) {
	key := s.Rewrite(url)
	if page, source := pc.Get(key); page != nil && !scrape.CacheStaleForBrowser(source, s.HasBrowser()) {
		return page, nil
	}

	page, source, err := s.Scrape(ctx, url)
	if err != nil {
		return nil, err
	}

	pc.Put(key, page, source)
	return page, nil
}

// scrapeWithSelector mirrors cmd.scrapeURLWithSelector: fetches a URL and
// extracts content matching a CSS selector, bypassing the page cache and
// readability extraction.
func scrapeWithSelector(ctx context.Context, s *scrape.Scraper, rawURL, selector string) (*scrape.Page, error) {
	fetchURL := s.Rewrite(rawURL)
	html, err := s.Fetch(ctx, fetchURL)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	html, _ = s.MaybeBrowserFetch(ctx, fetchURL, html)

	markdown, err := extract.ExtractSelector(html, selector)
	if err != nil {
		return nil, fmt.Errorf("selector extraction failed: %w", err)
	}
	if markdown == "" {
		return nil, fmt.Errorf("no elements matched selector %q", selector)
	}

	page := &scrape.Page{URL: rawURL, Title: extract.Title(html), Markdown: markdown}
	if fetchURL != rawURL {
		page.FetchedURL = fetchURL
	}
	return page, nil
}

// postProcess mirrors cmd.postProcess: applies trim then truncate to
// markdown content.
func postProcess(s string, trim bool, maxChars int) string {
	if trim {
		s = extract.StripMarkdown(s)
	}
	return truncateContent(s, maxChars)
}

// truncateContent mirrors cmd.truncateContent: caps s at maxChars Unicode
// code points, appending a truncation marker.
func truncateContent(s string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxChars]) + "\n\n[truncated]"
}
