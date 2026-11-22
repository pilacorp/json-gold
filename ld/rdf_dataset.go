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

package ld

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// Quad represents an RDF quad.
type Quad struct {
	Subject   Node
	Predicate Node
	Object    Node
	Graph     Node
}

// NewQuad creates a new instance of Quad.
func NewQuad(subject Node, predicate Node, object Node, graph string) *Quad {
	q := &Quad{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
	}

	if graph != "" && graph != "@default" {
		// TODO: i'm not yet sure if this should be added or if the
		// graph should only be represented by the keys in the dataset
		if strings.HasPrefix(graph, "_:") {
			q.Graph = NewBlankNode(graph)
		} else {
			q.Graph = NewIRI(graph)
		}
	}
	return q
}

// Equal returns true if this quad is equal to the given quad.
func (q *Quad) Equal(o *Quad) bool {
	if o == nil {
		return false
	}

	if (q.Graph != nil && !q.Graph.Equal(o.Graph)) || (q.Graph == nil && o.Graph != nil) {
		return false
	}

	return q.Subject.Equal(o.Subject) && q.Predicate.Equal(o.Predicate) && q.Object.Equal(o.Object)
}

func (q *Quad) Valid() bool {
	if q.Subject != nil {
		if InvalidNode(q.Subject) {
			return false
		}
	}

	if q.Predicate != nil {
		if InvalidNode(q.Predicate) {
			return false
		}
	}

	if q.Object != nil {
		if InvalidNode(q.Object) {
			return false
		}
	}

	if q.Graph != nil {
		if InvalidNode(q.Graph) {
			return false
		}
	}

	return true
}

// RDFDataset is an internal representation of an RDF dataset.
type RDFDataset struct {
	Graphs map[string][]*Quad

	context map[string]string

	// indicates that this dataset has had no duplicates (guaranteed by the serializer) at the time it's been parsed
	parsedWithoutDuplicates bool
}

// RDFSerializer can serialize and de-serialize RDFDatasets.
type RDFSerializer interface {
	// Parse the input into the internal RDF Dataset format.
	// The format is a map with the following structure:
	// {
	// 	   GRAPH_1: [ TRIPLE_1, TRIPLE_2, ..., TRIPLE_N ],
	//     GRAPH_2: [ TRIPLE_1, TRIPLE_2, ..., TRIPLE_N ],
	//     ...
	//     GRAPH_N: [ TRIPLE_1, TRIPLE_2, ..., TRIPLE_N ]
	// }
	//
	// GRAPH: Must be the graph name/IRI. If no graph is present for a triple,
	// add it to the "@default" graph TRIPLE: Must be a map with the following
	// structure:
	// {
	//     "subject" : SUBJECT,
	//     "predicate" : PREDICATE,
	//     "object" : OBJECT
	// }
	//
	// Each of the values in the triple map must also be a map with the
	// following key-value pairs:
	//
	// "value": The value of the node.
	// "subject" can be an IRI or blank node id.
	// "predicate" should only ever be an IRI
	// "object" can be and IRI or blank node id, or a literal value (represented
	//     as a string)
	// "type": "IRI" if the value is an IRI or "blank node" if the
	// value is a blank node. "object" can also be "literal" in the case of
	// literals. The value of "object" can also contain the following optional
	// key-value pairs:
	//
	// "language" : the language value of a string literal
	// "datatype" : the datatype of the literal. (if not set will default to XSD:string,
	//     if set to null, null will be used).
	//
	Parse(input interface{}) (*RDFDataset, error)

	// Serialize an RDFDataset
	Serialize(dataset *RDFDataset) (interface{}, error)
}

// RDFSerializerTo can serialize RDFDatasets into io.Writer.
type RDFSerializerTo interface {
	SerializeTo(w io.Writer, dataset *RDFDataset) error
}

// NewRDFDataset creates a new instance of RDFDataset.
func NewRDFDataset() *RDFDataset {
	ds := &RDFDataset{
		context: make(map[string]string),
	}

	ds.Graphs = make(map[string][]*Quad)
	ds.Graphs["@default"] = make([]*Quad, 0)

	return ds
}

