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

// RDFC-1.0 Appendix A writes characters outside the XML 1.1 Char
// production as UCHAR; U+FFFE and U+FFFF are outside it, U+FFFD and
// U+10000 are inside it and stay as they are.
func TestCanonicalNQuadsEscapesNonCharacters(t *testing.T) {
	doc := map[string]interface{}{
		"@context": map[string]interface{}{"note": "https://example.org/note"},
		"@id":      "urn:ex:1",
		"note":     "a\uFFFEb\uFFFFc\uFFFDd\U00010000",
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := `"a\uFFFEb\uFFFFc` + "\uFFFDd\U00010000" + `"`
	if !strings.Contains(out.(string), want) {
		t.Fatalf("quads =\n%s\nwant a literal %s", out, want)
	}
}
