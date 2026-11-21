package ld

import (
	"reflect"
	"strings"
	"testing"
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
