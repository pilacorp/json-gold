// Copyright 2015-2017 Piprate Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ld_test

import (
	"fmt"
	"testing"

	. "github.com/pilacorp/json-gold/ld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A space cannot appear in an IRI (RFC 3987 §2.2).
const invalidHTTPIRI = "http://example.com/bad p"

// validityDoc has n nodes that share valid predicates, an invalid predicate
// and a datatype, so every IRI is seen many times. Expansion rejects an
// invalid datatype outright, so none appears here.
func validityDoc(n int) map[string]interface{} {
	nodes := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		nodes = append(nodes, map[string]interface{}{
			"@id":                     fmt.Sprintf("http://example.com/n%d", i),
			"http://example.com/name": fmt.Sprintf("node %d", i),
			invalidHTTPIRI:            "dropped",
			"http://example.com/when": map[string]interface{}{"@value": "2020-01-01", "@type": "http://www.w3.org/2001/XMLSchema#date"},
			"http://example.com/link": map[string]interface{}{"@id": "http://example.com/target"},
		})
	}
	return map[string]interface{}{"@graph": nodes}
}

// ToRDF remembers IRI validity within a call; the statements it keeps must be
// exactly those Quad.Valid accepts.
func TestToRDFDropsInvalidIRIsConsistently(t *testing.T) {
	const n = 20
	ds, err := NewJsonLdProcessor().ToRDF(validityDoc(n), NewJsonLdOptions(""))
	require.NoError(t, err)
	quads := ds.(*RDFDataset).GetQuads("@default")

	// name, when and link survive for every node; the invalid predicate is
	// dropped every time.
	assert.Len(t, quads, 3*n)
	for _, q := range quads {
		assert.True(t, q.Valid(), "kept an invalid quad: %v", q)
	}
}

func BenchmarkToRDFRepeatedIRIs(b *testing.B) {
	doc := validityDoc(200)
	proc := NewJsonLdProcessor()
	opts := NewJsonLdOptions("")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := proc.ToRDF(doc, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// JSON-LD 1.1 API §8.1 allows only language tags that are well-formed per
// BCP 47 §2.2.9, and §8.2 drops a value whose @language is not. The expected
// results follow the RFC 5646 §2.1 ABNF.
func TestToRDFKeepsOnlyWellFormedLanguageTags(t *testing.T) {
	cases := map[string]bool{
		"":                 false,
		"a":                false, // primary language is 2 to 8 letters
		"abcdefghi":        false, // nine letters
		"en-":              false,
		"en--US":           false,
		"en US":            false,
		"en-a":             false, // a singleton needs a subtag after it
		"en-x":             false, // so does the private-use x
		"en-abcdefghi":     false, // a subtag is at most 8 characters
		"en":               true,
		"EN-us":            true,
		"zh-Hant-TW":       true,
		"es-419":           true,
		"sl-rozaj-biske":   true,
		"de-CH-1901":       true,
		"en-a-bbb-x-a-ccc": true,
		"x-whatever":       true,
		"i-klingon":        true,
		"zh-min-nan":       true,
	}
	for tag, keep := range cases {
		doc := map[string]interface{}{
			"@id":                     "http://example.com/s",
			"http://example.com/name": map[string]interface{}{"@value": "x", "@language": tag},
		}
		ds, err := NewJsonLdProcessor().ToRDF(doc, NewJsonLdOptions(""))
		require.NoError(t, err, tag)
		quads := ds.(*RDFDataset).GetQuads("@default")
		if keep {
			assert.Len(t, quads, 1, "tag %q", tag)
		} else {
			assert.Empty(t, quads, "tag %q", tag)
		}
	}
}
