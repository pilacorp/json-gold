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

// When a string value sits under a term with @container: @graph, expansion
// wraps it as {"@graph": [{"@value": "..."}]}. GenerateNodeMap must return an
// error for the value object instead of panicking on a nil subject node.
func TestGraphContainerStringValueReturnsError(t *testing.T) {
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
	var input interface{}
	if err := json.Unmarshal([]byte(doc), &input); err != nil {
		t.Fatal(err)
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	_, err := NewJsonLdProcessor().ToRDF(input, opts)
	if err == nil {
		t.Fatal("expected an error for a value object landing in a graph, got nil")
	}
	jErr, ok := err.(*JsonLdError)
	if !ok || jErr.Code != InvalidValueObject {
		t.Fatalf("expected InvalidValueObject error, got %v", err)
	}
}

// A list object under a term with @container: @graph is wrapped as
// {"@graph": [{"@list": [...]}]}, so it lands in the graph with no subject.
// GenerateNodeMap must return an error for it too instead of panicking.
func TestGraphContainerListValueReturnsError(t *testing.T) {
	const doc = `{
		"@context": {
			"@version": 1.1,
			"g": {"@id": "https://example.org/g", "@container": "@graph"}
		},
		"@id": "urn:ex:1",
		"g": {"@list": ["a"]}
	}`
	var input interface{}
	if err := json.Unmarshal([]byte(doc), &input); err != nil {
		t.Fatal(err)
	}
	opts := NewJsonLdOptions("")
	opts.Format = "application/n-quads"
	_, err := NewJsonLdProcessor().ToRDF(input, opts)
	if err == nil {
		t.Fatal("expected an error for a list object landing in a graph, got nil")
	}
}
