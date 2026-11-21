package ld

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IRIs in the form of a keyword expand to null (JSON-LD 1.1 API §5.2.2
// step 2). The spec leaves the node map silent on a null @id; these pin the
// jsonld.js behaviour, so a Go and a JavaScript signer emit the same quads.
func TestKeywordLikeIRIs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		quads string
	}{
		{"top-level @id becomes a blank node",
			`{"@id":"@ignoreMe","http://ex.org/p":"x"}`,
			"_:b0 <http://ex.org/p> \"x\" .\n"},
		{"nested @id drops the value and its properties",
			`{"@id":"http://ex.org/s","http://ex.org/p":[{"@id":"@ignoreMe","http://ex.org/q":"v"},{"@id":"http://ex.org/o"}]}`,
			"<http://ex.org/s> <http://ex.org/p> <http://ex.org/o> .\n"},
		{"@type @id term value is dropped",
			`{"@context":{"p":{"@id":"http://ex.org/p","@type":"@id"}},"@id":"http://ex.org/s","p":"@ignoreMe","http://ex.org/r":"k"}`,
			"<http://ex.org/s> <http://ex.org/r> \"k\" .\n"},
		{"node @type is dropped",
			`{"@id":"http://ex.org/s","@type":["@ignoreMe","http://ex.org/T"]}`,
			"<http://ex.org/s> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://ex.org/T> .\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var in interface{}
			require.NoError(t, json.Unmarshal([]byte(tc.input), &in))
			p := NewJsonLdProcessor()
			opts := NewJsonLdOptions("")
			opts.Format = "application/n-quads"
			quads, err := p.ToRDF(in, opts)
			require.NoError(t, err)
			assert.Equal(t, tc.quads, quads)

			// Compaction keeps the null @id instead of panicking on it.
			_, err = p.Compact(in, map[string]interface{}{"@base": "http://example.org/"}, NewJsonLdOptions(""))
			require.NoError(t, err)
		})
	}
}

func TestKeywordLikeIRICompactsToNull(t *testing.T) {
	var in interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"@id":"http://ex.org/s","http://ex.org/p":{"@id":"@ignoreMe"}}`), &in))
	out, err := NewJsonLdProcessor().Compact(in, map[string]interface{}{}, NewJsonLdOptions(""))
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"@id": nil}, out["http://ex.org/p"])
}

// Compaction picks a term for a node reference whose @id is null. The
// expected outputs are those of jsonld.js 8.3.3.
func TestKeywordLikeIRICompactTermSelection(t *testing.T) {
	tests := []struct {
		name, ctx, want string
	}{
		{"@type @id term", `{"p":{"@id":"http://ex.org/p","@type":"@id"}}`, `null`},
		{"@type @vocab term", `{"p":{"@id":"http://ex.org/p","@type":"@vocab"}}`, `null`},
		{"@id container", `{"p":{"@id":"http://ex.org/p","@container":"@id"}}`, `{"@none":{}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var in, ctx, want interface{}
			require.NoError(t, json.Unmarshal([]byte(`{"@id":"http://ex.org/s","http://ex.org/p":{"@id":"@ignoreMe"}}`), &in))
			require.NoError(t, json.Unmarshal([]byte(tc.ctx), &ctx))
			require.NoError(t, json.Unmarshal([]byte(tc.want), &want))
			out, err := NewJsonLdProcessor().Compact(in, ctx, NewJsonLdOptions(""))
			require.NoError(t, err)
			assert.Equal(t, want, out["p"])
		})
	}
}

