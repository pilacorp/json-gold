package ld

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// JSON-LD 1.1 API §4.2.2 step 27: a protected term keeps its definition, so
// redefining it as null outside a property-scoped context is an error, while
// redefining a protected null term as null again is allowed.
func TestProtectedTermNullRedefinition(t *testing.T) {
	cases := []struct {
		doc     string
		wantErr bool
	}{
		{`{"@context": [{"@version": 1.1, "@protected": true, "term": {"@id": "http://example.com/t"}}, {"term": null}], "term": "x"}`, true},
		{`{"@context": [{"@version": 1.1, "@protected": true, "term": null}, {"term": {"@id": null}}], "term": "x"}`, false},
	}
	for _, tc := range cases {
		var input interface{}
		if err := json.Unmarshal([]byte(tc.doc), &input); err != nil {
			t.Fatal(err)
		}
		_, err := NewJsonLdProcessor().Expand(input, NewJsonLdOptions(""))
		var ldErr *JsonLdError
		gotErr := errors.As(err, &ldErr) && ldErr.Code == ProtectedTermRedefinition
		if gotErr != tc.wantErr || (err != nil && !gotErr) {
			t.Errorf("%s: got %v, want protected term redefinition = %v", tc.doc, err, tc.wantErr)
		}
	}
}

// A term definition with a keyword-like @id or @reverse is ignored and the
// term keeps its previous definition, so a protected term cannot be removed
// that way and then redefined. jsonld.js 8.3.3 gives the same errors.
func TestProtectedTermSurvivesIgnoredRedefinition(t *testing.T) {
	for _, ctx := range []string{
		`[{"@protected":true,"t":"http://ex.org/t"},{"t":{"@reverse":"@x"}},{"t":"http://evil.org/t"}]`,
		`[{"@protected":true,"t":"http://ex.org/t"},{"t":{"@id":"@x"}},{"t":"http://evil.org/t"}]`,
		`[{"@protected":true,"t":null},{"t":{"@id":"@x"}},{"t":"http://evil.org/t"}]`,
	} {
		t.Run(ctx, func(t *testing.T) {
			var c interface{}
			if err := json.Unmarshal([]byte(ctx), &c); err != nil {
				t.Fatal(err)
			}
			_, err := NewJsonLdProcessor().Expand(map[string]interface{}{"@context": c, "t": "v"}, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != ProtectedTermRedefinition {
				t.Fatalf("got %v, want %s", err, ProtectedTermRedefinition)
			}
		})
	}
}

// §4.2.2 checks every entry of a term definition whose @id is null; only its
// IRI mapping is null. The codes are those jsonld.js 8.3.3 reports.
func TestNullTermEntriesAreChecked(t *testing.T) {
	tests := []struct {
		def  string
		code ErrorCode
	}{
		{`{"@id":null,"@type":5}`, InvalidTypeMapping},
		{`{"@id":null,"@foo":1}`, InvalidTermDefinition},
		{`{"@id":null,"@container":"@foo"}`, InvalidContainerMapping},
		{`{"@id":null,"@reverse":"http://ex.org/r"}`, InvalidReverseProperty},
	}
	for _, tc := range tests {
		t.Run(tc.def, func(t *testing.T) {
			var def interface{}
			if err := json.Unmarshal([]byte(tc.def), &def); err != nil {
				t.Fatal(err)
			}
			doc := map[string]interface{}{"@context": map[string]interface{}{"t": def}, "t": "v"}
			_, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
		})
	}
}

// A term defined as null may have a compact IRI, an IRI or a relative path
// as its key (§4.2.2 step 7); its value is then dropped. Expected values are
// those of jsonld.js 8.3.3.
func TestNullTermWithIRIKey(t *testing.T) {
	for _, key := range []string{"ex:foo", "http://ex.org/x", "a/b"} {
		t.Run(key, func(t *testing.T) {
			doc := map[string]interface{}{
				"@context": map[string]interface{}{"ex": "http://ex.org/", "q": "http://ex.org/q", key: nil},
				key:        "z",
				"q":        "y",
			}
			got, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			want := []interface{}{map[string]interface{}{"http://ex.org/q": []interface{}{map[string]interface{}{"@value": "y"}}}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}
