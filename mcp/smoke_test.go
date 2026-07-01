//go:build mcpsmoke

// This smoke test exercises the real `ketch mcp serve` binary end to end
// over stdio, including one live network call (grepapp code search, which
// needs no API key). It is gated behind the mcpsmoke build tag so `go test
// ./...` stays hermetic in CI; run it explicitly with:
//
//	go test -tags mcpsmoke ./mcp/... -run TestMCPServerSmoke -v
package mcp_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerSmoke(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "ketch-mcp-smoke")
	build := exec.Command("go", "build", "-o", bin, "..")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("failed to build ketch: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "smoke-test", Version: "v0.0.0"}, nil)
	transport := &mcpsdk.CommandTransport{Command: exec.Command(bin, "mcp", "serve")}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	toolsRes, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools failed: %v", err)
	}
	byName := make(map[string]*mcpsdk.Tool, len(toolsRes.Tools))
	for _, tool := range toolsRes.Tools {
		byName[tool.Name] = tool
	}
	for _, want := range []string{"search", "code", "docs", "scrape"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("expected tool %q in tools/list, got: %v", want, toolNames(toolsRes.Tools))
		}
	}
	t.Logf("tools/list: %v", toolNames(toolsRes.Tools))
	if search, ok := byName["search"]; ok {
		t.Logf("search input schema: %+v", search.InputSchema)
	}

	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "code",
		Arguments: map[string]any{"query": "func main", "lang": "go", "limit": 1},
	})
	if err != nil {
		t.Fatalf("call code tool failed: %v", err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*mcpsdk.TextContent); ok {
				t.Logf("error content: %s", tc.Text)
			}
		}
		t.Fatalf("code tool returned an error result: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatalf("code tool returned no content")
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			t.Logf("code tool call result text: %s", tc.Text)
		}
	}
	t.Logf("code tool call structured content: %+v", res.StructuredContent)
}

func toolNames(tools []*mcpsdk.Tool) []string {
	out := make([]string, len(tools))
	for i, tool := range tools {
		out[i] = tool.Name
	}
	return out
}