// Framing §4.1: a frame's @id and @type must be IRIs, and a keyword-like
// one has expanded to null.
func TestKeywordLikeIRIInFrame(t *testing.T) {
	var in interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"@id":"http://ex.org/s","@type":"http://ex.org/T"}`), &in))
	for _, f := range []string{`{"@id":"@x"}`, `{"@id":["@x","http://ex.org/s"]}`, `{"@type":"@x"}`, `{"@type":["@x"]}`,
		// Sub-frames follow the same rules, even when no node reaches them.
		`{"http://ex.org/p":{"@id":"@x"}}`, `{"http://ex.org/p":{"@type":"@x"}}`,
		`{"@reverse":{"http://ex.org/p":{"@type":"@x"}}}`} {
		t.Run(f, func(t *testing.T) {
			var frame interface{}
			require.NoError(t, json.Unmarshal([]byte(f), &frame))
			_, err := NewJsonLdProcessor().Frame(in, frame, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			require.ErrorAs(t, err, &ldErr)
			assert.Equal(t, InvalidFrame, ldErr.Code)
		})
	}
}

// In safe mode nothing is turned into null or left out without an error.
func TestSafeModeRejectsLostValues(t *testing.T) {
	tests := []struct {
		name, input string
		code        ErrorCode
	}{
		{"keyword-like node @id", `{"@id":"urn:s","http://ex.org/p":{"@id":"@foo","http://ex.org/q":"x"}}`, InvalidIDValue},
		{"keyword-like @type @id value", `{"@context":{"p":{"@id":"http://ex.org/p","@type":"@id"}},"@id":"urn:s","p":"@foo"}`, InvalidIDValue},
		{"keyword-like @type entry", `{"@id":"urn:s","@type":["@foo","http://ex.org/T"]}`, InvalidTypeValue},
		{"keyword-like @id in @set", `{"@id":"urn:s","http://ex.org/p":{"@set":[{"@id":"@foo"}]}}`, InvalidIDValue},
		{"keyword-like @id in @list", `{"@id":"urn:s","http://ex.org/p":{"@list":[{"@id":"@foo"}]}}`, InvalidIDValue},
		{"keyword-like @id map key", `{"@context":{"p":{"@id":"http://ex.org/p","@container":"@id"}},"@id":"urn:s","p":{"@foo":{"http://ex.org/q":"x"}}}`, InvalidIDValue},
		{"keyword-like @type map key", `{"@context":{"p":{"@id":"http://ex.org/p","@container":"@type"}},"@id":"urn:s","p":{"@foo":{"http://ex.org/q":"x"}}}`, InvalidTypeValue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var in interface{}
			require.NoError(t, json.Unmarshal([]byte(tc.input), &in))
			opts := NewJsonLdOptions("")
			opts.SafeMode = true
			opts.Format = "application/n-quads"
			opts.Algorithm = AlgorithmURDNA2015
			_, err := NewJsonLdProcessor().Normalize(in, opts)
			var ldErr *JsonLdError
			require.ErrorAs(t, err, &ldErr)
			assert.Equal(t, tc.code, ldErr.Code)
		})
	}
}

// A list item or @reverse value whose @id is keyword-like becomes a blank
// node, so it still counts in the canonical form. Expected N-Quads are the
// URDNA2015 output of jsonld.js 8.3.3.
func TestKeywordLikeIRIInListAndReverse(t *testing.T) {
	tests := []struct{ input, quads string }{
		{`{"@id":"http://ex.org/s","http://ex.org/l":{"@list":[{"@id":"@foo","http://ex.org/q":"x"},{"@id":"http://ex.org/o"}]}}`,
			"<http://ex.org/s> <http://ex.org/l> _:c14n1 .\n" +
				"_:c14n0 <http://www.w3.org/1999/02/22-rdf-syntax-ns#first> <http://ex.org/o> .\n" +
				"_:c14n0 <http://www.w3.org/1999/02/22-rdf-syntax-ns#rest> <http://www.w3.org/1999/02/22-rdf-syntax-ns#nil> .\n" +
				"_:c14n1 <http://www.w3.org/1999/02/22-rdf-syntax-ns#first> _:c14n2 .\n" +
				"_:c14n1 <http://www.w3.org/1999/02/22-rdf-syntax-ns#rest> _:c14n0 .\n" +
				"_:c14n2 <http://ex.org/q> \"x\" .\n"},
		{`{"@id":"http://ex.org/s","@reverse":{"http://ex.org/r":{"@id":"@foo","http://ex.org/q":"x"}}}`,
			"_:c14n0 <http://ex.org/q> \"x\" .\n_:c14n0 <http://ex.org/r> <http://ex.org/s> .\n"},
	}
	for _, tc := range tests {
		var in interface{}
		require.NoError(t, json.Unmarshal([]byte(tc.input), &in))
		opts := NewJsonLdOptions("")
		opts.Format = "application/n-quads"
		opts.Algorithm = AlgorithmURDNA2015
		got, err := NewJsonLdProcessor().Normalize(in, opts)
		require.NoError(t, err)
		assert.Equal(t, tc.quads, got)
	}
}

// An IRI whose relative form is a keyword compacts with a "./" prefix, or
// it would expand back as the keyword (jsonld.js 8.3.3 writes "./@type").
func TestRelativeIRINamedLikeKeywordCompacts(t *testing.T) {
	var in interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"@id":"http://ex.org/s","http://ex.org/p":{"@id":"http://ex.org/@type"}}`), &in))
	out, err := NewJsonLdProcessor().Compact(in, map[string]interface{}{"@base": "http://ex.org/"}, NewJsonLdOptions(""))
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"@id": "./@type"}, out["http://ex.org/p"])
}

// §5.1.2 step 13.4.4.5: a second key expanding to @type adds its types
// (jsonld.js 8.3.3 keeps both).
func TestSecondTypeKeyAddsTypes(t *testing.T) {
	var in interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"@context":{"type":"@type"},"@id":"http://ex.org/s","@type":"http://ex.org/Admin","type":["http://ex.org/B"]}`), &in))
	out, err := NewJsonLdProcessor().Expand(in, NewJsonLdOptions(""))
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.ElementsMatch(t, []interface{}{"http://ex.org/Admin", "http://ex.org/B"}, out[0].(map[string]interface{})["@type"])
}
