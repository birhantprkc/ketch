// Package urlrewrite applies an ordered list of regex-based rewrite rules
// to URLs. It is used to transparently swap or transform request URLs at
// every fetch entry point so that the agent surface is unchanged but the
// underlying fetch can hit a more cooperative host (e.g. old.reddit.com).
package urlrewrite

import (
	"fmt"
	"regexp"
)

// Rule is one rewrite: if Match (a Go regexp) matches the input URL, the URL
// is replaced with Replace, with $1..$N capture-group expansion.
type Rule struct {
	Match   string `json:"match"`
	Replace string `json:"replace"`
}

// Rewriter is a compiled, ordered set of Rules. First match wins.
type Rewriter struct {
	compiled []compiledRule
}

type compiledRule struct {
	re      *regexp.Regexp
	replace string
}

// NewRewriter compiles rules. Empty/nil input returns (nil, nil) so callers
// can treat "no rewriter" as a fast path without nil checks at the call site.
func NewRewriter(rules []Rule) (*Rewriter, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	compiled := make([]compiledRule, 0, len(rules))
	for i, r := range rules {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			return nil, fmt.Errorf("url_rewrites[%d]: %w", i, err)
		}
		compiled = append(compiled, compiledRule{re: re, replace: r.Replace})
	}
	return &Rewriter{compiled: compiled}, nil
}

// Apply returns the rewritten URL, or the original if no rule matches.
// Safe to call on a nil receiver (acts as identity).
func (r *Rewriter) Apply(url string) string {
	if r == nil {
		return url
	}
	for _, c := range r.compiled {
		if c.re.MatchString(url) {
			return c.re.ReplaceAllString(url, c.replace)
		}
	}
	return url
}