// SetNamespace
func (ds *RDFDataset) SetNamespace(ns string, prefix string) {
	ds.context[ns] = prefix
}

// GetNamespace
func (ds *RDFDataset) GetNamespace(ns string) string {
	return ds.context[ns]
}

// ClearNamespaces clears all the namespaces in this dataset
func (ds *RDFDataset) ClearNamespaces() {
	ds.context = make(map[string]string)
}

// GetNamespaces
func (ds *RDFDataset) GetNamespaces() map[string]string {
	return ds.context
}

// GetContext returns a valid context containing any namespaces set.
func (ds *RDFDataset) GetContext() map[string]interface{} {
	rval := make(map[string]interface{})
	for k, v := range ds.context {
		if k == "" {
			// replace "" with "@vocab"
			rval["@vocab"] = v
		} else {
			rval[k] = v
		}
	}
	return rval
}

// ParseContext parses a context object and sets any namespaces found within it.
// Relative context IRIs resolve against the base option.
func (ds *RDFDataset) ParseContext(contextLike interface{}, opts *JsonLdOptions) error {
	if opts == nil {
		opts = NewJsonLdOptions("")
	}
	return ds.parseContext(contextLike, opts.Base, opts)
}

// parseContext is ParseContext with relative context IRIs resolved against
// baseURL, so a context read from a remote document resolves as it did when
// the document was expanded.
func (ds *RDFDataset) parseContext(contextLike interface{}, baseURL string, opts *JsonLdOptions) error {
	// Context will do our recursive parsing and initial IRI resolution
	context, err := NewContext(nil, opts).parse(contextLike, baseURL, nil, false, true, false, false)
	if err != nil {
		return err
	}
	// And then leak to us the potential 'prefixes'
	prefixes := context.GetPrefixes(true)

	for key, val := range prefixes {
		if key == "@vocab" {
			ds.SetNamespace("", val)
		} else if !IsKeyword(key) {
			ds.SetNamespace(key, val)
			// TODO: should we make sure val is a valid URI prefix (i.e. it
			// ends with /# or ?)
			// or is it ok that full URIs for terms are used?
		}
	}
	return nil
}

var first = NewIRI(RDFFirst)
var rest = NewIRI(RDFRest)
var nilIRI = NewIRI(RDFNil)

// GraphToRDF creates an array of RDF triples for the given graph.
func (ds *RDFDataset) GraphToRDF(graphName string, graph map[string]interface{}, issuer *IdentifierIssuer,
	produceGeneralizedRdf bool) error {
	return ds.graphToRDF(graphName, graph, issuer, produceGeneralizedRdf, false)
}

// graphToRDF is GraphToRDF. With safeMode, a statement left out because an
// IRI or language tag is not well-formed (§8.1) is an error instead, so the
// caller does not sign or verify a dataset that silently lost data.
func (ds *RDFDataset) graphToRDF(graphName string, graph map[string]interface{}, issuer *IdentifierIssuer,
	produceGeneralizedRdf, safeMode bool) error {
	// 4.2)
	triples := make([]*Quad, 0)
	// 4.3)
	for _, id := range GetKeys(graph) {
		if IsRelativeIri(id) {
			if safeMode {
				return relativeIRIError("subject", id)
			}
			continue
		}

		node := graph[id].(map[string]interface{})
		for _, property := range GetOrderedKeys(node) {
			var values []interface{}
			// 4.3.2.1)
			if property == "@type" {
				values = node["@type"].([]interface{})
				property = RDFType
			} else if IsKeyword(property) {
				// 4.3.2.2)
				continue
			} else if strings.HasPrefix(property, "_:") && !produceGeneralizedRdf {
				// 4.3.2.3)
				continue
			} else if IsRelativeIri(property) {
				// 4.3.2.4)
				if safeMode {
					return relativeIRIError("predicate", property)
				}
				continue
			} else {
				values = node[property].([]interface{})
			}

			var subject Node
			if strings.Index(id, "_:") == 0 {
				// NOTE: don't rename, just set it as a blank node
				subject = NewBlankNode(id)
			} else {
				subject = NewIRI(id)
			}

			// RDF predicates
			var predicate Node
			if strings.HasPrefix(property, "_:") {
				predicate = NewBlankNode(property)
			} else {
				predicate = NewIRI(property)
			}

			for _, item := range values {
				var object Node
				var err error
				object, triples, err = objectToRDF(item, issuer, graphName, triples, safeMode)
				if err != nil {
					return err
				}
				if object != nil {
					triples = append(triples, NewQuad(subject, predicate, object, graphName))
				}
			}
		}
	}

	// drop invalid statements (other than IRIs)
	//
	// A graph repeats the same IRIs (predicates, types, datatypes) across many
	// statements, and validating an IRI runs the IsURL regular expression, so
	// results are remembered for the duration of this call.
	v := make(iriValidity)
	sanitisedTriples := make([]*Quad, 0, len(triples))
	for _, t := range triples {
		if v.validQuad(t) {
			sanitisedTriples = append(sanitisedTriples, t)
		} else if safeMode {
			return NewJsonLdError(InvalidRdfTerm, fmt.Sprintf(
				"statement about %q with predicate %q has an IRI or language tag that is not well-formed",
				t.Subject.GetValue(), t.Predicate.GetValue()))
		}
	}
	ds.Graphs[graphName] = sanitisedTriples
	return nil
}

