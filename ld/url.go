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
	"net/url"
	"regexp"
	"strings"
)

// JsonLdUrl represents a URL split into individual components
// for easy manipulation.
// TODO: This implementation was taken from Java. Do we really need it in Go?
type JsonLdUrl struct { //nolint:stylecheck
	Href      string
	Protocol  string
	Host      string
	Auth      string
	User      string
	Password  string
	Hostname  string
	Port      string
	Relative  string
	Path      string
	Directory string
	File      string
	Query     string
	Hash      string

	// things not populated by the regex
	Pathname       string
	NormalizedPath string
	Authority      string
}

var parser = regexp.MustCompile(`^(?:([^:/?#]+):)?(?://((?:(([^:@]*)(?::([^:@]*))?)?@)?([^:/?#]*)(?::(\d*))?))?((((?:[^?#/]*/)*)([^?#]*))(?:\?([^#]*))?(?:#(.*))?)`)

// ParseURL parses a string URL into JsonLdUrl struct.
func ParseURL(urlStr string) *JsonLdUrl {
	rval := JsonLdUrl{Href: urlStr}

	if parser.MatchString(urlStr) {
		matches := parser.FindStringSubmatch(urlStr)
		if matches[1] != "" {
			rval.Protocol = matches[1]
		}
		if matches[2] != "" {
			rval.Host = matches[2]
		}
		if matches[3] != "" {
			rval.Auth = matches[3]
		}
		if matches[4] != "" {
			rval.User = matches[4]
		}
		if matches[5] != "" {
			rval.Password = matches[5]
		}
		if matches[6] != "" {
			rval.Hostname = matches[6]
		}
		if matches[7] != "" {
			rval.Port = matches[7]
		}
		if matches[8] != "" {
			rval.Relative = matches[8]
		}
		if matches[9] != "" {
			rval.Path = matches[9]
		}
		if matches[10] != "" {
			rval.Directory = matches[10]
		}
		if matches[11] != "" {
			rval.File = matches[11]
		}
		if matches[12] != "" {
			rval.Query = matches[12]
		}
		if matches[13] != "" {
			rval.Hash = matches[13]
		}

		// normalize to node.js API
		if rval.Host != "" && rval.Path == "" {
			rval.Path = "/"
		}

		rval.Pathname = rval.Path
		parseAuthority(&rval)
		rval.NormalizedPath = removeDotSegments(rval.Pathname, rval.Authority != "")
		if rval.Query != "" {
			rval.Path += "?" + rval.Query
		}
		if rval.Protocol != "" {
			rval.Protocol += ":"
		}
		if rval.Hash != "" {
			rval.Hash = "#" + rval.Hash
		}
	}

	return &rval
}

// removeDotSegments removes dot segments from a JsonLdUrl path.
func removeDotSegments(path string, hasAuthority bool) string {
	var rval []byte
	if strings.HasPrefix(path, "/") {
		rval = append(rval, '/')
	}

	// RFC 3986 5.2.4 (reworked)
	input := strings.Split(path, "/")
	var output = make([]string, 0)
	for i := 0; i < len(input); i++ {
		if input[i] == "." || (input[i] == "" && len(input)-i > 1) {
			continue
		}
		if input[i] == ".." {
			if hasAuthority || (len(output) > 0 && output[len(output)-1] != "..") {
				if len(output) > 0 {
					output = output[:len(output)-1]
				}
			} else {
				output = append(output, "..")
			}
			continue
		}
		output = append(output, input[i])
	}

	if len(output) > 0 {
		rval = append(rval, output[0]...)
		for i := 1; i < len(output); i++ {
			rval = append(rval, '/')
			rval = append(rval, output[i]...)
		}
	}
	return string(rval)
}

