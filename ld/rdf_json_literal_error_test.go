package ld

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

// A @json value with no IEEE 754 form for RFC 8785 (1e400) cannot be written
// as rdf:JSON, so ToRDF refuses it with "invalid JSON literal". Inf is not
// JSON at all, so the processor refuses it as invalid input before that.
func TestJsonLiteralWithoutCanonicalFormIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		value interface{}
		code  ErrorCode
	}{
		"inf":   {math.Inf(1), InvalidInput},
		"1e400": {json.Number("1e400"), InvalidJsonLiteral},
	} {
		t.Run(name, func(t *testing.T) {
			doc := []interface{}{map[string]interface{}{
				"@id": "urn:s:1",
				"urn:p": []interface{}{map[string]interface{}{
					"@value": tc.value, "@type": "@json",
				}},
			}}
			_, err := NewJsonLdProcessor().ToRDF(doc, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != tc.code {
				t.Fatalf("got %v, want %q", err, tc.code)
			}
		})
	}
}
