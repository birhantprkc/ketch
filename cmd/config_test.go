package cmd

import (
	"strings"
	"testing"

	"github.com/1broseidon/ketch/config"
)

func TestApplyConfigSetURLRewritesValidJSON(t *testing.T) {
	c := config.Defaults()
	err := applyConfigSet(&c, "url_rewrites", `[{"match":"^https?://www\\.reddit\\.com/(.*)$","replace":"https://old.reddit.com/$1"}]`)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(c.URLRewrites) != 1 {
		t.Fatalf("want 1 rule, got %d", len(c.URLRewrites))
	}
	if c.URLRewrites[0].Replace != "https://old.reddit.com/$1" {
		t.Errorf("Replace mismatch: %q", c.URLRewrites[0].Replace)
	}
}

func TestApplyConfigSetURLRewritesInvalidJSON(t *testing.T) {
	c := config.Defaults()
	err := applyConfigSet(&c, "url_rewrites", `not json`)
	if err == nil {
		t.Fatalf("want JSON parse error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "json") {
		t.Errorf("error should mention JSON, got: %v", err)
	}
}

func TestApplyConfigSetURLRewritesInvalidRegex(t *testing.T) {
	c := config.Defaults()
	err := applyConfigSet(&c, "url_rewrites", `[{"match":"[","replace":"x"}]`)
	if err == nil {
		t.Fatalf("want regex compile error")
	}
}

func TestApplyConfigSetURLRewritesEmptyClears(t *testing.T) {
	c := config.Defaults()
	if err := applyConfigSet(&c, "url_rewrites", `[{"match":"^x$","replace":"y"}]`); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if len(c.URLRewrites) != 1 {
		t.Fatalf("setup failed: %d rules", len(c.URLRewrites))
	}
	if err := applyConfigSet(&c, "url_rewrites", `[]`); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(c.URLRewrites) != 0 {
		t.Errorf("want empty list after [] reset, got %d rules", len(c.URLRewrites))
	}
}

func TestApplyConfigSetUnknownKey(t *testing.T) {
	c := config.Defaults()
	err := applyConfigSet(&c, "no_such_key", "x")
	if err == nil {
		t.Fatalf("want unknown-key error")
	}
}
