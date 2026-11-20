package ld

import (
	"strings"
	"testing"
)

func TestCanonicalNQuadsEscapesControlCharacters(t *testing.T) {
	doc := map[string]interface{}{
		"@context": map[string]interface{}{"note": "https://example.org/note"},
		"@id":      "urn:ex:1",
		"note":     "a\tb\bc\fd\x01e",
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	want := `"a\tb\bc\fd\u0001e"`
	if !strings.Contains(quads, want) {
		t.Fatalf("quads =\n%s\nwant a literal %s", quads, want)
	}
	doc["note"] = "say \"hi\"\u007f ✓"
	out, err = NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads = out.(string)
	want = `"say \"hi\"\u007F ✓"`
	if !strings.Contains(quads, want) {
		t.Fatalf("quads =\n%s\nwant a literal %s", quads, want)
	}
}
