package ld

import (
	"encoding/json"
	"testing"
)

func compactTypeMap(t *testing.T, doc string) string {
	t.Helper()
	var in interface{}
	if err := json.Unmarshal([]byte(doc), &in); err != nil {
		t.Fatal(err)
	}
	ctx := map[string]interface{}{
		"@vocab": "http://ex.org/",
		"p":      map[string]interface{}{"@type": "@id", "@container": "@type"},
	}
	out, err := NewJsonLdProcessor().Compact(in, ctx, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(out["p"])
	return string(got)
}

// In a type map the first type is the key and the other types stay on the
// item; only an item left with just its @id is compacted to an IRI
// (JSON-LD 1.1 API §6.1.2). An item already compacted to an IRI string has
// no type, so it goes under @none.
func TestCompactTypeMap(t *testing.T) {
	cases := []struct{ name, doc, want string }{
		{"two types", `{"http://ex.org/p":{"@id":"http://ex.org/o","@type":["http://ex.org/A","http://ex.org/B"]}}`,
			`{"A":{"@id":"http://ex.org/o","@type":"B"}}`},
		{"one type", `{"http://ex.org/p":{"@id":"http://ex.org/o","@type":"http://ex.org/A"}}`,
			`{"A":"http://ex.org/o"}`},
		{"no type", `{"http://ex.org/p":{"@id":"http://ex.org/o"}}`,
			`{"@none":"http://ex.org/o"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compactTypeMap(t, c.doc); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}
