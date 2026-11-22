package ld

import (
	"reflect"
	"testing"
)

// A term set to null has a term definition without an IRI mapping. Using
// it as the prefix of a compact IRI, or meeting it as a compact IRI or an IRI
// while compacting, must not dereference that missing mapping.
func TestNullTermIsNotUsedAsAMapping(t *testing.T) {
	expandCases := map[string]struct {
		context map[string]interface{}
		want    []interface{}
	}{
		// IRI expansion §5.2.2 step 6.4 needs a non-null IRI mapping, so
		// "a:x" stays the IRI it already is.
		"value whose prefix is null": {
			context: map[string]interface{}{"a": nil, "b": "a:x"},
			want:    []interface{}{map[string]interface{}{"a:x": []interface{}{map[string]interface{}{"@value": "v"}}}},
		},
		"@id whose prefix is null": {
			context: map[string]interface{}{"a": nil, "b": map[string]interface{}{"@id": "a:x"}},
			want:    []interface{}{map[string]interface{}{"a:x": []interface{}{map[string]interface{}{"@value": "v"}}}},
		},
		// §4.2.2 step 15 has no IRI mapping to concatenate, so the term is
		// an IRI itself, as IRI expansion would also read it.
		"term whose prefix is null": {
			context: map[string]interface{}{"a": nil, "a:x": map[string]interface{}{"@type": "@id"}, "b": "a:x"},
			want:    []interface{}{map[string]interface{}{"a:x": []interface{}{map[string]interface{}{"@value": "v"}}}},
		},
	}
	for name, tc := range expandCases {
		t.Run(name, func(t *testing.T) {
			got, err := NewJsonLdProcessor().Expand(map[string]interface{}{"@context": tc.context, "b": "v"}, NewJsonLdOptions(""))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}

	compactCases := map[string]struct {
		context map[string]interface{}
		input   map[string]interface{}
		want    map[string]interface{}
	}{
		// IRI compaction §6.2.2 step 9.4: a compact IRI whose own term
		// definition maps elsewhere, here to nothing, is not used.
		"compact IRI set to null": {
			context: map[string]interface{}{"a": "https://e/", "a:x": nil},
			input:   map[string]interface{}{"@id": "https://e/s", "https://e/p": map[string]interface{}{"@id": "https://e/x"}},
			want:    map[string]interface{}{"@id": "a:s", "a:p": map[string]interface{}{"@id": "https://e/x"}},
		},
		"IRI set to null": {
			context: map[string]interface{}{"https://e/x": nil, "p": map[string]interface{}{"@id": "https://e/p", "@type": "@vocab"}},
			input:   map[string]interface{}{"https://e/p": map[string]interface{}{"@id": "https://e/x"}},
			want:    map[string]interface{}{"p": "https://e/x"},
		},
	}
	for name, tc := range compactCases {
		t.Run(name, func(t *testing.T) {
			got, err := NewJsonLdProcessor().Compact(tc.input, tc.context, NewJsonLdOptions(""))
			if err != nil {
				t.Fatal(err)
			}
			delete(got, "@context")
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

// Serializing a context writes a term set to null as null; it has no
// definition to read.
func TestNullTermIsSerializedAsNull(t *testing.T) {
	ctx, err := NewContext(nil, NewJsonLdOptions("")).Parse(map[string]interface{}{"ex": nil})
	if err != nil {
		t.Fatal(err)
	}
	out, err := ctx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]interface{}{"ex": nil}; !reflect.DeepEqual(out["@context"], want) {
		t.Fatalf("@context = %v, want %v", out["@context"], want)
	}
}
