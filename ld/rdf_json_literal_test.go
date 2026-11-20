package ld

import (
	"strings"
	"testing"
)

// An @json term whose value is a JSON array must produce a valid rdf:JSON
// literal with the canonicalized JSON array text. Before this fix, the
// array fell through the type switch and produced an empty literal, so two
// different arrays had identical N-Quads.
func TestJsonLiteralArray(t *testing.T) {
	doc := map[string]interface{}{
		"@context": map[string]interface{}{
			"claim": map[string]interface{}{
				"@id":   "https://example.org/claim",
				"@type": "@json",
			},
		},
		"@id":   "urn:ex:1",
		"claim": []interface{}{"aaa"},
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	want := `"[\"aaa\"]"^^<http://www.w3.org/1999/02/22-rdf-syntax-ns#JSON>`
	if !strings.Contains(quads, want) {
		t.Fatalf("array literal missing:\n%s\nwant %s", quads, want)
	}

	doc["claim"] = []interface{}{"bbb", "ccc"}
	out2, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads2 := out2.(string)
	if quads == quads2 {
		t.Fatal("two different arrays produced identical N-Quads")
	}
}

// An @json term whose value is a string must be JSON-marshaled (quoted),
// not passed raw to the canonicalizer.
func TestJsonLiteralString(t *testing.T) {
	doc := map[string]interface{}{
		"@context": map[string]interface{}{
			"claim": map[string]interface{}{
				"@id":   "https://example.org/claim",
				"@type": "@json",
			},
		},
		"@id":   "urn:ex:1",
		"claim": "hello",
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	// The literal value must be the JSON string with quotes: "hello"
	want := `"\"hello\""^^<http://www.w3.org/1999/02/22-rdf-syntax-ns#JSON>`
	if !strings.Contains(quads, want) {
		t.Fatalf("string literal wrong:\n%s\nwant %s", quads, want)
	}
}

// A map value under @json still works (regression guard).
func TestJsonLiteralMap(t *testing.T) {
	doc := map[string]interface{}{
		"@context": map[string]interface{}{
			"claim": map[string]interface{}{
				"@id":   "https://example.org/claim",
				"@type": "@json",
			},
		},
		"@id":   "urn:ex:1",
		"claim": map[string]interface{}{"b": 2, "a": 1},
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	want := `"{\"a\":1,\"b\":2}"^^<http://www.w3.org/1999/02/22-rdf-syntax-ns#JSON>`
	if !strings.Contains(quads, want) {
		t.Fatalf("map literal wrong:\n%s\nwant %s", quads, want)
	}
}

// A primitive under @json is written in JCS form (RFC 8785 §3.2.2.2): only
// quote, backslash and control characters are escaped, so <, > and & stay as
// they are. encoding/json would write them as \u003c, \u003e and \u0026.
func TestJsonLiteralStringIsJCS(t *testing.T) {
	doc := map[string]interface{}{
		"@context": map[string]interface{}{
			"claim": map[string]interface{}{
				"@id":   "https://example.org/claim",
				"@type": "@json",
			},
		},
		"@id":   "urn:ex:1",
		"claim": "a<b&c>",
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	want := `"\"a<b&c>\""^^<http://www.w3.org/1999/02/22-rdf-syntax-ns#JSON>`
	if !strings.Contains(quads, want) {
		t.Fatalf("string literal not in JCS form:\n%s\nwant %s", quads, want)
	}
}
