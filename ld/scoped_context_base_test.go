package ld

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// JSON-LD 1.1 API §5.1.2 and §6.1.2 process a scoped context with the
// "base URL from the term definition", so a relative scoped context in a
// remote context resolves next to that remote context, not the document.
func TestScopedContextResolvesAgainstDefiningContext(t *testing.T) {
	const ctxURL = "https://ctx.org/c/a.jsonld"
	newLoader := func() *urlDocumentLoader {
		return &urlDocumentLoader{docs: map[string]interface{}{
			ctxURL: map[string]interface{}{"@context": map[string]interface{}{
				"@version": 1.1,
				"p":        map[string]interface{}{"@id": "https://ex.org/p", "@context": "s.jsonld"},
				"T":        map[string]interface{}{"@id": "https://ex.org/T", "@context": "s.jsonld"},
				"m": map[string]interface{}{
					"@id": "https://ex.org/m", "@container": "@type", "@type": "@id",
				},
			}},
			"https://ctx.org/c/s.jsonld": map[string]interface{}{"@context": map[string]interface{}{
				"q": "https://ex.org/q",
			}},
		}}
	}
	doc := map[string]interface{}{
		"@context": ctxURL,
		"@id":      "urn:s:1",
		"@type":    "T",
		"q":        "by-type",
		"p":        map[string]interface{}{"q": "by-property"},
		"m":        map[string]interface{}{"T": map[string]interface{}{"q": "by-type-map"}},
	}
	wantQuads := []string{
		`<urn:s:1> <https://ex.org/q> "by-type" .`,
		`<https://ex.org/q> "by-property" .`,
	}

	opts := NewJsonLdOptions("https://doc.org/d/doc.jsonld")
	opts.DocumentLoader = newLoader()
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, q := range wantQuads {
		if !strings.Contains(out.(string), q) {
			t.Fatalf("missing %s in\n%s", q, out)
		}
	}
	if !strings.Contains(out.(string), `<https://ex.org/q> "by-type-map" .`) {
		t.Fatalf("type map value lost its scoped term:\n%s", out)
	}

	copts := NewJsonLdOptions("https://doc.org/d/doc.jsonld")
	copts.DocumentLoader = newLoader()
	expanded := []interface{}{map[string]interface{}{
		"@id":              "urn:s:1",
		"@type":            []interface{}{"https://ex.org/T"},
		"https://ex.org/q": []interface{}{map[string]interface{}{"@value": "by-type"}},
		"https://ex.org/p": []interface{}{map[string]interface{}{
			"https://ex.org/q": []interface{}{map[string]interface{}{"@value": "by-property"}},
		}},
	}}
	compacted, err := NewJsonLdProcessor().Compact(expanded, ctxURL, copts)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	delete(compacted, "@context")
	want := map[string]interface{}{
		"@id":   "urn:s:1",
		"@type": "T",
		"q":     "by-type",
		"p":     map[string]interface{}{"q": "by-property"},
	}
	if !reflect.DeepEqual(compacted, want) {
		t.Fatalf("compact got %v, want %v", compacted, want)
	}
}

// JSON-LD 1.1 API §4.2.2: an identical redefinition of a protected term
// keeps the previous definition, including the base URL of its scoped
// context. The inline scoped context is the same in both contexts, but its
// relative @import only resolves next to the first one.
func TestProtectedRedefinitionKeepsScopedContextBase(t *testing.T) {
	def := func() map[string]interface{} {
		return map[string]interface{}{"@context": map[string]interface{}{
			"@version":   1.1,
			"@protected": true,
			"p": map[string]interface{}{"@id": "https://ex.org/p",
				"@context": map[string]interface{}{"@import": "s.jsonld"}},
		}}
	}
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://ctx.org/a/c.jsonld": def(),
		"https://ctx.org/b/c.jsonld": def(),
		"https://ctx.org/a/s.jsonld": map[string]interface{}{"@context": map[string]interface{}{"q": "https://ex.org/q"}},
		// Loaded only to validate the second definition (§4.2.2 step 21).
		"https://ctx.org/b/s.jsonld": map[string]interface{}{"@context": map[string]interface{}{"q": "https://ex.org/other"}},
	}}
	opts := NewJsonLdOptions("https://doc.org/d/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(map[string]interface{}{
		"@context": []interface{}{"https://ctx.org/a/c.jsonld", "https://ctx.org/b/c.jsonld"},
		"@id":      "urn:s:1",
		"p":        map[string]interface{}{"q": "v"},
	}, opts)
	if err != nil {
		t.Fatalf("loaded %v: %v", loader.got, err)
	}
	if !strings.Contains(out.(string), `<https://ex.org/q> "v" .`) {
		t.Fatalf("scoped term lost:\n%s", out)
	}
}

