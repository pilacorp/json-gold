package ld

import (
	"strings"
	"testing"
)

// Expected results follow the RFC 3987 §2.2 ABNF for IRI.
func TestWellFormedIRI(t *testing.T) {
	cases := map[string]bool{
		"http://example.com/a?b=c#d": true,
		"did:example:123#key-1":      true,
		"urn:uuid:3978":              true,
		"ex:":                        true,
		"http://-bad-/p":             true, // a reg-name may start with "-"
		"http://example.com/" + strings.Repeat("a", 3000): true,
		"http://bücher.example/straße":                    true,
		"http://[::1]:8080/a":                             true,
		"http://[v1.fe]/":                                 true,
		"http://user:pw@example.com:99999/":               true,
		"http://example.com/%41":                          true,
		"http://example.com/?":                           true, // iprivate is allowed in iquery
		"tag:a,b:c/d;e=f!$&'()*+":                         true,
		"":                                                false,
		"a/b":                                             false, // no scheme
		"1x:y":                                            false,
		"ex:a b":                                          false,
		"ex:o> <ex:x":                                     false,
		"ex:o><ex:x":                                      false,
		`ex:o"x`:                                          false,
		"ex:o{x}":                                         false,
		`ex:o\x`:                                          false,
		"ex:o^x":                                          false,
		"ex:o|x":                                          false,
		"ex:o`x":                                          false,
		"ex:o\nx":                                         false,
		"ex:%4":                                           false,
		"ex:%zz":                                          false,
		"http://example.com/":                            false, // iprivate is not allowed in a path
		"http://example.com/￾":                            false,
		"http://[::1/":                                    false,
		"http://[g::1]/":                                  false,
		"http://a:b@c:d/":                                 false, // port must be digits
		"http://exa mple.com/":                            false,
		"ex://a//b":                                       true,
		"ex:#a#b":                                         false,
		"ex:\xff":                                         false,
	}
	for iri, want := range cases {
		if got := wellFormedIRI(iri); got != want {
			t.Errorf("wellFormedIRI(%q) = %v, want %v", iri, got, want)
		}
	}
}