// RemoveBase removes base URL from the given IRI.
func RemoveBase(baseobj interface{}, iri string) string {
	if baseobj == nil {
		return iri
	}

	var base *JsonLdUrl
	if baseStr, isString := baseobj.(string); isString {
		base = ParseURL(baseStr)
	} else {
		base = baseobj.(*JsonLdUrl)
	}

	// establish base root
	root := ""
	if base.Href != "" {
		root += base.Protocol + "//" + base.Authority
	} else if !strings.HasPrefix(iri, "//") {
		// support network-path reference with empty base
		root += "//"
	}

	// IRI not relative to base
	if strings.Index(iri, root) != 0 {
		return iri
	}

	// remove root from IRI and parse remainder
	rel := ParseURL(iri[len(root):])

	// remove path segments that match
	baseSegments := strings.Split(base.NormalizedPath, "/")
	iriSegments := strings.Split(rel.NormalizedPath, "/")

	last := 1
	if len(rel.Hash) > 0 || len(rel.Query) > 0 {
		last = 0
	}

	for len(baseSegments) > 0 && len(iriSegments) > last && baseSegments[0] == iriSegments[0] {
		baseSegments = baseSegments[1:]
		iriSegments = iriSegments[1:]
	}

	// use '../' for each non-matching base segment
	rval := ""

	if len(baseSegments) > 0 {
		// don't count the last segment if it isn't a path (doesn't end in
		// '/')
		// don't count empty first segment, it means base began with '/'
		if !strings.HasSuffix(base.NormalizedPath, "/") || baseSegments[0] == "" {
			baseSegments = baseSegments[0 : len(baseSegments)-1]
		}
		for i := 0; i < len(baseSegments); i++ {
			rval += "../"
		}
	}

	// prepend remaining segments
	if len(iriSegments) > 0 {
		rval += iriSegments[0]
	}
	for i := 1; i < len(iriSegments); i++ {
		rval += "/" + iriSegments[i]
	}

	// add query and hash
	if rel.Query != "" {
		rval += "?" + rel.Query
	}
	if rel.Hash != "" {
		rval += rel.Hash
	}

	if rval == "" {
		rval = "./"
	}

	return rval
}

// Resolve resolves the reference pathToResolve against baseURI as RFC 3986
// §5.2 specifies. It works on the strings, so it neither percent-encodes nor
// normalizes an IRI, and it handles bases with no authority such as DIDs and
// URNs. An empty base, or a base or reference that net/url cannot parse,
// returns pathToResolve unchanged for the caller to reject.
func Resolve(baseURI string, pathToResolve string) string {
	if baseURI == "" {
		return pathToResolve
	}
	if _, err := url.Parse(baseURI); err != nil {
		return pathToResolve
	}
	if _, err := url.Parse(pathToResolve); err != nil {
		return pathToResolve
	}

	b := splitURIReference(baseURI)
	r := splitURIReference(pathToResolve)
	var t uriReference
	// §5.2.2
	switch {
	case r.hasScheme:
		// JSON-LD only resolves relative references; an absolute one is kept.
		return pathToResolve
	case r.hasAuthority:
		t.hasAuthority, t.authority = true, r.authority
		t.path = removeDotSegmentsRFC3986(r.path)
		t.hasQuery, t.query = r.hasQuery, r.query
	case r.path == "":
		t.path = b.path
		if r.hasQuery {
			t.hasQuery, t.query = true, r.query
		} else {
			t.hasQuery, t.query = b.hasQuery, b.query
		}
	default:
		if strings.HasPrefix(r.path, "/") {
			t.path = removeDotSegmentsRFC3986(r.path)
		} else {
			t.path = removeDotSegmentsRFC3986(mergePaths(b, r.path))
		}
		t.hasQuery, t.query = r.hasQuery, r.query
	}
	if !r.hasAuthority {
		t.hasAuthority, t.authority = b.hasAuthority, b.authority
	}
	t.hasScheme, t.scheme = b.hasScheme, b.scheme
	t.hasFragment, t.fragment = r.hasFragment, r.fragment
	return t.String()
}

// uriReference holds the five components of RFC 3986 §3. A component can be
// present and empty, as in "http://a/b?", so each optional one has a flag.
type uriReference struct {
	scheme, authority, path, query, fragment       string
	hasScheme, hasAuthority, hasQuery, hasFragment bool
}

