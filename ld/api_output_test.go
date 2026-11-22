package ld

import (
	"errors"
	"reflect"
	"testing"
)

// JSON-LD 1.1 API §9.1 compact() step 9.3 adds the context provided even when
// nothing compacts: an empty array becomes an empty map (step 9.1) and then
// gets @context. jsonld.js does the same.
func TestCompactEmptyOutputKeepsContext(t *testing.T) {
	ctx := map[string]interface{}{"ex": "https://e.org/"}
	out, err := NewJsonLdProcessor().Compact(map[string]interface{}{"@id": "https://e.org/s"},
		map[string]interface{}{"@context": ctx}, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]interface{}{"@context": ctx}; !reflect.DeepEqual(out, want) {
		t.Fatalf("Compact = %v, want %v", out, want)
	}
}

// An empty context stays out of the output, as compact tests 0001 and 0003 of
// the W3C suite expect.
func TestCompactEmptyOutputWithEmptyContext(t *testing.T) {
	out, err := NewJsonLdProcessor().Compact(map[string]interface{}{"@id": "https://e.org/s"},
		map[string]interface{}{}, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("Compact = %v, want {}", out)
	}
}

// flatten() step 6.1 compacts whenever a context is given, and a flattened
// document always holds its nodes in @graph, so an empty result is
// {"@context": ..., "@graph": []}, as jsonld.js returns, not a bare array.
func TestFlattenEmptyOutputWithContext(t *testing.T) {
	ctx := map[string]interface{}{"ex": "https://e.org/"}
	out, err := NewJsonLdProcessor().Flatten(map[string]interface{}{"@id": "https://e.org/s"},
		map[string]interface{}{"@context": ctx}, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"@context": ctx, "@graph": []interface{}{}}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("Flatten = %#v, want %#v", out, want)
	}
}

// Without a context flatten() does not compact, so the result stays an array.
func TestFlattenEmptyOutputWithoutContext(t *testing.T) {
	out, err := NewJsonLdProcessor().Flatten(map[string]interface{}{"@id": "https://e.org/s"}, nil, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if list, ok := out.([]interface{}); !ok || len(list) != 0 {
		t.Fatalf("Flatten = %#v, want []", out)
	}
}

// FromRDF with OutputForm compacted or flattened compacts with the dataset's
// prefixes. N-Quads has none, so the context is empty and the output is plain
// compacted data, not an "invalid local context" error.
func TestFromRDFOutputForms(t *testing.T) {
	const nquads = "<http://e.org/s> <http://e.org/p> \"v\" .\n"
	node := map[string]interface{}{"@id": "http://e.org/s", "http://e.org/p": "v"}
	for form, want := range map[string]interface{}{
		"compacted": node,
		"flattened": map[string]interface{}{"@graph": []interface{}{node}},
	} {
		opts := NewJsonLdOptions("")
		opts.OutputForm = form
		got, err := NewJsonLdProcessor().FromRDF(nquads, opts)
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %#v, want %#v", form, got, want)
		}
	}
}

// compact() step 9.3 adds @context only when a context was given; a nil one
// must not show up as "@context": null.
func TestCompactNilContextAddsNoContext(t *testing.T) {
	out, err := NewJsonLdProcessor().Compact(map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := out["@context"]; has {
		t.Fatalf("Compact = %v, want no @context", out)
	}
}

type frameLoader struct{ frame interface{} }

func (l frameLoader) LoadDocument(u string) (*RemoteDocument, error) {
	return &RemoteDocument{DocumentURL: u, Document: l.frame}, nil
}

// A frame that is not a single object is an invalid frame error, whether it
// is given directly or loaded from an IRI, not a panic.
func TestFrameRefusesNonObjectFrame(t *testing.T) {
	in := map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"}
	for name, frame := range map[string]interface{}{
		"nil":          nil,
		"array":        []interface{}{map[string]interface{}{}},
		"loaded array": "https://e.org/frame",
	} {
		opts := NewJsonLdOptions("")
		opts.DocumentLoader = frameLoader{frame: []interface{}{map[string]interface{}{}}}
		_, err := NewJsonLdProcessor().Frame(in, frame, opts)
		var ldErr *JsonLdError
		if !errors.As(err, &ldErr) || ldErr.Code != InvalidFrame {
			t.Fatalf("%s: err = %v, want %s", name, err, InvalidFrame)
		}
	}
}

// Framing 1.1 frame() takes the frame as an IRI too: it is loaded like the
// input and its @context is copied to the output.
func TestFrameLoadsFrameIRI(t *testing.T) {
	ctx := map[string]interface{}{"ex": "https://e.org/"}
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = frameLoader{frame: map[string]interface{}{"@context": ctx}}
	in := map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"}
	out, err := NewJsonLdProcessor().Frame(in, "https://e.org/frame", opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"@context": ctx, "@id": "ex:s", "ex:p": "v"}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("Frame = %#v, want %#v", out, want)
	}
}

