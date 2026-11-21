package ld

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type urlDocumentLoader struct {
	docs      map[string]interface{}
	redirects map[string]string // requested URL -> final URL after redirects
	noDocURL  bool              // leave RemoteDocument.DocumentURL empty
	links     map[string]string // final URL -> Link header context target
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
	if l.noDocURL {
		u = ""
	}
	return &RemoteDocument{DocumentURL: u, Document: doc, ContextURL: l.links[u]}, nil
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

// JSON-LD 1.1 API §4.1.2 step 5.2.2 aborts with "context overflow" once
// remote contexts pass a processor limit; a cycle reaches it.
func TestRemoteContextCycleIsContextOverflow(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://example.org/a.jsonld": map[string]interface{}{"@context": "b.jsonld"},
		"https://example.org/b.jsonld": map[string]interface{}{"@context": "a.jsonld"},
	}}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
		"@context": "https://example.org/a.jsonld",
		"@id":      "urn:s:1",
	}, opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != ContextOverflow {
		t.Fatalf("got %v, want %q", err, ContextOverflow)
	}
}

// The same remote context twice in one @context array is not a cycle;
// JSON-LD 1.1 processes it again.
func TestRemoteContextRepeatedInArray(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://example.org/a.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{"y": "https://example.org/y"},
		},
	}}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(map[string]interface{}{
		"@context": []interface{}{"https://example.org/a.jsonld", "https://example.org/a.jsonld"},
		"@id":      "urn:s:1",
		"y":        "2",
	}, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := "<urn:s:1> <https://example.org/y> \"2\" .\n"
	if out.(string) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// Each level lists the next one twice, so a processor that walks every
// path does 2^30 loads. The total number of remote contexts in one
// context processing is limited, so this ends in "context overflow" at once.
func TestRemoteContextFanOutIsBounded(t *testing.T) {
	docs := map[string]interface{}{}
	for i := 0; i < 30; i++ {
		next := fmt.Sprintf("l%d.jsonld", i+1)
		docs[fmt.Sprintf("https://example.org/l%d.jsonld", i)] = map[string]interface{}{"@context": []interface{}{next, next}}
	}
	docs["https://example.org/l30.jsonld"] = map[string]interface{}{"@context": map[string]interface{}{"y": "https://example.org/y"}}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = &urlDocumentLoader{docs: docs}
	_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
		"@context": "https://example.org/l0.jsonld",
		"@id":      "urn:s:1",
	}, opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != ContextOverflow {
		t.Fatalf("got %v, want %q", err, ContextOverflow)
	}
}

// JSON-LD 1.1 API §4.1.2 step 5.2.4: a context already dereferenced MUST
// NOT be dereferenced again.
func TestRemoteContextDereferencedOnce(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://example.org/a.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{"y": "https://example.org/y"},
		},
	}}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
		"@context": []interface{}{"https://example.org/a.jsonld", "https://example.org/a.jsonld"},
		"@id":      "urn:s:1",
	}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(loader.got) != 1 {
		t.Fatalf("loaded %v, want one load", loader.got)
	}
}

// Many sibling contexts in one array stay under the default limit.
func TestRemoteContextManySiblings(t *testing.T) {
	docs := map[string]interface{}{}
	ctx := []interface{}{}
	for i := 0; i < 20; i++ {
		u := fmt.Sprintf("https://example.org/s%d.jsonld", i)
		docs[u] = map[string]interface{}{"@context": map[string]interface{}{fmt.Sprintf("t%d", i): "https://example.org/t"}}
		ctx = append(ctx, u)
	}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = &urlDocumentLoader{docs: docs}
	if _, err := NewJsonLdProcessor().Expand(map[string]interface{}{"@context": ctx, "@id": "urn:s:1"}, opts); err != nil {
		t.Fatal(err)
	}
}

