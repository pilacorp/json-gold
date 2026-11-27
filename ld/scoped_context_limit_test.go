package ld

import (
	"errors"
	"testing"
)

// scopedLimitDoc has its own @context, uses the type-scoped type T on one
// node and the property-scoped term p on n objects: n+3 applications in
// expansion (the context, T, p's key and each object).
func scopedLimitDoc(n int) map[string]interface{} {
	items := make([]interface{}, n)
	for i := range items {
		items[i] = map[string]interface{}{"q": "v"}
	}
	return map[string]interface{}{
		"@context": map[string]interface{}{
			"@vocab": "https://ex.org/",
			"p":      map[string]interface{}{"@context": map[string]interface{}{"q": "https://ex.org/q2"}},
			"T":      map[string]interface{}{"@context": map[string]interface{}{"r": "https://ex.org/r"}},
		},
		"@type": "T",
		"p":     items,
	}
}

// Each object under a property-scoped term applies its context again, so the
// limit counts objects, not keys; a node's own @context counts too.
func TestMaxScopedContexts(t *testing.T) {
	proc := NewJsonLdProcessor()
	for _, tc := range []struct {
		limit   int
		wantErr bool
	}{
		{0, false}, {7, false}, {6, true},
	} {
		opts := NewJsonLdOptions("")
		opts.MaxScopedContexts = tc.limit
		_, err := proc.Expand(scopedLimitDoc(4), opts)
		var ldErr *JsonLdError
		if got := errors.As(err, &ldErr) && ldErr.Code == ScopedContextOverflow; got != tc.wantErr {
			t.Errorf("limit %d: err = %v, want overflow %v", tc.limit, err, tc.wantErr)
		}
	}
}

// A @context on each node is processed again for each node.
func TestMaxScopedContexts_NodeContexts(t *testing.T) {
	items := make([]interface{}, 5)
	for i := range items {
		items[i] = map[string]interface{}{"@context": map[string]interface{}{"q": "https://ex.org/q"}, "q": "v"}
	}
	doc := map[string]interface{}{"@context": map[string]interface{}{"@vocab": "https://ex.org/"}, "p": items}
	opts := NewJsonLdOptions("")
	opts.MaxScopedContexts = 5
	_, err := NewJsonLdProcessor().Expand(doc, opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != ScopedContextOverflow {
		t.Fatalf("err = %v, want ScopedContextOverflow", err)
	}
}

// Compact expands and then compacts; both phases count toward one limit.
func TestMaxScopedContexts_WholeCall(t *testing.T) {
	doc := scopedLimitDoc(4) // 7 applications in expansion, more in compaction
	ctx := doc["@context"]
	opts := NewJsonLdOptions("")
	opts.MaxScopedContexts = 9
	if _, err := NewJsonLdProcessor().Expand(doc, opts); err != nil {
		t.Fatalf("expand at the limit: %v", err)
	}
	_, err := NewJsonLdProcessor().Compact(doc, ctx, opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != ScopedContextOverflow {
		t.Fatalf("compact: err = %v, want ScopedContextOverflow", err)
	}
}
