package urlrewrite

import "testing"

func TestNewRewriterEmptyIsNil(t *testing.T) {
	rw, err := NewRewriter(nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rw != nil {
		t.Fatalf("want nil rewriter for empty rules, got %v", rw)
	}
}

func TestNewRewriterInvalidRegex(t *testing.T) {
	_, err := NewRewriter([]Rule{{Match: "[", Replace: "x"}})
	if err == nil {
		t.Fatalf("want compile error for invalid regex")
	}
}

func TestApplyNilRewriterIsIdentity(t *testing.T) {
	var rw *Rewriter
	got := rw.Apply("https://example.com/x")
	if got != "https://example.com/x" {
		t.Fatalf("nil rewriter must be identity, got %q", got)
	}
}

func TestApplyNoMatchReturnsOriginal(t *testing.T) {
	rw, _ := NewRewriter([]Rule{{Match: `^https?://foo\.com/(.*)$`, Replace: "https://bar.com/$1"}})
	got := rw.Apply("https://example.com/x")
	if got != "https://example.com/x" {
		t.Fatalf("no-match must return input, got %q", got)
	}
}

func TestApplyFirstMatchWins(t *testing.T) {
	rw, _ := NewRewriter([]Rule{
		{Match: `^https?://www\.reddit\.com/(.*)$`, Replace: "https://old.reddit.com/$1"},
		{Match: `^https?://reddit\.com/(.*)$`, Replace: "https://example.com/$1"},
	})
	got := rw.Apply("https://www.reddit.com/r/golang")
	if got != "https://old.reddit.com/r/golang" {
		t.Fatalf("first rule must win, got %q", got)
	}
}

func TestApplyCaptureGroup(t *testing.T) {
	rw, _ := NewRewriter([]Rule{
		{Match: `^(https://www\.theguardian\.com/uk)$`, Replace: "$1/rss"},
	})
	got := rw.Apply("https://www.theguardian.com/uk")
	if got != "https://www.theguardian.com/uk/rss" {
		t.Fatalf("capture-group rewrite failed, got %q", got)
	}
}
