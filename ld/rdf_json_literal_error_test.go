package ld

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

// A @json value that has no JSON form (Inf) or no IEEE 754 form for
// RFC 8785 (1e400) cannot be written as rdf:JSON, so ToRDF refuses it
// with "invalid JSON literal".
func TestJsonLiteralWithoutCanonicalFormIsRefused(t *testing.T) {
	for name, v := range map[string]interface{}{
		"inf":   math.Inf(1),
		"1e400": json.Number("1e400"),
	} {
		t.Run(name, func(t *testing.T) {
			doc := []interface{}{map[string]interface{}{
				"@id": "urn:s:1",
				"urn:p": []interface{}{map[string]interface{}{
					"@value": v, "@type": "@json",
				}},
			}}
			_, err := NewJsonLdProcessor().ToRDF(doc, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != InvalidJsonLiteral {
				t.Fatalf("got %v, want %q", err, InvalidJsonLiteral)
			}
		})
	}
}
