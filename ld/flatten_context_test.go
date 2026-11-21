package ld

import (
	"reflect"
	"testing"
)

// JSON-LD 1.1 API §9.1 flatten() step 6 compacts with compact(), whose last
// step sets @context to the context provided. A context rebuilt from the
// parsed one loses entries such as @prefix, and the output then expands to
// other IRIs.
func TestFlattenOutputExpandsToTheSameData(t *testing.T) {
	ctx := map[string]interface{}{"ex": map[string]interface{}{"@id": "https://e.org/x", "@prefix": true}}
	in := map[string]interface{}{"@id": "https://e.org/xs", "https://e.org/xp": "v"}
	out, err := NewJsonLdProcessor().Flatten(in, map[string]interface{}{"@context": ctx}, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	if got := out.(map[string]interface{})["@context"]; !reflect.DeepEqual(got, ctx) {
		t.Fatalf("@context = %v, want the context provided %v", got, ctx)
	}
	got, err := NewJsonLdProcessor().Expand(out, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{map[string]interface{}{
		"@id":              "https://e.org/xs",
		"https://e.org/xp": []interface{}{map[string]interface{}{"@value": "v"}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded output = %v, want %v", got, want)
	}
}

// The output owns its @context: editing it must not change the context the
// caller passed in and may pass again, as with Compact.
func TestFlattenOutputContextIsACopy(t *testing.T) {
	ctx := map[string]interface{}{"ex": "https://e.org/"}
	in := map[string]interface{}{"@id": "https://e.org/s", "https://e.org/p": "v"}
	out, err := NewJsonLdProcessor().Flatten(in, ctx, NewJsonLdOptions(""))
	if err != nil {
		t.Fatal(err)
	}
	out.(map[string]interface{})["@context"].(map[string]interface{})["ex"] = "https://other.org/"
	if want := map[string]interface{}{"ex": "https://e.org/"}; !reflect.DeepEqual(ctx, want) {
		t.Fatalf("context passed in = %v, want %v", ctx, want)
	}
}
