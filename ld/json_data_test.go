package ld

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

// A document built in Go with types encoding/json never decodes into gives
// the same RDF as the JSON text it marshals to, instead of panicking.
func TestNormalizeGoTypedDocument(t *testing.T) {
	type name struct {
		Value string `json:"@value"`
	}
	context := map[string]string{"@vocab": "https://ex.org/"}
	values := map[string]interface{}{
		"int":     1,
		"int64":   int64(2),
		"uint8":   uint8(3),
		"float32": float32(1.5),
		"strings": []string{"a", "b"},
		"ints":    []int{1, 2},
		"map":     map[string]string{"@value": "x"},
		"maps":    []map[string]interface{}{{"@value": "y"}},
		"struct":  name{Value: "z"},
	}
	for label, value := range values {
		doc := map[string]interface{}{"@context": context, "@id": "urn:s", "p": value}
		original := CloneDocument(doc)
		got, err := normalizeForTest(doc)
		if err != nil {
			t.Fatalf("%s: Normalize: %v", label, err)
		}
		want, err := normalizeForTest(decodeJSONForTest(t, doc))
		if err != nil {
			t.Fatalf("%s: Normalize of the JSON text: %v", label, err)
		}
		if got != want || got == "" {
			t.Fatalf("%s: Normalize = %q, want %q", label, got, want)
		}
		if !reflect.DeepEqual(doc, original) {
			t.Fatalf("%s: Normalize changed the input to %v", label, doc)
		}
	}
}

// A value encoding/json cannot marshal is refused with an error.
func TestNormalizeRefusesValueWithoutJSONForm(t *testing.T) {
	doc := map[string]interface{}{"@id": "urn:s", "https://ex.org/p": make(chan int)}
	_, err := normalizeForTest(doc)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != InvalidInput {
		t.Fatalf("Normalize error = %v, want %s", err, InvalidInput)
	}
}

func normalizeForTest(doc interface{}) (string, error) {
	opts := NewJsonLdOptions("")
	opts.Algorithm = AlgorithmURDNA2015
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().Normalize(doc, opts)
	if err != nil {
		return "", err
	}
	return out.(string), nil
}