// With omitGraph and a frame without @context the output has no @context
// entry, rather than "@context": null.
func TestFrameOmitGraphWithoutContext(t *testing.T) {
	opts := NewJsonLdOptions("")
	opts.OmitGraph = true
	out, err := NewJsonLdProcessor().Frame(map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"},
		map[string]interface{}{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"}; !reflect.DeepEqual(out, want) {
		t.Fatalf("Frame = %#v, want %#v", out, want)
	}
}

// Normalize parses its input with the inputFormat option, whatever the
// output format is.
func TestNormalizeReadsInputFormat(t *testing.T) {
	opts := NewJsonLdOptions("")
	opts.InputFormat = "application/n-quads"
	opts.Algorithm = AlgorithmURDNA2015
	out, err := NewJsonLdProcessor().Normalize("<https://e.org/s> <https://e.org/p> \"v\" .\n", opts)
	if err != nil {
		t.Fatal(err)
	}
	if quads := out.(*RDFDataset).Graphs["@default"]; len(quads) != 1 {
		t.Fatalf("default graph holds %d quads, want 1", len(quads))
	}
}

// useNamespaces takes the prefixes from the input's @context, for a single
// object as well as for an array of them.
func TestToRDFNamespacesFromInputContext(t *testing.T) {
	ctx := map[string]interface{}{"ex": "https://e.org/"}
	node := map[string]interface{}{"@context": ctx, "@id": "ex:s", "ex:p": "v"}
	for name, in := range map[string]interface{}{
		"object":     node,
		"array":      []interface{}{node},
		"array maps": []map[string]interface{}{node},
	} {
		opts := NewJsonLdOptions("")
		opts.UseNamespaces = true
		out, err := NewJsonLdProcessor().ToRDF(in, opts)
		if err != nil {
			t.Fatal(err)
		}
		if got := out.(*RDFDataset).GetNamespaces(); !reflect.DeepEqual(got, map[string]string{"ex": "https://e.org/"}) {
			t.Fatalf("%s: namespaces = %v, want ex", name, got)
		}
	}
}

type mapLoader map[string]interface{}

func (l mapLoader) LoadDocument(u string) (*RemoteDocument, error) {
	doc, ok := l[u]
	if !ok {
		return nil, NewJsonLdError(LoadingDocumentFailed, u)
	}
	return &RemoteDocument{DocumentURL: u, Document: doc}, nil
}

// A remote input's relative @context resolves against the input's URL for
// the namespaces too, as it did when the input was expanded.
func TestToRDFNamespacesFromRemoteInput(t *testing.T) {
	opts := NewJsonLdOptions("")
	opts.UseNamespaces = true
	opts.DocumentLoader = mapLoader{
		"https://i.example/data/in.jsonld":  map[string]interface{}{"@context": "ctx.jsonld", "@id": "ex:s", "ex:p": "v"},
		"https://i.example/data/ctx.jsonld": map[string]interface{}{"@context": map[string]interface{}{"ex": "https://e.org/"}},
	}
	out, err := NewJsonLdProcessor().ToRDF("https://i.example/data/in.jsonld", opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.(*RDFDataset).GetNamespaces(); !reflect.DeepEqual(got, map[string]string{"ex": "https://e.org/"}) {
		t.Fatalf("namespaces = %v, want ex", got)
	}
}

// Framing 1.1 frame() resolves a remote frame's @context against the frame's
// URL, so the output is compacted with the context the frame was matched with.
func TestFrameRemoteFrameRelativeContext(t *testing.T) {
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = mapLoader{
		"https://f.example/frames/rel.jsonld": map[string]interface{}{"@context": "ctx.jsonld"},
		"https://f.example/frames/ctx.jsonld": map[string]interface{}{"@context": map[string]interface{}{"ex": "https://e.org/"}},
	}
	in := map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"}
	out, err := NewJsonLdProcessor().Frame(in, "https://f.example/frames/rel.jsonld", opts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"@context": "ctx.jsonld", "@id": "ex:s", "ex:p": "v"}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("Frame = %#v, want %#v", out, want)
	}
}

// ParseContext takes nil options as the defaults, as NewContext does.
func TestParseContextNilOptions(t *testing.T) {
	ds := NewRDFDataset()
	if err := ds.ParseContext(map[string]interface{}{"ex": "https://e.org/"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := ds.GetNamespaces(); !reflect.DeepEqual(got, map[string]string{"ex": "https://e.org/"}) {
		t.Fatalf("namespaces = %v, want ex", got)
	}
}

// A node reference under an @id map whose term coerces to @id or @vocab
// compacts to a plain IRI string, which has no @id entry to take the map key
// from, so it goes under @none (JSON-LD 1.1 API §6.1.2 step 12.8.9.10), as
// jsonld.js does. Expanding the result gives the input back.
func TestCompactIDMapItemCompactedToString(t *testing.T) {
	for _, coerce := range []string{"@id", "@vocab"} {
		ctx := map[string]interface{}{"a": map[string]interface{}{
			"@id": "https://ex.org/a", "@type": coerce, "@container": "@id"}}
		expanded := []interface{}{map[string]interface{}{
			"@id":              "urn:s",
			"https://ex.org/a": []interface{}{map[string]interface{}{"@id": "https://ex.org/o1"}},
		}}
		out, err := NewJsonLdProcessor().Compact(expanded, ctx, NewJsonLdOptions(""))
		if err != nil {
			t.Fatalf("%s: Compact: %v", coerce, err)
		}
		want := map[string]interface{}{
			"@context": ctx, "@id": "urn:s",
			"a": map[string]interface{}{"@none": "https://ex.org/o1"},
		}
		if !reflect.DeepEqual(out, want) {
			t.Fatalf("%s: Compact = %v, want %v", coerce, out, want)
		}
		back, err := NewJsonLdProcessor().Expand(out, NewJsonLdOptions(""))
		if err != nil {
			t.Fatalf("%s: Expand: %v", coerce, err)
		}
		if !reflect.DeepEqual(back, expanded) {
			t.Fatalf("%s: Expand(Compact) = %v, want %v", coerce, back, expanded)
		}
	}
}
