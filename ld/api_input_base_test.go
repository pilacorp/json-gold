package ld

import (
	"reflect"
	"testing"
)

// A remote input redirects from a.org/old to b.org/new. Each folder holds a
// ctx.jsonld that maps p differently, so the IRI p expands to shows which
// URL a relative context was resolved against.
func redirectedInputLoader() *urlDocumentLoader {
	type m = map[string]interface{}
	return &urlDocumentLoader{
		redirects: map[string]string{
			"https://a.org/old/doc.jsonld":   "https://b.org/new/doc.jsonld",
			"https://a.org/old/abs.jsonld":   "https://b.org/new/abs.jsonld",
			"https://a.org/old/plain.jsonld": "https://b.org/new/plain.jsonld",
		},
		docs: map[string]interface{}{
			"https://b.org/new/doc.jsonld":   m{"@context": "ctx.jsonld", "@id": "rel", "p": "x"},
			"https://b.org/new/plain.jsonld": m{"@id": "rel", "p": "x"},
			"https://b.org/new/abs.jsonld":   m{"@context": "https://b.org/new/ctx.jsonld", "@id": "rel", "p": "x"},
			"https://b.org/new/ctx.jsonld":   m{"@context": m{"p": "http://ex.org/p"}},
			"https://a.org/old/ctx.jsonld":   m{"@context": m{"p": "http://old.org/p"}},
			"https://base.org/ctx.jsonld":    m{"@context": m{"p": "http://base.org/p"}},
		},
	}
}

const redirectedInput = "https://a.org/old/doc.jsonld"

// redirectedAbsInput names its own context by absolute IRI, so only the
// compaction context can pick a wrong ctx.jsonld.
const redirectedAbsInput = "https://a.org/old/abs.jsonld"

// JSON-LD 1.1 API §9.2 expand() steps 5, 6 and 8: the base option overrides
// only the base IRI. The document's own contexts and expandContext resolve
// against the documentUrl.
func TestExpandRemoteInputBase(t *testing.T) {
	type m = map[string]interface{}
	type a = []interface{}
	cases := []struct {
		name, input, base string
		expandContext     interface{}
		link              string
		want              a
	}{
		{"no base option", redirectedInput, "", nil, "",
			a{m{"@id": "https://b.org/new/rel", "http://ex.org/p": a{m{"@value": "x"}}}}},
		{"base option", redirectedInput, "https://base.org/doc", nil, "",
			a{m{"@id": "https://base.org/rel", "http://ex.org/p": a{m{"@value": "x"}}}}},
		{"expandContext with base option", "https://b.org/new/plain.jsonld", "https://base.org/doc", "ctx.jsonld", "",
			a{m{"@id": "https://base.org/rel", "http://ex.org/p": a{m{"@value": "x"}}}}},
		// A relative Link header target resolves against the documentUrl
		// after redirects (RFC 8288 §3.1, RFC 3986 §5.1.3), not against the
		// requested URL or the base option.
		{"Link context with base option", "https://a.org/old/plain.jsonld", "https://base.org/doc", nil, "ctx.jsonld",
			a{m{"@id": "https://base.org/rel", "http://ex.org/p": a{m{"@value": "x"}}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewJsonLdOptions(tc.base)
			loader := redirectedInputLoader()
			loader.links = map[string]string{"https://b.org/new/plain.jsonld": tc.link}
			o.DocumentLoader = loader
			o.ExpandContext = tc.expandContext
			got, err := NewJsonLdProcessor().Expand(tc.input, o)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v\nwant %v", got, tc.want)
			}
		})
	}
}

// §9.2 compact() steps 3, 5, 7 and 8: the input is dereferenced once, its
// context resolves against the documentUrl after redirects, and so does a
// relative compaction context. The base IRI for compaction is the base
// option, else the document's IRI.
func TestCompactRemoteInputBase(t *testing.T) {
	type m = map[string]interface{}
	cases := []struct {
		name, input, base string
		context           interface{}
		want              m
	}{
		{"no context", redirectedInput, "", m{}, m{"@id": "rel", "http://ex.org/p": "x"}},
		{"relative context", redirectedAbsInput, "", "ctx.jsonld",
			m{"@context": "ctx.jsonld", "@id": "rel", "p": "x"}},
		{"relative context, base option", redirectedAbsInput, "https://base.org/doc", "ctx.jsonld",
			m{"@context": "ctx.jsonld", "@id": "rel", "p": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := NewJsonLdOptions(tc.base)
			o.DocumentLoader = redirectedInputLoader()
			got, err := NewJsonLdProcessor().Compact(tc.input, tc.context, o)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v\nwant %v", got, tc.want)
			}
		})
	}
}

func TestFlattenRemoteInputBase(t *testing.T) {
	type m = map[string]interface{}
	type a = []interface{}
	o := NewJsonLdOptions("")
	o.DocumentLoader = redirectedInputLoader()
	got, err := NewJsonLdProcessor().Flatten(redirectedInput, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	want := a{m{"@id": "https://b.org/new/rel", "http://ex.org/p": a{m{"@value": "x"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestFrameRemoteInputBase(t *testing.T) {
	type m = map[string]interface{}
	o := NewJsonLdOptions("")
	o.DocumentLoader = redirectedInputLoader()
	frame := m{"@context": m{"p": "http://ex.org/p"}}
	got, err := NewJsonLdProcessor().Frame(redirectedInput, frame, o)
	if err != nil {
		t.Fatal(err)
	}
	want := m{"@context": m{"p": "http://ex.org/p"}, "@id": "rel", "p": "x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// §9.2 flatten() step 6 compacts the flattened output, which is not a remote
// document, so compact() step 5 resolves the context against the base option.
func TestFlattenContextUsesBaseOption(t *testing.T) {
	type m = map[string]interface{}
	type a = []interface{}
	o := NewJsonLdOptions("https://base.org/doc")
	o.DocumentLoader = redirectedInputLoader()
	got, err := NewJsonLdProcessor().Flatten(redirectedAbsInput, "ctx.jsonld", o)
	if err != nil {
		t.Fatal(err)
	}
	// base.org/ctx.jsonld maps p to another IRI, so http://ex.org/p stays.
	graph := got.(m)["@graph"]
	want := a{m{"@id": "rel", "http://ex.org/p": "x"}}
	if !reflect.DeepEqual(graph, want) {
		t.Fatalf("got %v\nwant %v", graph, want)
	}
}

// §9.2 expand() step 4: a remote input with no document is a loading
// document failed error, not an empty result.
func TestRemoteInputWithoutDocument(t *testing.T) {
	o := NewJsonLdOptions("")
	o.DocumentLoader = &urlDocumentLoader{docs: map[string]interface{}{"https://x.org/doc": nil}}
	_, err := NewJsonLdProcessor().Expand("https://x.org/doc", o)
	if e, ok := err.(*JsonLdError); !ok || e.Code != LoadingDocumentFailed {
		t.Fatalf("got %v, want %s", err, LoadingDocumentFailed)
	}
}
