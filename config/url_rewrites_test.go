package config

import (
	"encoding/json"
	"testing"

	"github.com/1broseidon/ketch/urlrewrite"
)

func TestConfigJSONRoundTripsURLRewrites(t *testing.T) {
	in := Defaults()
	in.URLRewrites = []urlrewrite.Rule{
		{Match: `^https?://www\.reddit\.com/(.*)$`, Replace: "https://old.reddit.com/$1"},
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out Config
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.URLRewrites) != 1 {
		t.Fatalf("want 1 rule, got %d", len(out.URLRewrites))
	}
	if out.URLRewrites[0].Match != in.URLRewrites[0].Match {
		t.Errorf("Match mismatch: %q vs %q", out.URLRewrites[0].Match, in.URLRewrites[0].Match)
	}
	if out.URLRewrites[0].Replace != in.URLRewrites[0].Replace {
		t.Errorf("Replace mismatch: %q vs %q", out.URLRewrites[0].Replace, in.URLRewrites[0].Replace)
	}
}

func TestDefaultsHasEmptyURLRewrites(t *testing.T) {
	d := Defaults()
	if len(d.URLRewrites) != 0 {
		t.Errorf("Defaults().URLRewrites should be empty, got %v", d.URLRewrites)
	}
}

func TestConfigJSONOmitsEmptyURLRewrites(t *testing.T) {
	c := Defaults()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// omitempty: empty URLRewrites must not appear in serialized JSON
	if jsonContains(data, []byte(`"url_rewrites"`)) {
		t.Errorf("empty URLRewrites should be omitted from JSON, got: %s", data)
	}
}

func jsonContains(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
