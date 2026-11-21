package ld

import (
	"reflect"
	"testing"
)

// JSON-LD 1.1 API §5.1.2 step 13 reads the container mapping and reverse
// flag of a key from the active context; the key's scoped context only
// changes how its value is expanded. A scoped context of null must not drop
// them.
func TestNullScopedContextKeepsTermMapping(t *testing.T) {
	type m = map[string]interface{}
	type a = []interface{}
	const iri = "http://ex.org/t"
	q := func(v string) a { return a{m{"@value": v}} }

	cases := []struct {
		name       string
		definition m
		value      interface{}
		key        string // property of the expanded node that holds the result
		want       interface{}
	}{
		{
			name:       "@language",
			definition: m{"@container": "@language"},
			value:      m{"en": "Hi"},
			key:        iri,
			want:       a{m{"@value": "Hi", "@language": "en"}},
		},
		{
			name:       "@index",
			definition: m{"@container": "@index"},
			value:      m{"i1": "x"},
			key:        iri,
			want:       a{m{"@value": "x", "@index": "i1"}},
		},
		{
			name:       "@id",
			definition: m{"@container": "@id"},
			value:      m{"urn:n1": m{"http://ex.org/q": "v"}},
			key:        iri,
			want:       a{m{"@id": "urn:n1", "http://ex.org/q": q("v")}},
		},
		{
			name:       "@type",
			definition: m{"@container": "@type"},
			value:      m{"http://ex.org/T": m{"@id": "urn:n2"}},
			key:        iri,
			want:       a{m{"@id": "urn:n2", "@type": a{"http://ex.org/T"}}},
		},
		{
			name:       "@list",
			definition: m{"@container": "@list"},
			value:      a{"x", "y"},
			key:        iri,
			want:       a{m{"@list": a{m{"@value": "x"}, m{"@value": "y"}}}},
		},
		{
			name:       "@graph @index",
			definition: m{"@container": a{"@graph", "@index"}},
			value:      m{"i1": m{"@id": "urn:n3", "http://ex.org/q": "v"}},
			key:        iri,
			want: a{m{"@index": "i1", "@graph": a{
				m{"@id": "urn:n3", "http://ex.org/q": q("v")},
			}}},
		},
		{
			name:       "@graph @id",
			definition: m{"@container": a{"@graph", "@id"}},
			value:      m{"urn:g1": m{"http://ex.org/q": "v"}},
			key:        iri,
			want: a{m{"@id": "urn:g1", "@graph": a{
				m{"http://ex.org/q": q("v")},
			}}},
		},
		{
			name:       "@reverse",
			definition: m{"@reverse": iri},
			value:      m{"@id": "urn:n4"},
			key:        "@reverse",
			want:       m{iri: a{m{"@id": "urn:n4"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def := m{"@context": nil}
			if _, isReverse := tc.definition["@reverse"]; !isReverse {
				def["@id"] = iri
			}
			for k, v := range tc.definition {
				def[k] = v
			}
			doc := m{
				"@context": m{"@version": 1.1, "t": def},
				"@id":      "urn:s",
				"t":        tc.value,
			}
			out, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
			if err != nil {
				t.Fatal(err)
			}
			if len(out) != 1 {
				t.Fatalf("got %d nodes: %v", len(out), out)
			}
			got := out[0].(m)[tc.key]
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v\nwant %#v", got, tc.want)
			}
		})
	}
}
