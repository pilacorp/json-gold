package ld_test

// This map contains prefixes for test IDs that should be skipped
// when running the official test suites for JSON-LD, Framing and Normalisation.
//
// Structure: <relative path to manifest file> ==> list of test ID prefixes to skip
var skippedTests = map[string][]string{
	"testdata/compact-manifest.jsonld": {
		"#tp001", // json-ld-1.0 processing mode is not supported
		"#t0111", // TODO
		"#t0112", // TODO
		"#t0113", // TODO
		"#tin",   // TODO
	},
	"testdata/expand-manifest.jsonld": {
		"#t0122", // TODO
		"#t0123", // TODO
		"#tc032", // TODO
		"#tc033", // TODO
		"#tc037", // TODO: New test
		"#tc038", // TODO: New test
		"#tec02", // TODO
		"#ter52", // TODO
		"#ter54", // TODO: New test
		"#ter56", // TODO: New test
		"#tpr28", // TODO: New test
		"#tpr38", // TODO: New test
		"#tpr39", // TODO: New test
	},
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
		"#tc032", // TODO
		"#tc033", // TODO
		"#tc037", // TODO: New test
		"#tc038", // TODO: New test
		"#te123", // TODO: new test
		"#tec02", // TODO: new test
		"#ter52", // TODO: new test
		"#ter54", // TODO: new test
		"#ter56", // TODO: new test
		"#tpr28", // Skipped in Expand test suite
		"#tpr38", // TODO
		"#tpr39", // TODO
	},
	"testdata/html-manifest.jsonld": {
		"#t", // HTML inputs not supported yet
	},
	"testdata/frame-manifest.jsonld": {
		"#t0064", // @type with @default value in frame requires expansion changes
		"#tin",   // More involved, with @included to pass current subjects instead of frame keys
	},
}
