package ld

import (
	"encoding/json"
	"reflect"
	"testing"
)

// JSON-LD 1.1 API §4.2.2 step 16: a definition of the @type keyword maps to
// @type whatever entries it holds, not only when it has @container.
func TestTypeKeywordDefinitionMapsToType(t *testing.T) {
	for _, c := range []struct{ name, ctx, in, want string }{
		{
			"@protected only",
			`{"@type":{"@protected":true}}`,
			`[{"@type":["https://e/T"]}]`,
			`{"@type":"https://e/T"}`,
		},
		{
			"@protected with @vocab",
			`{"@vocab":"https://e/v/","@type":{"@protected":true}}`,
			`[{"https://e/v/@type":[{"@value":"x"}],"@type":["https://e/v/T"]}]`,
			`{"@type":"T","https://e/v/@type":"x"}`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var ctx, in, want interface{}
			for s, v := range map[string]*interface{}{c.ctx: &ctx, c.in: &in, c.want: &want} {
				if err := json.Unmarshal([]byte(s), v); err != nil {
					t.Fatal(err)
				}
			}
			out, err := NewJsonLdProcessor().Compact(in, ctx, NewJsonLdOptions(""))
			if err != nil {
				t.Fatal(err)
			}
			delete(out, "@context")
			if !reflect.DeepEqual(out, want) {
				got, _ := json.Marshal(out)
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

// §4.2.2 step 4 allows @type only as a map holding @container: @set,
// @protected or both, so a serialized context must keep that form and parse
// again.
func TestTypeKeywordDefinitionIsSerializedAsAMap(t *testing.T) {
	for _, c := range []struct{ name, def, want string }{
		{"@protected only", `{"@protected":true}`, `{"@protected":true}`},
		{"@protected false", `{"@protected":false}`, `{"@protected":false}`},
		{"@container only", `{"@container":"@set"}`, `{"@container":"@set"}`},
		{"both", `{"@container":"@set","@protected":true}`, `{"@container":"@set","@protected":true}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			var def, want interface{}
			if err := json.Unmarshal([]byte(c.def), &def); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(c.want), &want); err != nil {
				t.Fatal(err)
			}
			ctx, err := NewContext(nil, NewJsonLdOptions("")).Parse(map[string]interface{}{"@version": 1.1, "@type": def})
			if err != nil {
				t.Fatal(err)
			}
			out, err := ctx.Serialize()
			if err != nil {
				t.Fatal(err)
			}
			if got := out["@context"].(map[string]interface{})["@type"]; !reflect.DeepEqual(got, want) {
				t.Fatalf("@type = %v, want %s", got, c.want)
			}
			if _, err := NewContext(nil, NewJsonLdOptions("")).Parse(out["@context"]); err != nil {
				t.Fatalf("parse serialized context: %v", err)
			}
		})
	}
}
