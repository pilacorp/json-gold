package ld

import (
	"strings"
	"testing"
)

type urlDocumentLoader struct {
	docs      map[string]interface{}
	redirects map[string]string // requested URL -> final URL after redirects
	got       []string
}

func (l *urlDocumentLoader) LoadDocument(u string) (*RemoteDocument, error) {
	l.got = append(l.got, u)
	if to, ok := l.redirects[u]; ok {
		u = to
	}
	doc, ok := l.docs[u]
	if !ok {
		return nil, NewJsonLdError(LoadingDocumentFailed, u)
	}
	return &RemoteDocument{DocumentURL: u, Document: doc}, nil
}

// A relative @vocab in a remote context must resolve against the document
// base, not against the context URL. JSON-LD 1.1 API §4.1.2 step 5.6.3
// resolves @import against the context URL, but @vocab resolves against the
// base IRI from the active context (the document).
func TestRemoteContextRelativeVocabUsesDocumentBase(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://ctx.org/c/a.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{
				"@version": 1.1,
				"@vocab":   "v/",
			},
		},
	}}
	doc := map[string]interface{}{
		"@context": "https://ctx.org/c/a.jsonld",
		"@id":      "urn:s:1",
		"name":     "Alice",
	}
	opts := NewJsonLdOptions("https://doc.org/d/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	want := "https://doc.org/d/v/name"
	bad := "https://ctx.org/c/v/name"
	if strings.Contains(quads, bad) {
		t.Fatalf("@vocab resolved against context URL:\n%s", quads)
	}
	if !strings.Contains(quads, want) {
		t.Fatalf("@vocab did not resolve against document base:\n%s\nwant predicate %s", quads, want)
	}
}

func TestRemoteContextRelativeImportUsesContextURL(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://example.org/ctx/a.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{
				"@version": 1.1,
				"@import":  "b.jsonld",
				"x":        "https://example.org/x",
			},
		},
		"https://example.org/ctx/b.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{
				"y": "https://example.org/y",
			},
		},
	}}
	doc := map[string]interface{}{
		"@context": "https://example.org/ctx/a.jsonld",
		"@id":      "urn:s:1",
		"y":        "2",
	}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatalf("expand: %v\nrequested %v", err, loader.got)
	}
	var sawImport bool
	for _, u := range loader.got {
		if u == "https://example.org/ctx/b.jsonld" {
			sawImport = true
		}
		if u == "b.jsonld" || strings.HasPrefix(u, "https://example.org/b.jsonld") || u == "https://example.org/b.jsonld" {
			t.Errorf("import resolved against the document base: %s", u)
		}
	}
	if !sawImport {
		t.Fatalf("import was not loaded from the context URL, requested %v", loader.got)
	}
	quads := out.(string)
	if !strings.Contains(quads, "<urn:s:1> <https://example.org/y> \"2\"") {
		t.Fatalf("imported term missing:\n%s", quads)
	}
}

// JSON-LD 1.1 API §4.1.2 step 5.2.6 processes a remote context with "the
// documentUrl of context document" as base URL, so after a redirect a
// relative @import resolves next to the final URL.
func TestRemoteContextImportAfterRedirectUsesDocumentURL(t *testing.T) {
	loader := &urlDocumentLoader{
		redirects: map[string]string{"https://old.org/a.jsonld": "https://new.org/ctx/a.jsonld"},
		docs: map[string]interface{}{
			"https://new.org/ctx/a.jsonld": map[string]interface{}{
				"@context": map[string]interface{}{"@version": 1.1, "@import": "b.jsonld"},
			},
			"https://new.org/ctx/b.jsonld": map[string]interface{}{
				"@context": map[string]interface{}{"y": "https://example.org/y"},
			},
		},
	}
	doc := map[string]interface{}{
		"@context": "https://old.org/a.jsonld",
		"@id":      "urn:s:1",
		"y":        "2",
	}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatalf("loaded %v: %v", loader.got, err)
	}
	want := "<urn:s:1> <https://example.org/y> \"2\" .\n"
	if out.(string) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
