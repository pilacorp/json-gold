package ld

import (
	"reflect"
)

// scopedContextKey names one scoped context value for validateScopedContext:
// a resolved URL, or the identity of an inline map or array, with its base.
// An array also needs its length, as two arrays can share a backing store.
// The rest is the state of the active context's values that the check reads
// (§4.2.2 step 21.3 checks the value "using the active context"): a term
// without @id needs a vocabulary mapping (step 18), relative IRIs resolve
// against the base IRI, and the processing mode allows or refuses 1.1 terms.
// The terms it reads are recorded in each scopedPass.
type scopedContextKey struct {
	url  string
	ptr  uintptr
	len  int
	base string

	vocab    string
	hasVocab bool
	baseIRI  string
	mode     string
}

// maxScopedChecks bounds the passed checks kept for one scopedContextKey.
const maxScopedChecks = 8

// scopedChecks holds the checks of one scoped context value under one
// scopedContextKey: each passed check, and whether one is running (a scoped
// context that names itself).
type scopedChecks struct {
	running bool
	passed  []scopedPass
}

// scopedPass is one passed check: the terms it read from the active context,
// with what it found, and the remote contexts it processed itself.
type scopedPass struct {
	terms map[string]termState
	urls  []string
	// loads is how far past the processed count at its start the check
	// went, so it is reused only where that stays within the limit.
	loads int
}

// termState is what a lookup of a term in the active context returned.
type termState struct {
	def     *TermDefinition
	defined bool
	null    bool
}

func (c *Context) termState(term string) termState {
	def, defined := c.termDefinition(term)
	return termState{def: def, defined: defined, null: c.isProtectedNull(term)}
}

// same reports whether two lookups found equal definitions. Applying a scoped
// context defines its terms again, with new but equal definitions, so the
// fields are compared by value; the scoped context value by identity, as
// comparing its content would cost its size on every check. A false
// "different" only checks the value again.
func (s termState) same(o termState) bool {
	if s.defined != o.defined || s.null != o.null {
		return false
	}
	if s.def == o.def {
		return true
	}
	if s.def == nil || o.def == nil || !sameContextValue(s.def.Context, o.def.Context) {
		return false
	}
	a, b := *s.def, *o.def
	a.Context, b.Context = nil, nil
	return reflect.DeepEqual(a, b)
}

// sameContextValue compares two scoped context values by identity.
func sameContextValue(a, b interface{}) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case map[string]interface{}:
		y, ok := b.(map[string]interface{})
		return ok && reflect.ValueOf(x).Pointer() == reflect.ValueOf(y).Pointer()
	case []interface{}:
		y, ok := b.([]interface{})
		return ok && len(x) == len(y) && reflect.ValueOf(x).Pointer() == reflect.ValueOf(y).Pointer()
	}
	return false
}

// passedUnder reports whether a check passed while c held the same terms it
// read then, and none of the remote contexts it processed is in included now,
// where §4.1.2 step 5.2.2 would skip it.
func (sc *scopedChecks) passedUnder(c *Context, remote *remoteContexts) (scopedPass, bool) {
	limit := c.options.MaxRemoteContexts
	if limit <= 0 {
		limit = DefaultMaxRemoteContexts
	}
	for _, pass := range sc.passed {
		if remote.processed+pass.loads <= limit && pass.holds(c, remote.included) {
			return pass, true
		}
	}
	return scopedPass{}, false
}

func (p scopedPass) holds(c *Context, included map[string]bool) bool {
	for _, uri := range p.urls {
		if included[uri] {
			return false
		}
	}
	for term, state := range p.terms {
		if !c.termState(term).same(state) {
			return false
		}
	}
	return true
}

