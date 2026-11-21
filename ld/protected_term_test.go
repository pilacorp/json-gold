package ld

import (
	"encoding/json"
	"errors"
	"testing"
)

// A term definition with a keyword-like @id or @reverse is ignored and the
// term keeps its previous definition, so a protected term cannot be removed
// that way and then redefined. jsonld.js 8.3.3 gives the same errors.
func TestProtectedTermSurvivesIgnoredRedefinition(t *testing.T) {
	for _, ctx := range []string{
		`[{"@protected":true,"t":"http://ex.org/t"},{"t":{"@reverse":"@x"}},{"t":"http://evil.org/t"}]`,
		`[{"@protected":true,"t":"http://ex.org/t"},{"t":{"@id":"@x"}},{"t":"http://evil.org/t"}]`,
	} {
		t.Run(ctx, func(t *testing.T) {
			var c interface{}
			if err := json.Unmarshal([]byte(ctx), &c); err != nil {
				t.Fatal(err)
			}
			_, err := NewJsonLdProcessor().Expand(map[string]interface{}{"@context": c, "t": "v"}, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != ProtectedTermRedefinition {
				t.Fatalf("got %v, want %s", err, ProtectedTermRedefinition)
			}
		})
	}
}
