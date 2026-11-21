package ld

import (
	"errors"
	"reflect"
	"testing"
)

// The terms of a local context form a JSON object, so the result must not
// depend on which term is defined first. Go maps change their order between
// runs, so each case runs many times.
func TestTermDefinitionDoesNotDependOnOrder(t *testing.T) {
	const runs = 100
	expand := func(context map[string]interface{}) ([]interface{}, error) {
		doc := map[string]interface{}{"@context": context, "https://e.org/q": "v"}
		return NewJsonLdProcessor().Expand(doc, NewJsonLdOptions(""))
	}

	t.Run("dependency of a term's @type on the term itself", func(t *testing.T) {
		// b's @type needs d and d needs b as its prefix. §4.2.2 marks b
		// defined only once b is complete, so both orders meet the cycle.
		context := map[string]interface{}{
			"b": map[string]interface{}{"@id": "https://e.org/b", "@type": "d"},
			"d": "b:x",
		}
		for i := 0; i < runs; i++ {
			_, err := expand(context)
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) || ldErr.Code != CyclicIRIMapping {
				t.Fatalf("run %d: err = %v, want %s", i, err, CyclicIRIMapping)
			}
		}
	})

	t.Run("protected term redefined in a property-scoped context", func(t *testing.T) {
		// A property-scoped context may override protected terms. a needs b
		// as its prefix, so b may be defined through a first; it must get
		// the same override as when the loop reaches it.
		context := map[string]interface{}{
			"b": map[string]interface{}{"@id": nil, "@protected": true},
			"p": map[string]interface{}{"@id": "https://e.org/p", "@context": map[string]interface{}{
				"a": "b:x",
				"b": "https://e.org/b/",
			}},
		}
		want := []interface{}{map[string]interface{}{"https://e.org/q": []interface{}{map[string]interface{}{"@value": "v"}}}}
		for i := 0; i < runs; i++ {
			got, err := expand(context)
			if err != nil {
				t.Fatalf("run %d: %v", i, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("run %d: got %#v", i, got)
			}
		}
	})
	t.Run("context with several errors", func(t *testing.T) {
		// Processing stops at the first error, so which one is reported
		// depends on the order; it must be the same on every run.
		context := map[string]interface{}{
			"a": map[string]interface{}{"@id": true},
			"b": map[string]interface{}{"@id": "https://e.org/b", "@type": true},
			"c": map[string]interface{}{"@id": "https://e.org/c", "@container": true},
		}
		var first ErrorCode
		for i := 0; i < runs; i++ {
			_, err := expand(context)
			var ldErr *JsonLdError
			if !errors.As(err, &ldErr) {
				t.Fatalf("run %d: err = %v, want a JsonLdError", i, err)
			}
			if i == 0 {
				first = ldErr.Code
			} else if ldErr.Code != first {
				t.Fatalf("run %d: %s, run 0: %s", i, ldErr.Code, first)
			}
		}
	})
	t.Run("IRI-like term whose @type is the term itself", func(t *testing.T) {
		// The check that an IRI-like term expands to its own @id must not
		// leave the term undefined while its @type is processed, or the
		// definition recurses without end.
		context := map[string]interface{}{
			"ex":   "https://e.org/",
			"ex:x": map[string]interface{}{"@id": "https://e.org/x", "@type": "ex:x"},
		}
		_, err := expand(context)
		var ldErr *JsonLdError
		if !errors.As(err, &ldErr) || ldErr.Code != CyclicIRIMapping {
			t.Fatalf("err = %v, want %s", err, CyclicIRIMapping)
		}
	})
}
