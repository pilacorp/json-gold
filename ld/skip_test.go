package ld_test

// This map contains prefixes for test IDs that should be skipped
// when running the official test suites for JSON-LD, Framing and Normalisation.
//
// Structure: <relative path to manifest file> ==> list of test ID prefixes to skip
var skippedTests = map[string][]string{
	"testdata/compact-manifest.jsonld": {
		"#tp001", // json-ld-1.0 processing mode is not supported
	},
	"testdata/expand-manifest.jsonld":  {},
	"testdata/flatten-manifest.jsonld": {},
	"testdata/fromRdf-manifest.jsonld": {
		"#tdi05", // No support for the rdfDirection option (i18n-datatype)
		"#tdi06", // No support for the rdfDirection option (i18n-datatype)
		"#tdi11", // No support for the rdfDirection option (compound-literal)
		"#tdi12", // No support for the rdfDirection option (compound-literal)
	},
	"testdata/remote-doc-manifest.jsonld": {
		"#t0013", // HTML documents aren't supported yet
	},
	"testdata/toRdf-manifest.jsonld": {
		"#tdi09", // No support for the rdfDirection option (i18n-datatype)
		"#tdi10", // No support for the rdfDirection option (i18n-datatype)
		"#tdi11", // No support for the rdfDirection option (compound-literal)
		"#tdi12", // No support for the rdfDirection option (compound-literal)
		"#te075", // No support for produceGeneralizedRdf
	},
	"testdata/html-manifest.jsonld": {
		"#t", // HTML inputs not supported yet
	},
	"testdata/frame-manifest.jsonld": {},
}
