package ld

import (
	"encoding/json"
	"testing"
)

// JSON-LD 1.1 API §8.2.2 steps 10 and 11 and §8.6: a number with no
// fractional part below 10^21 is an xsd:integer written as plain digits, on
// every CPU architecture. Expected literals are JavaScript's toFixed(0) and
// the xsd:double form §8.6 gives.
func TestToRDFWritesIntegersBelow10To21(t *testing.T) {
	const integer = "^^<http://www.w3.org/2001/XMLSchema#integer>"
	const double = "^^<http://www.w3.org/2001/XMLSchema#double>"
	for text, want := range map[string]string{
		"42":                   `"42"` + integer,
		"-0":                   `"0"` + integer,
		"9223372036854775807":  `"9223372036854775808"` + integer,
		"9223372036854775808":  `"9223372036854775808"` + integer,
		"-9223372036854775809": `"-9223372036854775808"` + integer,
		"1e20":                 `"100000000000000000000"` + integer,
		"1e21":                 `"1.0E21"` + double,
		"1.5":                  `"1.5E0"` + double,
	} {
		var doc interface{}
		if err := json.Unmarshal([]byte(`{"@id":"urn:s","https://ex.org/p":`+text+`}`), &doc); err != nil {
			t.Fatal(err)
		}
		opts := NewJsonLdOptions("")
		opts.Format = "application/n-quads"
		got, err := NewJsonLdProcessor().ToRDF(doc, opts)
		if err != nil {
			t.Fatalf("%s: ToRDF: %v", text, err)
		}
		if w := "<urn:s> <https://ex.org/p> " + want + " .\n"; got != w {
			t.Fatalf("%s: ToRDF = %q, want %q", text, got, w)
		}
	}
}
