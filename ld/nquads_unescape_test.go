package ld

import "testing"

// A backslash written as \\ before n, r, t, a quote, or another backslash
// is those two characters. Decoding the escapes one after another turns
// \\n into a newline.
func TestParseNQuadsKeepsBackslashBeforeEscapeLetter(t *testing.T) {
	line := "<urn:s:1> <https://ex.org/n> \"a\\\\nb\\\\rc\\\\td\\\\\\\"e\\\\\\\\f\" .\n"
	ds, err := ParseNQuads(line)
	if err != nil {
		t.Fatal(err)
	}
	got := ds.Graphs["@default"][0].Object.(Literal).Value
	want := "a\\nb\\rc\\td\\\"e\\\\f"
	if got != want {
		t.Fatalf("parsed %q, want %q", got, want)
	}

	newline, err := ParseNQuads("<urn:s:1> <https://ex.org/n> \"a\\nb\" .\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := newline.Graphs["@default"][0].Object.(Literal).Value; got != "a\nb" {
		t.Fatalf("real newline escape parsed %q", got)
	}
}
