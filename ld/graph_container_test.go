package ld

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A VCDM presentation defines verifiableCredential with @container @graph and
// a property-scoped context of null. The container belongs to the term, so
// the embedded credential's triples stay in a named graph.
func TestVerifiableCredentialGraphContainerWithNullScopedContext(t *testing.T) {
	const doc = `{
		"@context": {
			"@version": 1.1,
			"type": "@type",
			"VerifiablePresentation": {
				"@id": "https://www.w3.org/2018/credentials#VerifiablePresentation",
				"@context": {
					"verifiableCredential": {
						"@id": "https://www.w3.org/2018/credentials#verifiableCredential",
						"@type": "@id",
						"@container": "@graph",
						"@context": null
					}
				}
			}
		},
		"type": "VerifiablePresentation",
		"verifiableCredential": {
			"@context": {"@version": 1.1, "name": "https://schema.org/name"},
			"@id": "urn:uuid:3978",
			"name": "Alice"
		}
	}`
	var input interface{}
	if err := json.Unmarshal([]byte(doc), &input); err != nil {
		t.Fatal(err)
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	quads := out.(string)
	var sawProperty, sawName bool
	for _, line := range strings.Split(quads, "\n") {
		if strings.Contains(line, "verifiableCredential") {
			sawProperty = true
			if strings.Contains(line, "<urn:uuid:3978>") {
				t.Errorf("verifiableCredential points at the credential id, want the graph: %s", line)
			}
		}
		if strings.Contains(line, `"Alice"`) {
			sawName = true
			if !strings.Contains(line, "_:") {
				t.Errorf("name triple is in the default graph: %s", line)
			}
		}
	}
	if !sawProperty || !sawName {
		t.Fatalf("missing quads:\n%s", quads)
	}
}

// JSON-LD 1.1 API §5.1.2 step 13.8.3 wraps every value under a @graph
// container term in a graph object, so a string becomes
// {"@graph": [{"@value": "..."}]}. The graph keeps its link triple; the value
// has no subject in that graph and, like any free-floating value (§5.1.2
// step 19.1), produces no triple.
func TestGraphContainerStringValueIsDropped(t *testing.T) {
	const doc = `{
		"@context": {
			"@version": 1.1,
			"VerifiablePresentation": {
				"@id": "https://www.w3.org/2018/credentials#VerifiablePresentation",
				"@context": {
					"vc": {
						"@id": "https://www.w3.org/2018/credentials#verifiableCredential",
						"@type": "@id",
						"@container": "@graph",
						"@context": null
					}
				}
			},
			"type": "@type"
		},
		"type": "VerifiablePresentation",
		"vc": ["urn:uuid:abc"]
	}`
	const want = "_:b0 <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://www.w3.org/2018/credentials#VerifiablePresentation> .\n" +
		"_:b0 <https://www.w3.org/2018/credentials#verifiableCredential> _:b1 .\n"
	assertNQuads(t, doc, want)
}

// A list object under a @graph container term is wrapped the same way and
// has no subject in the graph either, so only the link triple remains.
func TestGraphContainerListValueIsDropped(t *testing.T) {
	const doc = `{
		"@context": {
			"@version": 1.1,
			"g": {"@id": "https://example.org/g", "@container": "@graph"}
		},
		"@id": "urn:ex:1",
		"g": {"@list": ["a"]}
	}`
	assertNQuads(t, doc, "<urn:ex:1> <https://example.org/g> _:b0 .\n")
}

// JSON-LD 1.1 API §5.1.2 step 13.4.5 expands a @graph value with @graph as
// active property, which drops a scalar or value object (steps 4.1 and 19),
// and ensures an array of maps. Nothing is left, so the graph is empty: it
// keeps its link triple and every API handles it alike.
func TestGraphKeywordWithOnlyScalarIsEmpty(t *testing.T) {
	for _, value := range []string{`"x"`, `{"@value":"x"}`, `{}`, `null`, `["x"]`} {
		doc := `{"@id":"urn:s","http://ex.org/p":{"@graph":` + value + `}}`
		t.Run(value, func(t *testing.T) {
			var in interface{}
			require.NoError(t, json.Unmarshal([]byte(doc), &in))
			p := NewJsonLdProcessor()
			expanded, err := p.Expand(in, NewJsonLdOptions(""))
			require.NoError(t, err)
			assert.Equal(t, []interface{}{map[string]interface{}{
				"@id":             "urn:s",
				"http://ex.org/p": []interface{}{map[string]interface{}{"@graph": []interface{}{}}},
			}}, expanded)
			_, err = p.Flatten(in, nil, NewJsonLdOptions(""))
			require.NoError(t, err)
			_, err = p.Frame(in, map[string]interface{}{}, NewJsonLdOptions(""))
			require.NoError(t, err)
			assertNQuads(t, doc, "<urn:s> <http://ex.org/p> _:b0 .\n")
		})
	}
}

// Framing 1.1 §4.1 takes the first @graph entry of a frame as the
// sub-frame, or an empty map when there is none, so an empty or dropped
// @graph frames like a wildcard.
func TestFrameWithEmptyGraphIsWildcard(t *testing.T) {
	var in interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"@id":"urn:s","http://ex.org/p":{"@graph":{"@id":"urn:a","http://ex.org/q":"x"}}}`), &in))
	var wildcard interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"http://ex.org/p":{"@graph":{}}}`), &wildcard))
	want, err := NewJsonLdProcessor().Frame(in, wildcard, NewJsonLdOptions(""))
	require.NoError(t, err)
	for _, f := range []string{`{"http://ex.org/p":{"@graph":[]}}`, `{"http://ex.org/p":{"@graph":["x"]}}`} {
		t.Run(f, func(t *testing.T) {
			var frame interface{}
			require.NoError(t, json.Unmarshal([]byte(f), &frame))
			got, err := NewJsonLdProcessor().Frame(in, frame, NewJsonLdOptions(""))
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func assertNQuads(t *testing.T, doc, want string) {
	t.Helper()
	var input interface{}
	if err := json.Unmarshal([]byte(doc), &input); err != nil {
		t.Fatal(err)
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	out, err := NewJsonLdProcessor().ToRDF(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	if out.(string) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
