package ld

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func toNQuads(t *testing.T, doc string) string {
	t.Helper()
	var in interface{}
	if err := json.Unmarshal([]byte(doc), &in); err != nil {
		t.Fatal(err)
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(in, opts)
	if err != nil {
		t.Fatal(err)
	}
	return out.(string)
}

// Equal JSON literals on one property are one RDF literal, so the node map
// keeps one of them (JSON-LD 1.1 API §7.2.2 step 4.1 adds a value only when
// it is not already there).
func TestNodeMapDropsEqualJSONLiterals(t *testing.T) {
	got := toNQuads(t, `{"@id":"urn:s","http://ex.org/j":[
		{"@value":{"a":1,"b":[true,null]},"@type":"@json"},
		{"@value":{"b":[true,null],"a":1},"@type":"@json"},
		{"@value":{"a":2},"@type":"@json"}]}`)
	if n := strings.Count(got, "\n"); n != 2 {
		t.Fatalf("got %d quads, want 2:\n%s", n, got)
	}
}

// Values equal by @value, @type, @language and @index are kept once; any
// difference keeps both, in document order. Like jsonld.js, @direction is
// not compared, so values differing only by it make one quad.
func TestNodeMapDropsEqualValuesOnly(t *testing.T) {
	got := toNQuads(t, `{"@id":"urn:s","http://ex.org/p":[
		"a", "a", {"@value":"a","@language":"en"}, {"@value":"a","@language":"en"},
		{"@value":"1","@type":"http://ex.org/t"}, 1, 1, true, {"@id":"urn:o"}, {"@id":"urn:o"}]}`)
	want := `<urn:s> <http://ex.org/p> "a" .
<urn:s> <http://ex.org/p> "a"@en .
<urn:s> <http://ex.org/p> "1"^^<http://ex.org/t> .
<urn:s> <http://ex.org/p> "1"^^<http://www.w3.org/2001/XMLSchema#integer> .
<urn:s> <http://ex.org/p> "true"^^<http://www.w3.org/2001/XMLSchema#boolean> .
<urn:s> <http://ex.org/p> <urn:o> .
`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// Numbers compare by value, as toRDF writes them: -0 equals 0, and a
// json.Number equals any number with the same float64 value.
func TestNodeMapDropsValuesEqualByGoEquality(t *testing.T) {
	cases := []struct {
		name, doc string
		useNumber bool
		quads     int
	}{
		{"direction only", `[{"@value":"a","@language":"en","@direction":"ltr"},{"@value":"a","@language":"en","@direction":"rtl"}]`, false, 1},
		{"minus zero", `[0,-0]`, false, 1},
		{"json.Number", `[1,1,{"@value":2},{"@value":2}]`, true, 2},
		{"json.Number written two ways", `[1,1.0,1.5,15e-1]`, true, 2},
		{"json literal with json.Number", `[{"@value":{"a":1},"@type":"@json"},{"@value":{"a":1.0},"@type":"@json"}]`, true, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(`{"@id":"urn:s","http://ex.org/p":` + c.doc + `}`))
			if c.useNumber {
				dec.UseNumber()
			}
			var in interface{}
			if err := dec.Decode(&in); err != nil {
				t.Fatal(err)
			}
			opts := NewJsonLdOptions("")
			opts.Format = "application/n-quads"
			out, err := NewJsonLdProcessor().ToRDF(in, opts)
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(out.(string), "\n"); n != c.quads {
				t.Fatalf("got %d quads, want %d:\n%s", n, c.quads, out)
			}
		})
	}
}

// Adding a value to the node map costs the same however many values the
// property already has.
func TestNodeMapManyValuesIsLinear(t *testing.T) {
	vals := make([]string, 50000)
	for i := range vals {
		vals[i] = fmt.Sprintf("%q", fmt.Sprintf("v%d", i))
	}
	doc := `{"@id":"urn:s","http://ex.org/p":[` + strings.Join(vals, ",") + `]}`
	start := time.Now()
	toNQuads(t, doc)
	// Quadratic insertion took 46 to 158 s when measured; linear takes well under 1 s.
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("50,000 values took %v", d)
	}
}
