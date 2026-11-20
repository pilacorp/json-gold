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
