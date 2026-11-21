package ld

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
)

// Each term's scoped context is processed when the term is defined (JSON-LD
// 1.1 API §4.2.2 step 21), against the active context of that moment. Doing
// that on a full copy of the active context costs the number of terms squared:
// 10,000 terms took about 8 GB of allocation.
func TestScopedContextValidationCostIsLinear(t *testing.T) {
	const terms = 10000
	ctx := make(map[string]interface{}, terms)
	for i := 0; i < terms; i++ {
		ctx[fmt.Sprintf("t%d", i)] = map[string]interface{}{
			"@id":      fmt.Sprintf("https://example.org/t%d", i),
			"@context": map[string]interface{}{fmt.Sprintf("s%d", i): "https://example.org/s"},
		}
	}
	doc := map[string]interface{}{
		"@context":         ctx,
		"https://ex.org/p": "v",
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	if _, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions("")); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > 256<<20 {
		t.Fatalf("expanding a context of %d scoped terms allocated %d MiB", terms, got>>20)
	}
}

// The scoped context of a term no node uses is still processed against the
// active context, so whether it is valid can turn on a term defined there.
func TestScopedContextIsValidatedAgainstTheActiveContext(t *testing.T) {
	scopedTerm := func(scoped map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"@id": "https://example.org/p", "@context": scoped}
	}
	typeT := map[string]interface{}{"q": map[string]interface{}{"@id": "https://example.org/q", "@type": "T"}}
	for name, tc := range map[string]struct {
		context map[string]interface{}
		wantErr bool
	}{
		"type is a term of the active context": {
			context: map[string]interface{}{"T": "https://example.org/T", "p": scopedTerm(typeT)},
		},
		"type is a term the scoped context sets to null": {
			context: map[string]interface{}{"T": "https://example.org/T", "p": scopedTerm(map[string]interface{}{
				"T": nil,
				"q": typeT["q"],
			})},
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			doc := map[string]interface{}{"@context": tc.context, "https://example.org/other": "v"}
			_, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
			if tc.wantErr {
				var ldErr *JsonLdError
				if !errors.As(err, &ldErr) || ldErr.Code != InvalidScopedContext {
					t.Fatalf("err = %v, want %s", err, InvalidScopedContext)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