// JSON-LD 1.0 reports a context cycle as "recursive context inclusion".
func TestRemoteContextCycleInJSONLD10(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://example.org/a.jsonld": map[string]interface{}{"@context": "a.jsonld"},
	}}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	opts.ProcessingMode = JsonLd_1_0
	_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
		"@context": "https://example.org/a.jsonld",
		"@id":      "urn:s:1",
	}, opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != RecursiveContextInclusion {
		t.Fatalf("got %v, want %q", err, RecursiveContextInclusion)
	}
}

// JSON-LD 1.1 API §5.1.2 processes an embedded @context with the base URL
// of the expansion, so a relative context URL in a nested node does not
// resolve against an @base set by an outer context.
func TestEmbeddedContextIgnoresOuterBase(t *testing.T) {
	loader := &urlDocumentLoader{docs: map[string]interface{}{
		"https://doc.org/d/ctx.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{"y": "https://example.org/y"},
		},
	}}
	opts := NewJsonLdOptions("https://doc.org/d/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(map[string]interface{}{
		"@context": map[string]interface{}{"@base": "http://other.org/x/", "p": "https://example.org/p"},
		"@id":      "urn:s:1",
		"p":        map[string]interface{}{"@context": "ctx.jsonld", "@id": "urn:s:2", "y": "2"},
	}, opts)
	if err != nil {
		t.Fatalf("loaded %v: %v", loader.got, err)
	}
	if want := "<urn:s:2> <https://example.org/y> \"2\" .\n"; !strings.Contains(out.(string), want) {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// A loader that leaves DocumentURL empty still gets @import resolved
// next to the requested context URL.
func TestRemoteContextImportWithoutDocumentURL(t *testing.T) {
	loader := &urlDocumentLoader{noDocURL: true, docs: map[string]interface{}{
		"https://example.org/ctx/a.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{"@version": 1.1, "@import": "b.jsonld"},
		},
		"https://example.org/ctx/b.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{"y": "https://example.org/y"},
		},
	}}
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(map[string]interface{}{
		"@context": "https://example.org/ctx/a.jsonld",
		"@id":      "urn:s:1",
		"y":        "2",
	}, opts)
	if err != nil {
		t.Fatalf("loaded %v: %v", loader.got, err)
	}
	if want := "<urn:s:1> <https://example.org/y> \"2\" .\n"; out.(string) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// A repeat of a cached context is processed again, so it counts toward
