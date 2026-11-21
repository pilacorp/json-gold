package ld

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Malformed input must end in a JSON-LD error, never a panic: callers run
// these APIs on documents they did not write.
func TestMalformedInputReturnsError(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		code ErrorCode // empty: any outcome but a panic
	}{
		{"context @protected string", `{"@context":{"@protected":"yes","a":"http://ex.org/a"},"a":"x"}`, InvalidProtectedValue},
		{"context @protected number", `{"@context":{"@protected":1,"a":"http://ex.org/a"},"a":"x"}`, InvalidProtectedValue},
		{"term @protected string", `{"@context":{"a":{"@id":"http://ex.org/a","@protected":"yes"}},"a":"x"}`, InvalidProtectedValue},
		{"empty key with number", `{"":0}`, ""},
		{"empty key with array", `{"":[1,2,[3]]}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var doc interface{}
			if err := json.Unmarshal([]byte(c.doc), &doc); err != nil {
				t.Fatal(err)
			}
			_, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
			if c.code == "" {
				return
			}
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != c.code {
				t.Fatalf("got %v, want %q", err, c.code)
			}
		})
	}
}

// A key that is not a term and does not expand to an IRI is dropped, so the
// empty key leaves nothing behind (JSON-LD 1.1 API §5.1.2 step 13.3).
func TestExpandDropsEmptyKey(t *testing.T) {
	var doc interface{}
	_ = json.Unmarshal([]byte(`{"":0,"http://ex.org/p":"v"}`), &doc)
	out, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(out)
	if want := `[{"http://ex.org/p":[{"@value":"v"}]}]`; string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// A json.Number that float64 cannot hold, from a document decoded with
// UseNumber, must fail ToRDF with a JSON-LD error instead of a panic.
func TestToRDFRejectsNumberOutOfRange(t *testing.T) {
	dec := json.NewDecoder(strings.NewReader(`{"@id":"urn:s","http://ex.org/p":1e400}`))
	dec.UseNumber()
	var doc interface{}
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	_, err := NewJsonLdProcessor().ToRDF(doc, NewJsonLdOptions(""))
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != InvalidValueObjectValue {
		t.Fatalf("got %v, want %q", err, InvalidValueObjectValue)
	}
}