// relativeIRIError reports a term that §8.1 and §8.2 leave out of the dataset
// because it is a relative IRI (no base IRI resolved it).
func relativeIRIError(position, iri string) error {
	return NewJsonLdError(InvalidRdfTerm, fmt.Sprintf("%s %q is a relative IRI and has no RDF form", position, iri))
}

// GetQuads returns a list of quads for the given graph
func (ds *RDFDataset) GetQuads(graphName string) []*Quad {
	return ds.Graphs[graphName]
}

var canonicalDoubleRegEx = regexp.MustCompile(`(\d)0*E\+?(-)?0*(\d)`)

// GetCanonicalDouble returns a canonical string representation of a float64 number.
func GetCanonicalDouble(v float64) string {
	return canonicalDoubleRegEx.ReplaceAllString(fmt.Sprintf("%1.15E", v), "${1}E${2}${3}")
}

// wellFormedLanguageTag matches Language-Tag in the RFC 5646 §2.1 ABNF, which
// BCP 47 §2.2.9 makes the test for a well-formed tag.
var wellFormedLanguageTag = regexp.MustCompile(`(?i)^(?:` +
	// langtag: language, script, region, variants, extensions, private use
	`(?:[a-z]{2,3}(?:-[a-z]{3}){0,3}|[a-z]{4,8})` +
	`(?:-[a-z]{4})?` +
	`(?:-(?:[a-z]{2}|[0-9]{3}))?` +
	`(?:-(?:[a-z0-9]{5,8}|[0-9][a-z0-9]{3}))*` +
	`(?:-[0-9a-wyz](?:-[a-z0-9]{2,8})+)*` +
	`(?:-x(?:-[a-z0-9]{1,8})+)?` +
	// privateuse
	`|x(?:-[a-z0-9]{1,8})+` +
	// grandfathered: irregular, then regular
	`|en-GB-oed|i-ami|i-bnn|i-default|i-enochian|i-hak|i-klingon|i-lux|i-mingo` +
	`|i-navajo|i-pwn|i-tao|i-tay|i-tsu|sgn-BE-FR|sgn-BE-NL|sgn-CH-DE` +
	`|art-lojban|cel-gaulish|no-bok|no-nyn|zh-guoyu|zh-hakka|zh-min|zh-min-nan|zh-xiang` +
	`)$`)

// invalidLiteral reports whether l breaks JSON-LD 1.1 API §8.1: a language
// tag that is not well-formed, including the empty tag of a language-tagged
// string, or a datatype IRI that validIRI rejects.
func invalidLiteral(l Literal, validIRI func(string) bool) bool {
	if (l.Language != "" || l.Datatype == RDFLangString) && !wellFormedLanguageTag.MatchString(l.Language) {
		return true
	}
	return l.Datatype != "" && !validIRI(l.Datatype)
}

