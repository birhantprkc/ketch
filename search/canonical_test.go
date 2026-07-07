package search

import "testing"

// TestCanonicalURLKeys pins one row per canonicalization rule (§3) plus the
// combined worked example, so a regression in any single rule is visible.
func TestCanonicalURLKeys(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		// Rule 1: unparseable / hostless input is returned verbatim.
		{"hostless plain string", "not a url", "not a url"},
		{"scheme-only garbage", "::::", "::::"},
		// Rule 2: lowercase scheme and host, never the path.
		{"lowercase host", "https://EXAMPLE.com/Path", "https://example.com/Path"},
		{"path case preserved", "https://example.com/README", "https://example.com/README"},
		// Rule 3: fold http -> https.
		{"http folds to https", "http://example.com/a", "https://example.com/a"},
		// Rule 4: strip default ports.
		{"strip 443", "https://example.com:443/a", "https://example.com/a"},
		{"strip 80 after fold", "http://example.com:80/a", "https://example.com/a"},
		{"keep non-default port", "https://example.com:8443/a", "https://example.com:8443/a"},
		// Rule 5: strip a single leading www.
		{"strip www", "https://www.example.com/a", "https://example.com/a"},
		{"only one www stripped", "https://www.www.example.com/a", "https://www.example.com/a"},
		// Rule 6: drop the fragment.
		{"drop fragment", "https://example.com/a#section", "https://example.com/a"},
		// Rule 7: strip tracking params, keep meaningful ones.
		{"strip utm_", "https://example.com/a?utm_source=x&utm_medium=y", "https://example.com/a"},
		{"strip blocklist", "https://example.com/a?fbclid=z&gclid=q", "https://example.com/a"},
		{"keep selecting param", "https://example.com/watch?v=abc123", "https://example.com/watch?v=abc123"},
		{"strip tracking keep selecting", "https://example.com/watch?v=abc&utm_source=x", "https://example.com/watch?v=abc"},
		// Rule 8: surviving params sorted by name.
		{"params sorted", "https://example.com/a?b=2&a=1", "https://example.com/a?a=1&b=2"},
		// Rule 9: strip one trailing slash unless path is "/" or empty.
		{"strip trailing slash", "https://example.com/a/b/", "https://example.com/a/b"},
		{"root slash preserved", "https://example.com/", "https://example.com/"},
		{"empty path becomes slash", "https://example.com", "https://example.com/"},
		// Rule 10 non-goals: these are NOT normalized (must stay distinct keys).
		{"index.html kept", "https://example.com/index.html", "https://example.com/index.html"},
		{"mobile host kept", "https://m.example.com/a", "https://m.example.com/a"},
		// Combined worked example from §3.
		{"combined", "HTTP://www.Example.com:80/a/b/?utm_source=x&id=3#frag", "https://example.com/a/b?id=3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalURL(tc.raw); got != tc.want {
				t.Errorf("canonicalURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestCanonicalURLMerge covers the merge/no-merge decisions: representations of
// the same page must share a key; genuinely different pages must not.
func TestCanonicalURLMerge(t *testing.T) {
	cases := []struct {
		name      string
		a, b      string
		mustMerge bool
	}{
		{"http vs https", "http://example.com/a", "https://example.com/a", true},
		{"www vs bare", "https://www.example.com/a", "https://example.com/a", true},
		{"trailing slash", "https://example.com/a/", "https://example.com/a", true},
		{"fragment differs", "https://example.com/a#x", "https://example.com/a#y", true},
		{"utm soup", "https://example.com/a?utm_source=x", "https://example.com/a?utm_campaign=z", true},
		{"different query value", "https://example.com/watch?v=x", "https://example.com/watch?v=y", false},
		{"pagination differs", "https://example.com/a?page=1", "https://example.com/a?page=2", false},
		{"path case differs", "https://example.com/README", "https://example.com/readme", false},
		{"mobile host differs", "https://m.example.com/a", "https://example.com/a", false},
		{"unparseable identity", "not a url", "also not a url", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ka, kb := canonicalURL(tc.a), canonicalURL(tc.b)
			if merged := ka == kb; merged != tc.mustMerge {
				t.Errorf("merge(%q, %q) = %v (keys %q, %q), want %v",
					tc.a, tc.b, merged, ka, kb, tc.mustMerge)
			}
		})
	}
}

// TestCanonicalURLMalformedQuery pins the under-merge fallback: a query
// string that fails url.ParseQuery (mangled percent-encoding, legacy
// semicolon separators) is kept verbatim in the key instead of being
// silently dropped, so distinct-but-malformed pages never collapse.
func TestCanonicalURLMalformedQuery(t *testing.T) {
	cases := []struct {
		name      string
		a, b      string
		mustMerge bool
	}{
		{"mangled percent params stay distinct", "https://example.com/p?id=%zz1", "https://example.com/p?id=%zz2", false},
		{"mangled param vs bare page stays distinct", "https://example.com/p?id=%zz1", "https://example.com/p", false},
		{"semicolon query vs bare page stays distinct", "https://example.com/p?a=1;b=2", "https://example.com/p", false},
		{"identical mangled queries still merge", "https://example.com/p?id=%zz1", "https://example.com/p?id=%zz1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ka, kb := canonicalURL(tc.a), canonicalURL(tc.b)
			if merged := ka == kb; merged != tc.mustMerge {
				t.Errorf("merge(%q, %q) = %v (keys %q, %q), want %v",
					tc.a, tc.b, merged, ka, kb, tc.mustMerge)
			}
		})
	}
}