// scopedKey returns the key of scoped under c, and false for a value that has
// no identity to key on.
func (c *Context) scopedKey(scoped interface{}, baseURL string) (scopedContextKey, bool) {
	key := scopedContextKey{base: baseURL, baseIRI: c.Values.Base, mode: c.Values.processingMode}
	if c.Values.Vocab != nil {
		key.vocab, key.hasVocab = *c.Values.Vocab, true
	}
	switch v := scoped.(type) {
	case string:
		key.url = Resolve(baseURL, v)
	case map[string]interface{}:
		key.ptr = reflect.ValueOf(v).Pointer()
	case []interface{}:
		key.ptr, key.len = reflect.ValueOf(v).Pointer(), len(v)
	}
	return key, key.url != "" || key.ptr != 0
}

// forValidation is the copy of remote contexts step 21.3 passes, with
// validate scoped context false.
func (remote *remoteContexts) forValidation() *remoteContexts {
	copied := &remoteContexts{
		loaded: remote.loaded, processed: remote.processed, validated: remote.validated,
		included: make(map[string]bool, len(remote.included)), validating: true,
		processedURLs: make(map[string]bool), peak: remote.processed,
	}
	for uri := range remote.included {
		copied.included[uri] = true
	}
	return copied
}

// recordReads runs check and returns every term it looked up in c.
func (c *Context) recordReads(check func() error) (map[string]bool, error) {
	prev := c.reads
	c.reads = make(map[string]bool)
	defer func() { c.reads = prev }()
	err := check()
	return c.reads, err
}

// validateScopedContext is JSON-LD 1.1 API §4.2.2 step 21: the scoped context
// of a term definition is processed when the term is defined, so an error in
// it is reported even if the term is never used. The result is discarded; the
// scoped context is processed again where it applies.
//
// As step 21 says, the validation works on a copy of remote contexts, so the
// contexts it loads do not count toward the caller's limit. A passed check is
// reused for the same value under the same state of the active context, the
// terms it read included: applying a scoped context defines its terms again,
// and validating their scoped contexts every time would repeat the whole
// nested tree for each value of the term.
func (c *Context) validateScopedContext(termValue interface{}, baseURL string, remote *remoteContexts) error {
	termMap, isMap := termValue.(map[string]interface{})
	scoped, hasScoped := termMap["@context"]
	if !isMap || !hasScoped {
		return nil
	}
	key, cacheable := c.scopedKey(scoped, baseURL)
	validated := c.validated
	if validated == nil {
		if remote.validated == nil {
			remote.validated = make(map[scopedContextKey]*scopedChecks)
		}
		validated = remote.validated
	}
	var checks *scopedChecks
	if cacheable {
		checks = validated[key]
		if checks == nil {
			checks = &scopedChecks{}
			validated[key] = checks
		}
		if checks.running {
			return nil
		}
		if pass, ok := checks.passedUnder(c, remote); ok {
			// An outer check that reuses its own result later rests on the
			// URLs this one processed and on its loads.
			for _, uri := range pass.urls {
				if remote.processedURLs != nil {
					remote.processedURLs[uri] = true
				}
			}
			remote.peak = max(remote.peak, remote.processed+pass.loads)
			return nil
		}
		checks.running = true
		defer func() { checks.running = false }()
	}

	copied := remote.forValidation()
	reads, err := c.recordReads(func() error {
		_, err := c.overlay().parse(scoped, baseURL, copied, false, true, false, true)
		return err
	})
	if copied.skipped {
		remote.skipped = true // an outer check rests on the same list
	}
	if remote.processedURLs != nil {
		for uri := range copied.processedURLs {
			remote.processedURLs[uri] = true // and on what this one processed
		}
	}
	remote.peak = max(remote.peak, copied.peak)
	if err != nil {
		return NewJsonLdError(InvalidScopedContext, err)
	}
	// A check that skipped a context depends on the remote contexts list and
	// is not kept; past maxScopedChecks states the value is checked every time.
	if cacheable && !copied.skipped && len(checks.passed) < maxScopedChecks {
		pass := scopedPass{terms: make(map[string]termState, len(reads)), loads: copied.peak - remote.processed}
		for term := range reads {
			pass.terms[term] = c.termState(term)
		}
		for uri := range copied.processedURLs {
			pass.urls = append(pass.urls, uri)
		}
		checks.passed = append(checks.passed, pass)
	}
	return nil
}