// the limit although the loader sees one load. Without that, one fetched
// context could be processed far more often than a loader budget allows.
func TestRemoteContextRepeatsCountTowardLimit(t *testing.T) {
	const u = "https://example.org/a.jsonld"
	newDoc := func(n int) map[string]interface{} {
		ctx := make([]interface{}, n)
		for i := range ctx {
			ctx[i] = u
		}
		return map[string]interface{}{"@context": ctx, "@id": "urn:s:1"}
	}
	newLoader := func() *urlDocumentLoader {
		return &urlDocumentLoader{docs: map[string]interface{}{
			u: map[string]interface{}{"@context": map[string]interface{}{"y": "https://example.org/y"}},
		}}
	}

	loader := newLoader()
	opts := NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = loader
	if _, err := NewJsonLdProcessor().Expand(newDoc(32), opts); err != nil {
		t.Fatalf("32 repeats under the default limit: %v", err)
	}
	_, err := NewJsonLdProcessor().Expand(newDoc(33), opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != ContextOverflow {
		t.Fatalf("got %v, want %q", err, ContextOverflow)
	}
	if len(loader.got) != 2 {
		t.Fatalf("loaded %d times over two documents, want 2", len(loader.got))
	}

	opts = NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = newLoader()
	opts.MaxRemoteContexts = DefaultMaxRemoteContexts + 1
	if _, err := NewJsonLdProcessor().Expand(newDoc(DefaultMaxRemoteContexts+1), opts); err != nil {
		t.Fatalf("MaxRemoteContexts raised: %v", err)
	}

	opts = NewJsonLdOptions("https://example.org/doc.jsonld")
	opts.DocumentLoader = newLoader()
	opts.MaxRemoteContexts = 2
	_, err = NewJsonLdProcessor().Expand(newDoc(3), opts)
	if !errors.As(err, &ldErr) || ldErr.Code != ContextOverflow {
		t.Fatalf("MaxRemoteContexts lowered: got %v, want %q", err, ContextOverflow)
	}
}

// scopedImportDoc uses a term whose scoped context imports a remote context,
// n times.
func scopedImportDoc(n int) map[string]interface{} {
	items := make([]interface{}, n)
	for i := range items {
		items[i] = map[string]interface{}{"name": fmt.Sprintf("n%d", i)}
	}
	return map[string]interface{}{
		"@context": map[string]interface{}{
			"@version": 1.1,
			"item": map[string]interface{}{
				"@id":      "https://example.org/item",
				"@context": map[string]interface{}{"@import": "https://example.org/imp.jsonld"},
			},
		},
		"item": items,
	}
}

func importLoader() *urlDocumentLoader {
	return &urlDocumentLoader{docs: map[string]interface{}{
		"https://example.org/imp.jsonld": map[string]interface{}{
			"@context": map[string]interface{}{"name": "https://example.org/name"},
		},
	}}
}

// The loader may hand out the same object on every call, as a caching loader
// does. Merging @import must not write into it (JSON-LD 1.1 API §4.1.2 step
// 5.6.7 merges into a new map), or the second use finds an @import entry in
// the imported context and fails.
func TestImportLeavesLoadedDocumentUnchanged(t *testing.T) {
	loader := importLoader()
	for i := 0; i < 2; i++ {
		opts := NewJsonLdOptions("")
		opts.DocumentLoader = loader
		out, err := NewJsonLdProcessor().Expand(scopedImportDoc(1), opts)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if !strings.Contains(fmt.Sprint(out), "https://example.org/name") {
			t.Fatalf("run %d: imported term not applied: %v", i, out)
		}
	}
	imported := loader.docs["https://example.org/imp.jsonld"].(map[string]interface{})["@context"].(map[string]interface{})
	if len(imported) != 1 {
		t.Fatalf("loaded document changed: %v", imported)
	}
}

// A context URL dereferenced once in an operation is not dereferenced again
// (JSON-LD 1.1 API §4.1.2 step 5.2.4), however many times a scoped context
// that names it is applied.
func TestScopedImportDereferencedOnce(t *testing.T) {
	loader := importLoader()
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = loader
	if _, err := NewJsonLdProcessor().Expand(scopedImportDoc(50), opts); err != nil {
		t.Fatal(err)
	}
	if len(loader.got) != 1 {
		t.Fatalf("loaded %d times, want once", len(loader.got))
	}
}

func TestScopedRemoteContextDereferencedOnce(t *testing.T) {
	loader := importLoader()
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = loader
	doc := scopedImportDoc(50)
	doc["@context"].(map[string]interface{})["item"].(map[string]interface{})["@context"] = "https://example.org/imp.jsonld"
	if _, err := NewJsonLdProcessor().Expand(doc, opts); err != nil {
		t.Fatal(err)
	}
	if len(loader.got) != 1 {
		t.Fatalf("loaded %d times, want once", len(loader.got))
	}
}

type sharedDocLoader struct{ rd *RemoteDocument }

func (l sharedDocLoader) LoadDocument(string) (*RemoteDocument, error) { return l.rd, nil }

// Calls running at once may share one loaded document, as they do behind a
// caching loader; context processing only reads it. Run with -race.
func TestConcurrentCallsShareLoadedDocument(t *testing.T) {
	loader := sharedDocLoader{rd: &RemoteDocument{
		DocumentURL: "https://example.org/imp.jsonld",
		Document:    importLoader().docs["https://example.org/imp.jsonld"],
	}}
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			opts := NewJsonLdOptions("")
			opts.DocumentLoader = loader
			_, err := NewJsonLdProcessor().Expand(scopedImportDoc(5), opts)
			errs <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}

// Two protected definitions of a term whose scoped context is the same
// relative IRI differ when the IRI resolves to different contexts, so the
// second is a protected term redefinition (JSON-LD 1.1 API §4.2.2: a term
// definition carries its base URL). The same inline context, or relative
// IRIs that land on one URL, are the same definition.
func TestProtectedRedefinitionComparesResolvedScopedContext(t *testing.T) {
	ctx := func(scoped string) map[string]interface{} {
		return map[string]interface{}{"@context": map[string]interface{}{
			"@version": 1.1, "@protected": true,
			"t": map[string]interface{}{"@id": "https://example.org/t", "@context": scoped},
		}}
	}
	// JSON-LD 1.1 API §4.2.2 step 21 loads a scoped context when the term
	// is defined, so each one the contexts name must exist.
	scopedDoc := map[string]interface{}{"@context": map[string]interface{}{"v": "https://example.org/v"}}
	inline := func() map[string]interface{} {
		return map[string]interface{}{"@context": map[string]interface{}{
			"@version": 1.1, "@protected": true,
			"t": map[string]interface{}{"@id": "https://example.org/t",
				"@context": map[string]interface{}{"v": "https://example.org/v"}},
		}}
	}
	cases := []struct {
		name    string
		c1, c2  string
		docs    map[string]interface{}
		wantErr bool
	}{
		{"relative to two targets", "https://example.org/x/c1", "https://example.org/y/c2", map[string]interface{}{
			"https://example.org/x/c1": ctx("s"), "https://example.org/y/c2": ctx("s"),
			"https://example.org/x/s": scopedDoc, "https://example.org/y/s": scopedDoc,
		}, true},
		{"relative to one target", "https://example.org/x/c1", "https://example.org/x/c2", map[string]interface{}{
			"https://example.org/x/c1": ctx("s"), "https://example.org/x/c2": ctx("s"),
			"https://example.org/x/s": scopedDoc,
		}, false},
		{"same inline", "https://example.org/x/c1", "https://example.org/y/c2", map[string]interface{}{
			"https://example.org/x/c1": inline(), "https://example.org/y/c2": inline(),
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := NewJsonLdOptions("")
			opts.DocumentLoader = &urlDocumentLoader{docs: c.docs}
			_, err := NewJsonLdProcessor().Expand(map[string]interface{}{
				"@context": []interface{}{c.c1, c.c2},
				"@id":      "urn:s",
			}, opts)
			var ldErr *JsonLdError
			gotErr := errors.As(err, &ldErr) && ldErr.Code == ProtectedTermRedefinition
			if gotErr != c.wantErr || (err != nil && !gotErr) {
				t.Fatalf("got %v, want protected term redefinition: %v", err, c.wantErr)
			}
		})
	}
}

type countingLoader struct {
	mu    sync.Mutex
	loads int
}

func (l *countingLoader) LoadDocument(u string) (*RemoteDocument, error) {
	l.mu.Lock()
	l.loads++
	l.mu.Unlock()
	return &RemoteDocument{DocumentURL: u, Document: map[string]interface{}{
		"@context": map[string]interface{}{"name": "https://example.org/name"},
	}}, nil
}

// A Context built by the caller and passed to JsonLdApi keeps no state
// between calls: goroutines may share it, and each call loads the contexts
// it needs. Run with -race.
func TestCallerContextKeepsNoDocuments(t *testing.T) {
	loader := &countingLoader{}
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = loader
	ctx, err := NewContext(nil, opts).Parse(map[string]interface{}{"@vocab": "https://example.org/"})
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]interface{}{"@context": "https://example.org/c.jsonld", "name": "x"}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := NewJsonLdApi().Expand(ctx, "", CloneDocument(doc), opts, false, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if loader.loads != 8 {
		t.Fatalf("loaded %d times for 8 calls, want 8", loader.loads)
	}
}
