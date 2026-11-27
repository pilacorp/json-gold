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

type Embed string

const (
	JsonLd_1_0       = "json-ld-1.0"              //nolint:stylecheck
	JsonLd_1_1       = "json-ld-1.1"              //nolint:stylecheck
	JsonLd_1_1_Frame = "json-ld-1.1-expand-frame" //nolint:stylecheck

	EmbedLast   Embed = "@last"
	EmbedOnce   Embed = "@once"
	EmbedAlways Embed = "@always"
	EmbedNever  Embed = "@never"
)

// JsonLdOptions type as specified in the JSON-LD-API specification:
// http://www.w3.org/TR/json-ld-api/#the-jsonldoptions-type
type JsonLdOptions struct { //nolint:stylecheck

	// Base options: http://www.w3.org/TR/json-ld-api/#idl-def-JsonLdOptions

	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-base
	Base string
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-compactArrays
	CompactArrays bool
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-expandContext
	ExpandContext interface{}
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-processingMode
	ProcessingMode string
	// http://www.w3.org/TR/json-ld-api/#widl-JsonLdOptions-documentLoader
	DocumentLoader DocumentLoader

	// Frame options: https://www.w3.org/TR/json-ld-framing/#jsonldoptions
	Embed        Embed
	Explicit     bool
	FrameDefault bool
	OmitDefault  bool
	OmitGraph    bool
	Ordered      bool
	RequireAll   bool

	// RDF conversion options: http://www.w3.org/TR/json-ld-api/#serialize-rdf-as-json-ld-algorithm

	UseRdfType            bool
	UseNativeTypes        bool
	ProduceGeneralizedRdf bool

	// The following properties aren't in the spec

	InputFormat   string
	Format        string
	Algorithm     string
	UseNamespaces bool
	OutputForm    string

	// SafeMode turns these silent drops into errors: an undefined term, a
	// keyword-like IRI, an IRI or language tag that is not well-formed, a
	// relative IRI in a statement, an object with only @language, and a value
	// with no subject in its graph. The last also refuses a node with only @id
	// at the top level or in @graph, and a document with only @context, as
	// jsonld.js does. A frame passed to Frame is a pattern and is not checked.
	SafeMode bool

	// MaxRemoteContexts is the processor limit of JSON-LD 1.1 API §4.1.2
	// step 5.2.3: how many remote contexts one @context value may process,
	// repeats and nested contexts included. The document's top-level
	// context, each embedded node context, each scoped context applied and
	// ExpandContext count separately. Expanding one document through
	// JsonLdProcessor loads each context URL once, @import and scoped
	// contexts included, however often it is processed; compacting or
	// framing loads the contexts of its own context argument once more.
	// With a Context the caller builds and passes to JsonLdApi, each
	// @context value loads its URLs once and nothing is kept between calls.
	// Zero or negative means DefaultMaxRemoteContexts.
	MaxRemoteContexts int

	// MaxScopedContexts limits how many times one processor call runs the
	// Context Processing algorithm on a term's scoped context or on a node's
	// own @context. JSON-LD 1.1 API (W3C REC 2020-07-16) does so in §5.1.2
	// steps 8, 9, 11.2 and 13.8.3.2 and §6.1.2 steps 6.1 and 11.1, and this
	// processor also for @nest, as the JSON-LD 1.2 draft's §5.1.2 step 14.2.2
	// does. It runs once for every object under a property-scoped term and
	// every node of a type-scoped type, so a small document that uses a large
	// context many times costs uses times its size. Zero or negative means no
	// limit. Past the limit the call fails with ScopedContextOverflow. Only
	// calls through JsonLdProcessor count, as they keep per-call state.
	MaxScopedContexts int

	// scopedApplied counts toward MaxScopedContexts across the expansion and
	// compaction phases of one exported call. Copy leaves it out, so every
	// call, which copies the options first, starts at zero.
	scopedApplied *int

	// ExpandedElementHandler, when set, is called during expansion for each
	// node object expansion keeps, with that object's JSON Pointer and the
	// result of expanding it. See ElementHandler.
	//
	// Nil by default, and nil costs nothing: no pointer is built and no
	// callback is made.
	ExpandedElementHandler ElementHandler
}

// NewJsonLdOptions creates and returns new instance of JsonLdOptions with the given base.
func NewJsonLdOptions(base string) *JsonLdOptions { //nolint:stylecheck
	return &JsonLdOptions{
		Base:                  base,
		CompactArrays:         true,
		ProcessingMode:        JsonLd_1_1,
		DocumentLoader:        NewDefaultDocumentLoader(nil),
		Embed:                 EmbedOnce,
		Explicit:              false,
		RequireAll:            true,
		FrameDefault:          false,
		OmitDefault:           false,
		OmitGraph:             true,
		Ordered:               false,
		UseRdfType:            false,
		UseNativeTypes:        false,
		ProduceGeneralizedRdf: false,
		InputFormat:           "",
		Format:                "",
		Algorithm:             AlgorithmURGNA2012,
		UseNamespaces:         false,
		OutputForm:            "",
		SafeMode:              false,
	}
}

// callCounter returns the call's MaxScopedContexts counter, creating it on
// first use.
func (opt *JsonLdOptions) callCounter() *int {
	if opt.scopedApplied == nil {
		opt.scopedApplied = new(int)
	}
	return opt.scopedApplied
}

// Copy creates a deep copy of JsonLdOptions object.
func (opt *JsonLdOptions) Copy() *JsonLdOptions {
	return &JsonLdOptions{
		Base:                   opt.Base,
		CompactArrays:          opt.CompactArrays,
		ExpandContext:          opt.ExpandContext,
		ProcessingMode:         opt.ProcessingMode,
		DocumentLoader:         opt.DocumentLoader,
		Embed:                  opt.Embed,
		Explicit:               opt.Explicit,
		RequireAll:             opt.RequireAll,
		FrameDefault:           opt.FrameDefault,
		OmitDefault:            opt.OmitDefault,
		OmitGraph:              opt.OmitGraph,
		Ordered:                opt.Ordered,
		UseRdfType:             opt.UseRdfType,
		UseNativeTypes:         opt.UseNativeTypes,
		ProduceGeneralizedRdf:  opt.ProduceGeneralizedRdf,
		InputFormat:            opt.InputFormat,
		Format:                 opt.Format,
		Algorithm:              opt.Algorithm,
		UseNamespaces:          opt.UseNamespaces,
		OutputForm:             opt.OutputForm,
		SafeMode:               opt.SafeMode,
		ExpandedElementHandler: opt.ExpandedElementHandler,
		MaxRemoteContexts:      opt.MaxRemoteContexts,
		MaxScopedContexts:      opt.MaxScopedContexts,
	}
}
