package ld

import (
	"encoding/json"
	"errors"
	"testing"
)

// One remote scoped context is checked under two active contexts: valid under
// the first, invalid under the second (JSON-LD 1.1 API §4.2.2 step 21.3). The
// second definition is refused whatever the first check found.
func TestValidateScopedContext_PerActiveContext(t *testing.T) {
	for name, tc := range map[string]struct{ scoped, doc string }{
		// A term without @id needs a vocabulary mapping (step 18).
		"vocab": {
			`{"@context": {"t": {"@type": "@id"}}}`,
			`{"@context": [{"@vocab": "https://ex.org/", "a": {"@context": "https://ctx.test/s"}},
				{"@vocab": null, "b": {"@id": "https://ex.org/b", "@context": "https://ctx.test/s"}}], "https://ex.org/x": "y"}`,
		},
		// A compact IRI term must expand to its own @id (step 14.2.4.2).
		"prefix": {
			`{"@context": {"t": {"@id": "http://x.example/t", "@context": {"ex:foo": {"@id": "http://a.example/foo"}}}}}`,
			`{"@context": [{"ex": "http://a.example/"}, "https://ctx.test/s"], "@id": "http://s.example/1",
				"http://p.example/q": {"@context": [{"ex": "http://b.example/"}, "https://ctx.test/s"], "@id": "http://s.example/2"}}`,
		},
		// The whole value is looked up before its prefix (§5.2.2 steps 4-6):
		// "foo:" may be a keyword alias, so @type "foo:" is no IRI.
		"compact-alias": {
			`{"@context": {"t": {"@id": "http://x.example/t", "@context": {"x": {"@id": "http://x.example/x", "@type": "foo:"}}}}}`,
			`{"@context": ["https://ctx.test/s"], "@id": "http://s.example/1",
				"http://p.example/q": {"@context": [{"foo:": "@type"}, "https://ctx.test/s"], "@id": "http://s.example/2"}}`,
		},
		// A scoped context nested in one is checked with it (step 21.3 again).
		"nested": {
			`{"@context": {"t": {"@id": "http://x.example/t", "@context": {"t2": {"@id": "http://x.example/t2", "@context": {"ex:foo": {"@id": "http://a.example/foo"}}}}}}}`,
			`{"@context": [{"ex": "http://a.example/"}, "https://ctx.test/s"], "@id": "http://s.example/1",
				"http://p.example/q": {"@context": [{"ex": "http://b.example/"}, "https://ctx.test/s"], "@id": "http://s.example/2"}}`,
		},
		// A bare value may be a keyword alias of the active context (§5.2.2 step 4).
		"alias": {
			`{"@context": {"t": {"@id": "http://x.example/t", "@context": {"bar": "foo"}}}}`,
			`{"@context": [{"foo": "@type"}, "https://ctx.test/s"], "@id": "http://s.example/1",
				"http://p.example/q": {"@context": [{"foo": null}, "https://ctx.test/s"], "@id": "http://s.example/2"}}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var scoped, doc interface{}
			if err := json.Unmarshal([]byte(tc.scoped), &scoped); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.doc), &doc); err != nil {
				t.Fatal(err)
			}
			loader := NewCachingDocumentLoader(NewDefaultDocumentLoader(nil))
			loader.AddDocument("https://ctx.test/s", scoped)
			opts := NewJsonLdOptions("")
			opts.DocumentLoader = loader
			_, err := NewJsonLdProcessor().Expand(doc, opts)
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != InvalidScopedContext {
				t.Fatalf("err = %v, want %s", err, InvalidScopedContext)
			}
		})
	}
}

// While a scoped context is checked, a remote context already on the way
// there is not processed again (§4.1.2 step 5.2.2): U names V, V redefines y
// and names U, and the document never uses t, so it is valid. Using t
// processes U under the new y, which is refused.
func TestValidateScopedContext_RemoteCycle(t *testing.T) {
	loader := NewCachingDocumentLoader(NewDefaultDocumentLoader(nil))
	for u, ctx := range map[string]string{
		"https://ctx.test/u": `{"@context": {"t": {"@id": "http://x.example/t", "@context": "https://ctx.test/v"}, "y:foo": {"@id": "http://a.example/foo"}}}`,
		"https://ctx.test/v": `{"@context": {"y": "http://b.example/", "t2": {"@id": "http://x.example/t2", "@context": "https://ctx.test/u"}}}`,
	} {
		var doc interface{}
		if err := json.Unmarshal([]byte(ctx), &doc); err != nil {
			t.Fatal(err)
		}
		loader.AddDocument(u, doc)
	}
	expand := func(body string) error {
		var doc interface{}
		if err := json.Unmarshal([]byte(`{"@context": [{"y": "http://a.example/"}, "https://ctx.test/u"], "@id": "http://s.example/1", `+body+`}`), &doc); err != nil {
			t.Fatal(err)
		}
		opts := NewJsonLdOptions("")
		opts.DocumentLoader = loader
		_, err := NewJsonLdProcessor().Expand(doc, opts)
		return err
	}
	if err := expand(`"http://p.example/q": "v"`); err != nil {
		t.Fatalf("t unused: %v", err)
	}
	if err := expand(`"t": {"@id": "http://s.example/2"}`); err == nil {
		t.Fatal("t used: accepted, want an invalid IRI mapping")
	}
}

// expandWith expands doc with the remote contexts given, by URL.
func expandWith(t *testing.T, contexts map[string]string, doc string, maxRemote int) error {
	t.Helper()
	loader := NewCachingDocumentLoader(NewDefaultDocumentLoader(nil))
	for u, ctx := range contexts {
		var d interface{}
		if err := json.Unmarshal([]byte(ctx), &d); err != nil {
			t.Fatal(err)
		}
		loader.AddDocument(u, d)
	}
	var d interface{}
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		t.Fatal(err)
	}
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = loader
	opts.MaxRemoteContexts = maxRemote
	_, err := NewJsonLdProcessor().Expand(d, opts)
	return err
}

// A check reused, or a remote context skipped, must give what checking anew
// gives: each document below is refused by the REC.
func TestValidateScopedContext_ReuseMatchesRecheck(t *testing.T) {
	const (
		u1 = `{"@context": {"@vocab": "https://ex.org/v/"}}`
		x  = `{"@context": {"ex:foo": {"@id": "http://a.example/foo"}}}`
	)
	for name, tc := range map[string]struct {
		contexts  map[string]string
		doc       string
		maxRemote int
	}{
		// u1 is in the list the second time t is defined, so step 5.2.2
		// skips it and u2's t2 has no vocabulary mapping.
		"skipped-later": {map[string]string{
			"https://ctx.test/u1": u1,
			"https://ctx.test/u2": `{"@context": ["https://ctx.test/u1", {"t2": {}}]}`,
			"https://ctx.test/u3": `{"@context": [{"t": {"@id": "https://ex.org/t", "@context": "https://ctx.test/u2"}}, "https://ctx.test/u1",
				{"@vocab": null, "t": {"@id": "https://ex.org/t", "@context": "https://ctx.test/u2"}}]}`,
		}, `{"@context": "https://ctx.test/u3", "@id": "https://ex.org/n"}`, 0},
		// The same, one check further down.
		"skipped-in-nested-check": {map[string]string{
			"https://ctx.test/u1": u1,
			"https://ctx.test/u5": `{"@context": ["https://ctx.test/u1", {"t2": {}}]}`,
			"https://ctx.test/u2": `{"@context": {"s": {"@id": "https://ex.org/s", "@context": "https://ctx.test/u5"}}}`,
			"https://ctx.test/u3": `{"@context": [{"t": {"@id": "https://ex.org/t", "@context": "https://ctx.test/u2"}}, "https://ctx.test/u1",
				{"@vocab": null, "t": {"@id": "https://ex.org/t", "@context": "https://ctx.test/u2"}}]}`,
		}, `{"@context": "https://ctx.test/u3", "@id": "https://ex.org/n"}`, 0},
		// x is processed under a sibling first; it is not on the way to the
		// second sibling (step 5.2.6 passes a copy), so it is processed there.
		"sibling": {map[string]string{
			"https://ctx.test/x": x,
			"https://ctx.test/a": `{"@context": [{"ex": "http://a.example/"}, "https://ctx.test/x"]}`,
			"https://ctx.test/b": `{"@context": [{"ex": "http://b.example/"}, "https://ctx.test/x"]}`,
		}, `{"@context": {"t": {"@id": "http://x.example/t", "@context": ["https://ctx.test/a", "https://ctx.test/b"]}},
			"@id": "http://s.example/1", "http://p.example/q": "v"}`, 0},
		// The second check of ua would pass the remote context limit.
		"limit": {map[string]string{
			"https://ctx.test/ua": `{"@context": "https://ctx.test/ub"}`,
			"https://ctx.test/ub": `{"@context": {}}`,
			"https://ctx.test/x1": `{"@context": {}}`,
			"https://ctx.test/x2": `{"@context": {}}`,
			"https://ctx.test/u0": `{"@context": [{"t": {"@id": "https://ex.org/t", "@context": "https://ctx.test/ua"}}, "https://ctx.test/x1",
				"https://ctx.test/x2", {"t": {"@id": "https://ex.org/t", "@context": "https://ctx.test/ua"}}]}`,
		}, `{"@context": "https://ctx.test/u0", "@id": "https://ex.org/n"}`, 3},
	} {
		t.Run(name, func(t *testing.T) {
			if err := expandWith(t, tc.contexts, tc.doc, tc.maxRemote); err == nil {
				t.Fatal("accepted, want an error")
			}
		})
	}
}
