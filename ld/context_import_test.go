package ld

import (
	"strings"
	"testing"
)

type urlDocumentLoader struct {
	docs map[string]interface{}
	got  []string
}

func (l *urlDocumentLoader) LoadDocument(u string) (*RemoteDocument, error) {
	l.got = append(l.got, u)
	doc, ok := l.docs[u]
	if !ok {
		return nil, NewJsonLdError(LoadingDocumentFailed, u)
	}
	return &RemoteDocument{DocumentURL: u, Document: doc}, nil
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
