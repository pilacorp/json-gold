package ld

import (
	"encoding/json"
	"strings"
	"testing"
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
