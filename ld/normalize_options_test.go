package ld

import (
	"errors"
	"testing"
)

// Normalize runs ToRDF on the caller's options, so every expansion option the
// caller sets applies to it as it does to ToRDF.
func TestNormalizeKeepsExpansionOptions(t *testing.T) {
	type m = map[string]interface{}
	newOpts := func() *JsonLdOptions {
		o := NewJsonLdOptions("")
		o.Format = "application/n-quads"
		o.Algorithm = AlgorithmURDNA2015
		return o
	}

	t.Run("SafeMode", func(t *testing.T) {
		o := newOpts()
		o.SafeMode = true
		doc := m{"@id": "urn:s", "undefinedTerm": "x"}
		if _, err := NewJsonLdProcessor().Normalize(doc, o); err == nil {
			t.Fatal("undefined term was dropped silently")
		}
	})

	t.Run("ExpandContext", func(t *testing.T) {
		o := newOpts()
		o.ExpandContext = m{"p": "http://ex.org/p"}
		out, err := NewJsonLdProcessor().Normalize(m{"@id": "urn:s", "p": "x"}, o)
		if err != nil {
			t.Fatal(err)
		}
		if want := "<urn:s> <http://ex.org/p> \"x\" .\n"; out != want {
			t.Fatalf("got %q, want %q", out, want)
		}
	})

	// Canonical N-Quads cannot hold a blank node predicate, so the caller's
	// ProduceGeneralizedRdf does not reach the canonicalized dataset.
	t.Run("ProduceGeneralizedRdf", func(t *testing.T) {
		o := newOpts()
		o.ProduceGeneralizedRdf = true
		doc := m{"@context": m{"p": "_:bp"}, "@id": "urn:s", "p": "x"}
		out, err := NewJsonLdProcessor().Normalize(doc, o)
		if err != nil {
			t.Fatal(err)
		}
		if out != "" {
			t.Fatalf("got %q, want no quads", out)
		}
	})

	t.Run("MaxRemoteContexts", func(t *testing.T) {
		o := newOpts()
		o.DocumentLoader = &urlDocumentLoader{docs: map[string]interface{}{
			"https://c.org/1": m{"@context": "https://c.org/2"},
			"https://c.org/2": m{"@context": m{"p": "http://ex.org/p"}},
		}}
		o.MaxRemoteContexts = 1
		doc := m{"@context": "https://c.org/1", "@id": "urn:s", "p": "x"}
		_, err := NewJsonLdProcessor().Normalize(doc, o)
		var ldErr *JsonLdError
		if !errors.As(err, &ldErr) || ldErr.Code != ContextOverflow {
			t.Fatalf("got %v, want %s", err, ContextOverflow)
		}
	})
}
