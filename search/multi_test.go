package search

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// TestResultBackendsJSONOmitempty pins the additive JSON contract: a
// single-backend result (no Backends) marshals byte-identically to before
// (no "backends" key), while a fused result carries the field.
func TestResultBackendsJSONOmitempty(t *testing.T) {
	single, err := json.Marshal(Result{Title: "T", URL: "https://example.com/a", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(single), "backends") {
		t.Errorf("single-backend result must not emit a backends key: %s", single)
	}

	fused, err := json.Marshal(Result{Title: "T", URL: "https://example.com/a", Backends: []string{"brave", "ddg"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fused), `"backends":["brave","ddg"]`) {
		t.Errorf("fused result must carry backends: %s", fused)
	}
}

// url maps a short document id (u1) to a stable canonical URL. Path case and
// no trailing slash keep the canonical key equal to the URL, so fusion order
// reads directly off the ids.
func docURL(id string) string { return "https://example.com/" + id }

// list builds one backend's ranked result list from document ids in rank order.
func list(name string, ids ...string) backendResults {
	br := backendResults{name: name}
	for _, id := range ids {
		br.results = append(br.results, Result{Title: id, URL: docURL(id)})
	}
	return br
}

// ids extracts the fused document ids (from the representative URL) in order.
func ids(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = strings.TrimPrefix(r.URL, "https://example.com/")
	}
	return out
}

func term(rank int) float64 { return 1.0 / float64(rrfK+rank) }

// TestRankFuseWorkedExample is the §2 worked example verbatim: three backends,
// depth 5 each, with hand-computed RRF scores and the full expected order.
func TestRankFuseWorkedExample(t *testing.T) {
	lists := []backendResults{
		list("brave", "u1", "u2", "u3", "u4", "u5"),
		list("ddg", "u2", "u1", "u6", "u3", "u7"),
		list("exa", "u1", "u8", "u2", "u9", "u6"),
	}

	ranked := rankFuse(lists)
	gotOrder := make([]string, len(ranked))
	for i, s := range ranked {
		gotOrder[i] = strings.TrimPrefix(s.result.URL, "https://example.com/")
	}
	wantOrder := []string{"u1", "u2", "u3", "u6", "u8", "u4", "u9", "u5", "u7"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("fused order = %v, want %v", gotOrder, wantOrder)
	}

	wantScore := map[string]float64{
		"u1": term(1) + term(2) + term(1),
		"u2": term(2) + term(1) + term(3),
		"u3": term(3) + term(4),
		"u6": term(3) + term(5),
		"u8": term(2),
		"u4": term(4),
		"u9": term(4),
		"u5": term(5),
		"u7": term(5),
	}
	for _, s := range ranked {
		id := strings.TrimPrefix(s.result.URL, "https://example.com/")
		if math.Abs(s.score-wantScore[id]) > 1e-9 {
			t.Errorf("score(%s) = %.9f, want %.9f", id, s.score, wantScore[id])
		}
	}

	// Backends list on the three-engine consensus doc is sorted + complete.
	if got := strings.Join(ranked[0].result.Backends, ","); got != "brave,ddg,exa" {
		t.Errorf("u1 backends = %q, want brave,ddg,exa", got)
	}

	// Cut to --limit 5 keeps the fused prefix.
	if got := ids(fuse(lists, 5)); strings.Join(got, ",") != "u1,u2,u3,u6,u8" {
		t.Errorf("limit-5 cut = %v, want u1,u2,u3,u6,u8", got)
	}
}

func TestRankFuseCases(t *testing.T) {
	cases := []struct {
		name  string
		lists []backendResults
		limit int
		want  []string
	}{
		{
			name:  "single backend preserves order",
			lists: []backendResults{list("brave", "a", "b", "c")},
			limit: 0,
			want:  []string{"a", "b", "c"},
		},
		{
			name: "two-backend full overlap",
			lists: []backendResults{
				list("brave", "a", "b", "c"),
				list("ddg", "a", "b", "c"),
			},
			limit: 0,
			want:  []string{"a", "b", "c"},
		},
		{
			name: "zero overlap interleaves by score then url",
			// each doc is a lone rank on one backend; ranks pair up
			// (a/x at 1, b/y at 2) and ties break by canonical URL.
			lists: []backendResults{
				list("brave", "a", "b"),
				list("ddg", "x", "y"),
			},
			limit: 0,
			want:  []string{"a", "x", "b", "y"},
		},
		{
			name: "asymmetric counts: consensus lifts a mid-list doc",
			// long list of 5 vs short list of 1 agreeing on the long list's
			// #4. d gets brave#4 + ddg#1 = 1/64 + 1/61 > a's lone 1/61, so
			// two-engine agreement outranks a single engine's #1.
			lists: []backendResults{
				list("brave", "a", "b", "c", "d", "e"),
				list("ddg", "d"),
			},
			limit: 0,
			want:  []string{"d", "a", "b", "c", "e"},
		},
		{
			name: "tie broken by min rank",
			// f: brave#1 only -> term(1). g: ddg#1 only -> term(1).
			// equal score+count -> min-rank equal -> canonical url: f<g.
			lists: []backendResults{
				list("brave", "f"),
				list("ddg", "g"),
			},
			limit: 0,
			want:  []string{"f", "g"},
		},
		{
			name:  "limit cut applied after fusion",
			lists: []backendResults{list("brave", "a", "b", "c", "d")},
			limit: 2,
			want:  []string{"a", "b"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(fuse(tc.lists, tc.limit)); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("fused = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRankFuseCanonicalURLTiebreak pins tiebreak rule 4 (canonical URL asc)
// on an equal-score, equal-count, equal-min-rank pair. Tiebreak rule 3 (min
// rank asc) is effectively untestable in isolation: with exact RRF sums, two
// docs with equal score and equal backend count end up with equal rank
// multisets, so their min ranks coincide too — rule 4 is what actually
// breaks the tie. Kept in the chain as a cheap determinism backstop.
func TestRankFuseCanonicalURLTiebreak(t *testing.T) {
	// Identical rank multiset via mirrored assignment: z=(1,4), a=(4,1).
	lists := []backendResults{
		list("brave", "z", "m", "n", "a"),
		list("ddg", "a", "p", "q", "z"),
	}
	// z: brave#1 + ddg#4 = term(1)+term(4). a: brave#4 + ddg#1 = term(4)+term(1).
	// Equal score, equal count, best rank both 1 -> canonical URL: a < z.
	ranked := rankFuse(lists)
	if got := strings.TrimPrefix(ranked[0].result.URL, "https://example.com/"); got != "a" {
		t.Fatalf("first = %q, want a (equal score, min-rank tie -> canonical url)", got)
	}
	if got := strings.TrimPrefix(ranked[1].result.URL, "https://example.com/"); got != "z" {
		t.Fatalf("second = %q, want z", got)
	}
}

// TestRankFuseMergeMetadata checks the representative selection and field
// fallback walk when entries collapse.
func TestRankFuseMergeMetadata(t *testing.T) {
	// Same doc from two backends; ddg ranks it best (rank 1) so it is the
	// representative, but its description is empty -> fall back to brave's.
	lists := []backendResults{
		{name: "brave", results: []Result{
			{Title: "unused", URL: "https://example.com/doc", Description: "brave snippet", Content: "brave body"},
		}},
		{name: "ddg", results: []Result{
			{Title: "DDG Title", URL: "https://www.example.com/doc/", Description: ""},
		}},
	}
	// brave rank 1, ddg rank 1 -> tie on rank, brave listed first -> brave is
	// representative (URL + Title from brave).
	ranked := rankFuse(lists)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 fused doc, got %d", len(ranked))
	}
	r := ranked[0].result
	if r.URL != "https://example.com/doc" {
		t.Errorf("representative URL = %q, want brave's original", r.URL)
	}
	if r.Title != "unused" {
		t.Errorf("representative Title = %q, want brave's", r.Title)
	}
	if r.Description != "brave snippet" {
		t.Errorf("description = %q, want brave snippet", r.Description)
	}
	if r.Content != "brave body" {
		t.Errorf("content = %q, want brave body", r.Content)
	}
	if strings.Join(r.Backends, ",") != "brave,ddg" {
		t.Errorf("backends = %v, want brave,ddg", r.Backends)
	}
}

// --- degradation tests (fake searchers) ---

type fakeSearcher struct {
	results []Result
	err     error
	delay   time.Duration
}

func (f *fakeSearcher) Search(ctx context.Context, _ string, _ int) ([]Result, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.results, f.err
}

func newTestMulti(timeout time.Duration, backends ...namedSearcher) *Multi {
	return &Multi{backends: backends, timeout: timeout}
}

func TestMultiSearchPartialFailure(t *testing.T) {
	m := newTestMulti(time.Second,
		namedSearcher{name: "brave", searcher: &fakeSearcher{results: []Result{{URL: docURL("a")}, {URL: docURL("b")}}}},
		namedSearcher{name: "ddg", searcher: &fakeSearcher{err: errors.New("boom")}},
	)
	results, errs, err := m.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 1 || errs[0].Backend != "ddg" {
		t.Fatalf("errs = %+v, want one ddg failure", errs)
	}
	if got := ids(results); strings.Join(got, ",") != "a,b" {
		t.Errorf("results = %v, want a,b from the surviving backend", got)
	}
}

func TestMultiSearchTimeout(t *testing.T) {
	m := newTestMulti(20*time.Millisecond,
		namedSearcher{name: "brave", searcher: &fakeSearcher{results: []Result{{URL: docURL("a")}}}},
		namedSearcher{name: "slow", searcher: &fakeSearcher{results: []Result{{URL: docURL("z")}}, delay: 500 * time.Millisecond}},
	)
	start := time.Now()
	results, errs, err := m.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Errorf("search took %v; the slow backend should have timed out at ~20ms", elapsed)
	}
	if len(errs) != 1 || errs[0].Backend != "slow" || !errors.Is(errs[0].Err, context.DeadlineExceeded) {
		t.Fatalf("errs = %+v, want one slow DeadlineExceeded failure", errs)
	}
	if got := ids(results); strings.Join(got, ",") != "a" {
		t.Errorf("results = %v, want just a", got)
	}
}

func TestMultiSearchAllFail(t *testing.T) {
	m := newTestMulti(time.Second,
		namedSearcher{name: "brave", searcher: &fakeSearcher{err: errors.New("brave down")}},
		namedSearcher{name: "ddg", searcher: &fakeSearcher{err: errors.New("ddg down")}},
	)
	results, errs, err := m.Search(context.Background(), "q", 5)
	if err == nil {
		t.Fatal("expected an error when all backends fail")
	}
	if results != nil {
		t.Errorf("results = %v, want nil", results)
	}
	if len(errs) != 2 {
		t.Fatalf("errs = %+v, want both backends", errs)
	}
	msg := err.Error()
	if !strings.Contains(msg, "all 2 backends failed") || !strings.Contains(msg, "brave") || !strings.Contains(msg, "ddg") {
		t.Errorf("error = %q, want it to name all failed backends", msg)
	}
}

func TestMultiSearchEmptySuccessIsNotFailure(t *testing.T) {
	m := newTestMulti(time.Second,
		namedSearcher{name: "brave", searcher: &fakeSearcher{results: []Result{{URL: docURL("a")}}}},
		namedSearcher{name: "ddg", searcher: &fakeSearcher{results: nil}}, // 0 results, no error
	)
	results, errs, err := m.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("errs = %+v, want none (empty success is not a failure)", errs)
	}
	if got := ids(results); strings.Join(got, ",") != "a" {
		t.Errorf("results = %v, want a", got)
	}
}

func TestMultiSearchFanOutIsParallel(t *testing.T) {
	m := newTestMulti(time.Second,
		namedSearcher{name: "brave", searcher: &fakeSearcher{results: []Result{{URL: docURL("a")}}, delay: 50 * time.Millisecond}},
		namedSearcher{name: "ddg", searcher: &fakeSearcher{results: []Result{{URL: docURL("b")}}, delay: 50 * time.Millisecond}},
	)
	start := time.Now()
	if _, _, err := m.Search(context.Background(), "q", 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 90*time.Millisecond {
		t.Errorf("two 50ms backends took %v; expected parallel (<90ms)", elapsed)
	}
}

func TestMultiSearchFetchDepthFloor(t *testing.T) {
	// --limit 3 must still fetch at least the floor (10) from each backend so
	// fusion has overlap evidence below the final cutoff.
	var gotLimit int
	rec := &recordingSearcher{onSearch: func(limit int) { gotLimit = limit }}
	m := newTestMulti(time.Second, namedSearcher{name: "brave", searcher: rec})
	if _, _, err := m.Search(context.Background(), "q", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != fetchFloor {
		t.Errorf("per-backend fetch limit = %d, want floor %d", gotLimit, fetchFloor)
	}
}

type recordingSearcher struct {
	onSearch func(limit int)
}

func (r *recordingSearcher) Search(_ context.Context, _ string, limit int) ([]Result, error) {
	r.onSearch(limit)
	return nil, nil
}

// TestMultiSearchFetchDepthCap covers the upper clamp (Brave's API max).
func TestMultiSearchFetchDepthCap(t *testing.T) {
	var gotLimit int
	rec := &recordingSearcher{onSearch: func(limit int) { gotLimit = limit }}
	m := newTestMulti(time.Second, namedSearcher{name: "brave", searcher: rec})
	if _, _, err := m.Search(context.Background(), "q", 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != fetchCap {
		t.Errorf("per-backend fetch limit = %d, want cap %d", gotLimit, fetchCap)
	}
}
