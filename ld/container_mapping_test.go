package ld

import (
	"encoding/json"
	"errors"
	"testing"
)

// JSON-LD 1.1 API §4.2.2 step 19: a container mapping that is not one of the
// keywords allowed there is an invalid container mapping error, whatever JSON
// type it has. For @type, step 4 allows only @container: @set.
func TestContainerMappingThatIsNotAStringIsAnError(t *testing.T) {
	for name, c := range map[string]struct {
		doc  string
		want ErrorCode
	}{
		"null":               {`{"@context":{"t":{"@id":"https://e.org/p","@container":null}},"t":"v"}`, InvalidContainerMapping},
		"number in an array": {`{"@context":{"t":{"@id":"https://e.org/p","@container":[1]}},"t":"v"}`, InvalidContainerMapping},
		"null in an array":   {`{"@context":{"t":{"@id":"https://e.org/p","@container":[null]}},"t":"v"}`, InvalidContainerMapping},
		"number":             {`{"@context":{"t":{"@id":"https://e.org/p","@container":1}},"t":"v"}`, InvalidContainerMapping},
		"null on @type":      {`{"@context":{"@version":1.1,"@type":{"@container":null}},"@type":"https://e.org/T"}`, KeywordRedefinition},
	} {
		t.Run(name, func(t *testing.T) {
			var doc interface{}
			if err := json.Unmarshal([]byte(c.doc), &doc); err != nil {
				t.Fatal(err)
			}
			_, err := NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != c.want {
				t.Fatalf("err = %v, want %s", err, c.want)
			}
		})
	}
}

// Step 19.1: an array must hold one of the allowed keywords or an allowed
// combination, so an empty one is an invalid container mapping too.
func TestEmptyContainerArrayIsAnError(t *testing.T) {
	ctx := map[string]interface{}{"t": map[string]interface{}{"@id": "https://e.org/p", "@container": []interface{}{}}}
	_, err := NewContext(nil, NewJsonLdOptions("")).Parse(ctx)
	var ldErr *JsonLdError
	if !errors.As(err, &ldErr) || ldErr.Code != InvalidContainerMapping {
		t.Fatalf("err = %v, want %s", err, InvalidContainerMapping)
	}
}