// JSON-LD 1.1 API §4.2.2: a protected term redefined with another prefix
// flag is not the same definition, so it is a protected term redefinition.
func TestProtectedRedefinitionWithOtherPrefixIsRefused(t *testing.T) {
	ctx := func(prefix bool) map[string]interface{} {
		return map[string]interface{}{
			"@version":   1.1,
			"@protected": true,
			"ex":         map[string]interface{}{"@id": "http://ex.org/", "@prefix": prefix},
		}
	}
	_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
		"@context": []interface{}{ctx(true), ctx(false)},
		"@id":      "urn:s:1",
	}, NewJsonLdOptions(""))
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != ProtectedTermRedefinition {
		t.Fatalf("got %v, want %q", err, ProtectedTermRedefinition)
	}
}

// An unset prefix flag is false: a protected term whose definition only
// sets @type, redefined with the same IRI spelled out in @id, is the same
// definition.
func TestProtectedRedefinitionUnsetPrefixIsFalse(t *testing.T) {
	_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
		"@context": []interface{}{
			map[string]interface{}{
				"@version": 1.1, "@vocab": "http://ex.org/", "@protected": true,
				"p": map[string]interface{}{"@type": "@id"},
			},
			map[string]interface{}{
				"@version": 1.1, "@protected": true,
				"p": map[string]interface{}{"@id": "http://ex.org/p", "@type": "@id"},
			},
		},
		"@id": "urn:s:1",
	}, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
}

// §4.2.2 step 21 validates a scoped context on a copy of remote contexts, so
// the scoped contexts of unused terms do not count toward the limit of
// §4.1.2 step 5.2.3 for the context that defines them.
func TestScopedContextValidationDoesNotCountTowardLimit(t *testing.T) {
	docs := map[string]interface{}{}
	terms := map[string]interface{}{"q": "https://ex.org/q"}
	for i := 0; i < DefaultMaxRemoteContexts+8; i++ {
		u := "https://ctx.org/s" + strings.Repeat("x", i)
		docs[u] = map[string]interface{}{"@context": map[string]interface{}{"r": "https://ex.org/r"}}
		terms["t"+strings.Repeat("x", i)] = map[string]interface{}{"@id": "https://ex.org/t", "@context": u}
	}
	doc := map[string]interface{}{"@context": terms, "@id": "urn:s", "q": "v"}
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = &urlDocumentLoader{docs: docs}
	opts.Format = "application/n-quads"
	got, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	if want := "<urn:s> <https://ex.org/q> \"v\" .\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Each scoped context is validated once per call, whether it is named by
// URL or written inline. Every level defines ten terms that share the next
// level's context, so validating each use separately would parse 10^8
// contexts and not finish.
func TestScopedContextValidatedOncePerCall(t *testing.T) {
	const width, depth = 10, 8
	level := func(next interface{}) map[string]interface{} {
		terms := map[string]interface{}{}
		for i := 0; i < width; i++ {
			def := map[string]interface{}{"@id": fmt.Sprintf("https://ex.org/t%d", i)}
			if next != nil {
				def["@context"] = next
			}
			terms[fmt.Sprintf("t%d", i)] = def
		}
		return terms
	}

	docs := map[string]interface{}{}
	var byURL, inline interface{}
	for l := 0; l < depth; l++ {
		u := fmt.Sprintf("https://ctx.org/%d", l)
		docs[u] = map[string]interface{}{"@context": level(byURL)}
		byURL = u
		inline = level(inline)
	}

	for name, ctx := range map[string]interface{}{"url": byURL, "inline": inline} {
		t.Run(name, func(t *testing.T) {
			doc := map[string]interface{}{"@context": ctx, "@id": "urn:s", "t0": "v"}
			opts := NewJsonLdOptions("")
			opts.DocumentLoader = &urlDocumentLoader{docs: docs}
			done := make(chan error, 1)
			go func() {
				_, err := NewJsonLdProcessor().Expand(doc, opts)
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("Expand: %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("scoped context validation did not finish in 10s")
			}
		})
	}
}