func InvalidNode(node Node) bool {

	switch v := node.(type) {
	case IRI:
		if !validIRI(v.Value) {
			return true
		}
	case Literal:
		return invalidLiteral(v, validIRI)
	}

	return false
}

// iriValidity remembers the result of validIRI per IRI. It is local to one
// GraphToRDF call, so it needs no locking and is bounded by the graph's size.
type iriValidity map[string]bool

func (v iriValidity) validIRI(val string) bool {
	if ok, seen := v[val]; seen {
		return ok
	}
	ok := validIRI(val)
	v[val] = ok
	return ok
}

// invalidNode is InvalidNode with IRI results taken from v.
func (v iriValidity) invalidNode(node Node) bool {
	switch n := node.(type) {
	case IRI:
		return !v.validIRI(n.Value)
	case Literal:
		return invalidLiteral(n, v.validIRI)
	}
	return false
}

// validQuad is Quad.Valid with IRI results taken from v.
func (v iriValidity) validQuad(q *Quad) bool {
	return (q.Subject == nil || !v.invalidNode(q.Subject)) &&
		(q.Predicate == nil || !v.invalidNode(q.Predicate)) &&
		(q.Object == nil || !v.invalidNode(q.Object)) &&
		(q.Graph == nil || !v.invalidNode(q.Graph))
}

// validIRI is the IRI test of JSON-LD 1.1 API §8.1: a quad with an IRI that
// is not well-formed per RFC 3987 is left out of the dataset.
func validIRI(val string) bool {
	return wellFormedIRI(val)
}

/*
===========
The URL validation logic below was borrowed from github.com/asaskevich/govalidator package.
The original code is distributed under MIT license. Copyright (c) 2014 Alex Saskevich
===========
*/
var (
	IP           = `(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`
	URLSchema    = `((ftp|tcp|udp|wss?|https?):\/\/)`
	URLUsername  = `(\S+(:\S*)?@)`
	URLPath      = `((\/|\?|#)[^\s]*)`
	URLPort      = `(:(\d{1,5}))`
	URLIP        = `([1-9]\d?|1\d\d|2[01]\d|22[0-3])(\.(1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.([0-9]\d?|1\d\d|2[0-4]\d|25[0-4]))`
	URLSubdomain = `((www\.)|([a-zA-Z0-9]+([-_\.]?[a-zA-Z0-9])*[a-zA-Z0-9]\.[a-zA-Z0-9]+))`
	URL          = `^` + URLSchema + `?` + URLUsername + `?` + `((` + URLIP + `|(\[` + IP + `\])|(([a-zA-Z0-9]([a-zA-Z0-9-_]+)?[a-zA-Z0-9]([-\.][a-zA-Z0-9]+)*)|(` + URLSubdomain + `?))?(([a-zA-Z\x{00a1}-\x{ffff}0-9]+-?-?)*[a-zA-Z\x{00a1}-\x{ffff}0-9]+)(?:\.([a-zA-Z\x{00a1}-\x{ffff}]{1,}))?))\.?` + URLPort + `?` + URLPath + `?$`
	rxURL        *regexp.Regexp
	onlyOnce     sync.Once
)

const maxURLRuneCount = 2083
const minURLRuneCount = 3

// IsURL check if the string is an URL.
func IsURL(str string) bool {
	onlyOnce.Do(func() {
		rxURL = regexp.MustCompile(URL)
	})
	if str == "" || utf8.RuneCountInString(str) >= maxURLRuneCount || len(str) <= minURLRuneCount || strings.HasPrefix(str, ".") {
		return false
	}
	strTemp := str
	if strings.Contains(str, ":") && !strings.Contains(str, "://") {
		// support no indicated urlscheme but with colon for port number
		// http:// is appended so url.Parse will succeed, strTemp used so it does not impact rxURL.MatchString
		strTemp = "http://" + str
	}
	u, err := url.Parse(strTemp)
	if err != nil {
		return false
	}
	if strings.HasPrefix(u.Host, ".") {
		return false
	}
	if u.Host == "" && (u.Path != "" && !strings.Contains(u.Path, ".")) {
		return false
	}
	return rxURL.MatchString(str)
}
