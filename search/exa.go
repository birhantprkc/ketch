package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/1broseidon/ketch/httpx"
)

type EXA struct {
	apiKey *string
	client *http.Client
}

func NewEXA(apiKey *string) *EXA {
	return &EXA{
		apiKey: apiKey,
		client: httpx.Default(),
	}
}

func (e *EXA) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	// Step 1 : Build request body:
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "web_search_exa",
			"arguments": map[string]any{
				"query":                query,
				"numResults":           limit,
				"type":                 "auto",
				"livecrawl":            "fallback",
				"contextMaxCharacters": 3000,
			},
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Step 2 : Send this request to EXA
	endpoint := "https://mcp.exa.ai/mcp"
	if e.apiKey != nil && strings.TrimSpace(*e.apiKey) != "" {
		v := url.Values{}
		v.Set("exaApiKey", strings.TrimSpace(*e.apiKey))
		endpoint += "?" + v.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exa request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exa returned status %d", resp.StatusCode)
	}

	// Step 3 : Parse the SSE-like response
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read exa response: %w", err)
	}
	var payload string
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			payload = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	if payload == "" {
		return nil, fmt.Errorf("exa response contained no data payload")
	}

	var parsed struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil, fmt.Errorf("failed to decode exa response: %w", err)
	}

	// Step 4 : Populate response and return
	results := make([]Result, 0, limit)
	for _, content := range parsed.Result.Content {
		if content.Type != "text" || len(results) >= limit {
			continue
		}
		remaining := limit - len(results)
		results = append(results, parseContent(content.Text, remaining)...)
	}

	return results, nil
}

// parseContent converts Exa's text-formatted MCP search output into structured
// results. Exa returns result blocks separated by "---", with metadata lines
// such as "Title:" and "URL:" followed by highlight text. This parser extracts
// the title, URL, highlight text as content, and the first plain highlight
// line as the description.
func parseContent(rawContent string, limit int) []Result {
	results := make([]Result, 0, limit)
	for block := range strings.SplitSeq(rawContent, "\n---\n") {
		if len(results) >= limit {
			break
		}
		var result Result
		var contentLines []string
		for line := range strings.SplitSeq(block, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Title:"):
				result.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title:"))
			case strings.HasPrefix(line, "URL:"):
				result.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
			case line != "" && !strings.Contains(line, ":"):
				contentLines = append(contentLines, line)
				if result.Description == "" {
					result.Description = line
				}
			}
		}
		result.Content = strings.Join(contentLines, "\n")
		if result.Title != "" && result.URL != "" {
			results = append(results, result)
		}
	}
	return results
}