// decodeJSONForTest returns value as its JSON text decoded with UseNumber,
// the form the processor reads a Go-typed value as.
func decodeJSONForTest(t *testing.T, value interface{}) interface{} {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var decoded interface{}
	if err := dec.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// goTypedContextForTest is a context built in Go with types encoding/json
// never decodes into: a map[string]string term definition and a []string.
func goTypedContextForTest() map[string]interface{} {
	return map[string]interface{}{
		"@vocab": "https://ex.org/",
		"knows":  map[string]string{"@id": "https://ex.org/knows", "@type": "@id"},
		"tags":   map[string]interface{}{"@id": "https://ex.org/tags", "@container": []string{"@set"}},
	}
}

func documentForContextTest() map[string]interface{} {
	return map[string]interface{}{
		"@id":                  "urn:s",
		"https://ex.org/knows": map[string]interface{}{"@id": "urn:o"},
		"https://ex.org/tags":  []interface{}{"a"},
	}
}

// Every place a caller hands the processor a context reads a Go-typed one as
// its JSON form.
func TestProcessorReadsGoTypedContext(t *testing.T) {
	goContext := goTypedContextForTest()
	jsonContext := decodeJSONForTest(t, goContext)
	original := CloneDocument(goContext)
	proc := NewJsonLdProcessor()

	runs := map[string]func(context interface{}) (interface{}, error){
		"Compact": func(context interface{}) (interface{}, error) {
			return proc.Compact(documentForContextTest(), context, nil)
		},
		"Flatten": func(context interface{}) (interface{}, error) {
			return proc.Flatten(documentForContextTest(), context, nil)
		},
		"ExpandContext": func(context interface{}) (interface{}, error) {
			opts := NewJsonLdOptions("")
			opts.ExpandContext = context
			return proc.Expand(map[string]interface{}{"@id": "urn:s", "knows": "urn:o", "tags": "a"}, opts)
		},
		"Context.Parse": func(context interface{}) (interface{}, error) {
			parsed, err := NewContext(nil, NewJsonLdOptions("")).Parse(context)
			if err != nil {
				return nil, err
			}
			return parsed.Serialize()
		},
		"RDFDataset.ParseContext": func(context interface{}) (interface{}, error) {
			ds := NewRDFDataset()
			if err := ds.ParseContext(context, nil); err != nil {
				return nil, err
			}
			return ds.context, nil
		},
	}
	for label, run := range runs {
		got, err := run(goContext)
		if err != nil {
			t.Fatalf("%s with the Go context: %v", label, err)
		}
		want, err := run(jsonContext)
		if err != nil {
			t.Fatalf("%s with the JSON context: %v", label, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
		if !reflect.DeepEqual(goContext, original) {
			t.Fatalf("%s changed the context to %v", label, goContext)
		}
	}
}

// A document loader may return Go-typed documents, for the input and for a
// remote context, and the processor does not change them.
func TestProcessorReadsGoTypedLoadedDocuments(t *testing.T) {
	loaded := map[string]interface{}{
		"https://ex.org/context": map[string]interface{}{"@context": goTypedContextForTest()},
		"https://ex.org/doc": map[string]interface{}{
			"@context": "https://ex.org/context",
			"@id":      "urn:s",
			"knows":    "urn:o",
			"tags":     []string{"a", "b"},
			"count":    []int{1},
		},
	}
	expand := func(docs map[string]interface{}) (interface{}, error) {
		loader := NewCachingDocumentLoader(nil)
		for u, doc := range docs {
			loader.AddDocument(u, doc)
		}
		opts := NewJsonLdOptions("")
		opts.DocumentLoader = loader
		return NewJsonLdProcessor().Expand("https://ex.org/doc", opts)
	}
	original := CloneDocument(loaded)
	got, err := expand(loaded)
	if err != nil {
		t.Fatalf("Expand of Go-typed loaded documents: %v", err)
	}
	want, err := expand(decodeJSONForTest(t, loaded).(map[string]interface{}))
	if err != nil {
		t.Fatalf("Expand of the JSON documents: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(loaded, original) {
		t.Fatalf("Expand changed the loaded documents to %v", loaded)
	}
}

// A remote context that has no JSON form fails as a remote context.
func TestRemoteContextWithoutJSONForm(t *testing.T) {
	loader := NewCachingDocumentLoader(nil)
	loader.AddDocument("https://ex.org/context", map[string]interface{}{
		"@context": map[string]interface{}{"p": make(chan int)},
	})
	opts := NewJsonLdOptions("")
	opts.DocumentLoader = loader
	doc := map[string]interface{}{"@context": "https://ex.org/context", "@id": "urn:s"}
	_, err := NewJsonLdProcessor().Expand(doc, opts)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != LoadingRemoteContextFailed {
		t.Fatalf("Expand error = %v, want %s", err, LoadingRemoteContextFailed)
	}
}

// @version is read as a number however it was decoded.
func TestContextVersionAsJSONNumber(t *testing.T) {
	for label, version := range map[string]interface{}{
		"json.Number": json.Number("1.1"),
		"float32":     float32(1.1),
	} {
		parsed, err := NewContext(nil, NewJsonLdOptions("")).Parse(map[string]interface{}{"@version": version})
		if err != nil {
			t.Fatalf("%s: Parse: %v", label, err)
		}
		if parsed.Values.Version == nil || *parsed.Values.Version != 1.1 {
			t.Fatalf("%s: Version = %v, want 1.1", label, parsed.Values.Version)
		}
	}
	_, err := NewContext(nil, NewJsonLdOptions("")).Parse(map[string]interface{}{"@version": json.Number("1.0")})
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != InvalidVersionValue {
		t.Fatalf("Parse of @version 1.0 error = %v, want %s", err, InvalidVersionValue)
	}
}

// An int64 above 2^53 keeps every digit, as JSON text decoded with UseNumber does.
func TestExpandKeepsGoIntegerDigits(t *testing.T) {
	doc := map[string]interface{}{"@id": "urn:s", "https://ex.org/p": []int64{9007199254740993}}
	expanded, err := NewJsonLdProcessor().Expand(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	value := expanded[0].(map[string]interface{})["https://ex.org/p"].([]interface{})[0].(map[string]interface{})["@value"]
	if value != json.Number("9007199254740993") {
		t.Fatalf("@value = %#v, want json.Number 9007199254740993", value)
	}
}

// Values JSON cannot hold are refused: NaN, infinities and a value that
// contains itself.
func TestNormalizeRefusesNonJSONValues(t *testing.T) {
	cycle := map[string]interface{}{"@id": "urn:s"}
	cycle["https://ex.org/p"] = cycle
	list := []interface{}{nil}
	list[0] = list
	for label, doc := range map[string]interface{}{
		"NaN":        map[string]interface{}{"@id": "urn:s", "https://ex.org/p": math.NaN()},
		"+Inf":       map[string]interface{}{"@id": "urn:s", "https://ex.org/p": math.Inf(1)},
		"map cycle":  cycle,
		"list cycle": map[string]interface{}{"@id": "urn:s", "https://ex.org/p": list},
	} {
		_, err := normalizeForTest(doc)
		var ldErr *JsonLdError
		if !errors.As(err, &ldErr) || ldErr.Code != InvalidInput {
			t.Fatalf("%s: Normalize error = %v, want %s", label, err, InvalidInput)
		}
	}
}

// With several values that have no JSON form, the error names the same one
// on every run.
func TestNormalizeNamesSameRefusedValue(t *testing.T) {
	doc := map[string]interface{}{"@id": "urn:s"}
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		doc["https://ex.org/"+key] = make(chan int)
	}
	doc["https://ex.org/a"] = func() {}
	_, first := normalizeForTest(doc)
	if first == nil {
		t.Fatal("Normalize accepted a func value")
	}
	for i := 0; i < 50; i++ {
		if _, err := normalizeForTest(doc); err == nil || err.Error() != first.Error() {
			t.Fatalf("run %d error = %v, want %v", i, err, first)
		}
	}
}

// A container shared at two depths is held to the depth limit where it sits
// deepest, whichever place the walk reaches first.
func TestToJSONDataLimitsSharedContainerDepth(t *testing.T) {
	nest := func(levels int, inner interface{}) interface{} {
		for i := 0; i < levels; i++ {
			inner = []interface{}{inner}
		}
		return inner
	}
	shallow := nest(6000, "x")
	deep := nest(6000, shallow)
	for label, doc := range map[string]interface{}{
		"shallow first": map[string]interface{}{"a": shallow, "b": deep},
		"deep first":    map[string]interface{}{"b": deep, "c": shallow},
	} {
		_, err := toJSONData(doc)
		var ldErr *JsonLdError
		if !errors.As(err, &ldErr) || ldErr.Code != InvalidInput {
			t.Fatalf("%s: toJSONData error = %v, want %s", label, err, InvalidInput)
		}
	}
	// A Go value is held to the limit with the nesting its JSON form adds.
	raw := json.RawMessage(strings.Repeat("[", 2000) + "1" + strings.Repeat("]", 2000))
	_, err := toJSONData(nest(9000, raw))
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != InvalidInput {
		t.Fatalf("toJSONData of a deep Go value error = %v, want %s", err, InvalidInput)
	}
	if _, err := toJSONData(map[string]interface{}{"a": shallow, "b": shallow}); err != nil {
		t.Fatalf("toJSONData of a container shared within the limit: %v", err)
	}
}

// A json.Number whose text is not a JSON number is refused. An empty one is
// read as 0, as encoding/json writes it.
func TestNormalizeRefusesInvalidJSONNumberText(t *testing.T) {
	empty, err := normalizeForTest(map[string]interface{}{"@id": "urn:s", "https://ex.org/p": json.Number("")})
	if err != nil || !strings.Contains(empty, `"0"^^`) {
		t.Fatalf("Normalize of an empty json.Number = %q, %v, want the integer 0", empty, err)
	}
	for _, text := range []string{"NaN", "Inf", "0x10", "+1", "01", "1_0"} {
		doc := map[string]interface{}{"@id": "urn:s", "https://ex.org/p": json.Number(text)}
		_, err := normalizeForTest(doc)
		var ldErr *JsonLdError
		if !errors.As(err, &ldErr) || ldErr.Code != InvalidInput {
			t.Fatalf("%q: Normalize error = %v, want %s", text, err, InvalidInput)
		}
	}
}

// The limit holds to the exact level through a memo hit, for map and slice
// chains: the leaf of a shared container may sit at depth 10000, not 10001.
func TestToJSONDataSharedContainerDepthBoundary(t *testing.T) {
	chains := map[string]func(levels int, inner interface{}) interface{}{
		"map": func(levels int, inner interface{}) interface{} {
			for i := 0; i < levels; i++ {
				inner = map[string]interface{}{"k": inner}
			}
			return inner
		},
		"slice": func(levels int, inner interface{}) interface{} {
			for i := 0; i < levels; i++ {
				inner = []interface{}{inner}
			}
			return inner
		},
	}
	for label, nest := range chains {
		shared := nest(5000, "x")
		// "a" is walked first, so "b" reaches shared through the memo. The
		// leaf of shared sits at 1 + above + 5000.
		for above, wantErr := range map[int]bool{4999: false, 5000: true} {
			doc := map[string]interface{}{"a": shared, "b": nest(above, shared)}
			_, err := toJSONData(doc)
			if (err != nil) != wantErr {
				t.Fatalf("%s chain with the leaf at depth %d: error = %v, want error %v", label, 1+above+5000, err, wantErr)
			}
		}
	}
}