// uriReferencePattern is the RFC 3986 Appendix B regular expression.
var uriReferencePattern = regexp.MustCompile(`^(([^:/?#]+):)?(//([^/?#]*))?([^?#]*)(\?([^#]*))?(#(.*))?$`)

func splitURIReference(ref string) uriReference {
	m := uriReferencePattern.FindStringSubmatchIndex(ref)
	part := func(group int) (string, bool) {
		if m[2*group] < 0 {
			return "", false
		}
		return ref[m[2*group]:m[2*group+1]], true
	}
	var u uriReference
	u.scheme, u.hasScheme = part(2)
	u.authority, u.hasAuthority = part(4)
	u.path, _ = part(5)
	u.query, u.hasQuery = part(7)
	u.fragment, u.hasFragment = part(9)
	return u
}

// String recomposes the reference as RFC 3986 §5.3 does.
func (u uriReference) String() string {
	var sb strings.Builder
	if u.hasScheme {
		sb.WriteString(u.scheme)
		sb.WriteByte(':')
	}
	if u.hasAuthority {
		sb.WriteString("//")
		sb.WriteString(u.authority)
	}
	sb.WriteString(u.path)
	if u.hasQuery {
		sb.WriteByte('?')
		sb.WriteString(u.query)
	}
	if u.hasFragment {
		sb.WriteByte('#')
		sb.WriteString(u.fragment)
	}
	return sb.String()
}

// mergePaths is RFC 3986 §5.2.3.
func mergePaths(base uriReference, refPath string) string {
	if base.hasAuthority && base.path == "" {
		return "/" + refPath
	}
	return base.path[:strings.LastIndex(base.path, "/")+1] + refPath
}

// removeDotSegmentsRFC3986 is RFC 3986 §5.2.4. Unlike removeDotSegments it
// keeps empty segments, so "a//b" stays as it is.
func removeDotSegmentsRFC3986(path string) string {
	var out strings.Builder
	in := path
	for in != "" {
		switch {
		case strings.HasPrefix(in, "../"):
			in = in[3:]
		case strings.HasPrefix(in, "./"):
			in = in[2:]
		case strings.HasPrefix(in, "/./"):
			in = in[2:]
		case in == "/.":
			in = "/"
		case strings.HasPrefix(in, "/../"):
			in = in[3:]
			dropLastSegment(&out)
		case in == "/..":
			in = "/"
			dropLastSegment(&out)
		case in == "." || in == "..":
			in = ""
		default:
			next := strings.IndexByte(in[1:], '/')
			if next < 0 {
				out.WriteString(in)
				in = ""
			} else {
				out.WriteString(in[:next+1])
				in = in[next+1:]
			}
		}
	}
	return out.String()
}

// dropLastSegment removes the last segment of out and the "/" before it.
func dropLastSegment(out *strings.Builder) {
	s := out.String()
	s = s[:max(strings.LastIndexByte(s, '/'), 0)]
	out.Reset()
	out.WriteString(s)
}

// parseAuthority parses the authority for the pre-parsed given JsonLdUrl.
func parseAuthority(parsed *JsonLdUrl) {
	// parse authority for unparsed relative network-path reference
	if !strings.Contains(parsed.Href, ":") && strings.HasPrefix(parsed.Href, "//") && parsed.Host == "" {
		// must parse authority from pathname
		parsed.Pathname = parsed.Pathname[2:]
		idx := strings.Index(parsed.Pathname, "/")
		if idx == -1 {
			parsed.Authority = parsed.Pathname
			parsed.Pathname = ""
		} else {
			parsed.Authority = parsed.Pathname[0:idx]
			parsed.Pathname = parsed.Pathname[idx:]
		}
	} else {
		// construct authority
		parsed.Authority = parsed.Host
		if parsed.Auth != "" {
			parsed.Authority = parsed.Auth + "@" + parsed.Authority
		}
	}
}
