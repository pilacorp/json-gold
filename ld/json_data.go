package ld

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
)

// maxJSONDepth is the nesting limit encoding/json applies when it decodes
// JSON text, so a Go value is held to the same limit as the text it stands
// for. It also ends a walk over a value that contains itself.
const maxJSONDepth = 10000

// toJSONData returns value in the form encoding/json decodes JSON into:
// map[string]interface{}, []interface{}, string, float64 or json.Number,
// bool and nil. The algorithms only handle those types, so a document built
// in Go with int, float32, []string, map[string]string or a struct passes
// through encoding/json first, keeping its numbers as json.Number so no digit
// is lost. A value already in that form is returned as is, and a container is
// copied only when something inside it changes, so the caller's document is
// never modified.
//
// A container shared in several places is walked once. The processor reads
// a shared scoped context once per call, so walking each use again would
// make the walk the cost of the call.
func toJSONData(value interface{}) (interface{}, error) {
	w := jsonDataWalker{done: make(map[jsonContainerKey]jsonDataResult)}
	r, err := w.convert(value, 0)
	return r.value, err
}

// jsonContainerKey names a map, or a slice with its length, by the memory
// it points to.
type jsonContainerKey struct {
	pointer uintptr
	length  int
}

// jsonDataResult is a converted value, whether it differs from the input,
// and how many levels it nests, so a container met again deeper down is
// held to maxJSONDepth without walking it twice.
type jsonDataResult struct {
	value   interface{}
	changed bool
	height  int
}

type jsonDataWalker struct {
	done map[jsonContainerKey]jsonDataResult
}

func (w *jsonDataWalker) convert(value interface{}, depth int) (jsonDataResult, error) {
	if depth > maxJSONDepth {
		return jsonDataResult{}, tooDeepError()
	}
	switch v := value.(type) {
	case nil, string, bool:
		return jsonDataResult{value: value}, nil
	case json.Number:
		// encoding/json checks the text of a json.Number when it marshals
		// one, and writes "" as 0. A decoder never makes "NaN" or "0x10",
		// but Go code can.
		encoded, err := json.Marshal(v)
		if err != nil {
			return jsonDataResult{}, NewJsonLdError(InvalidInput, fmt.Errorf("%q is not a JSON number: %w", string(v), err))
		}
		if string(encoded) != string(v) {
			return jsonDataResult{value: json.Number(encoded), changed: true}, nil
		}
		return jsonDataResult{value: value}, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return jsonDataResult{}, NewJsonLdError(InvalidInput, fmt.Sprintf("%v is not a JSON number", v))
		}
		return jsonDataResult{value: value}, nil
	case map[string]interface{}:
		key := jsonContainerKey{pointer: reflect.ValueOf(v).Pointer(), length: -1}
		return w.convertOnce(key, depth, func() (jsonDataResult, error) { return w.convertMap(v, depth) })
	case []interface{}:
		if len(v) == 0 {
			return jsonDataResult{value: value}, nil
		}
		key := jsonContainerKey{pointer: reflect.ValueOf(v).Pointer(), length: len(v)}
		return w.convertOnce(key, depth, func() (jsonDataResult, error) { return w.convertSlice(v, depth) })
	default:
		decoded, err := marshalAsJSONData(value)
		if err != nil {
			return jsonDataResult{}, err
		}
		// The decoded value is walked too, so its nesting counts from here.
		r, err := w.convert(decoded, depth)
		r.changed = true
		return r, err
	}
}

func (w *jsonDataWalker) convertOnce(key jsonContainerKey, depth int, convert func() (jsonDataResult, error)) (jsonDataResult, error) {
	if r, seen := w.done[key]; seen {
		if depth+r.height > maxJSONDepth {
			return jsonDataResult{}, tooDeepError()
		}
		return r, nil
	}
	r, err := convert()
	if err != nil {
		return jsonDataResult{}, err
	}
	w.done[key] = r
	return r, nil
}

func (w *jsonDataWalker) convertMap(m map[string]interface{}, depth int) (jsonDataResult, error) {
	var copied map[string]interface{}
	height := 0
	// Sorted, so the first value that cannot be converted is the same on
	// every run.
	for _, key := range GetOrderedKeys(m) {
		r, err := w.convert(m[key], depth+1)
		if err != nil {
			return jsonDataResult{}, err
		}
		height = max(height, r.height+1)
		if r.changed && copied == nil {
			copied = make(map[string]interface{}, len(m))
			for k, original := range m {
				copied[k] = original
			}
		}
		if copied != nil {
			copied[key] = r.value
		}
	}
	if copied == nil {
		return jsonDataResult{value: m, height: height}, nil
	}
	return jsonDataResult{value: copied, changed: true, height: height}, nil
}

func (w *jsonDataWalker) convertSlice(s []interface{}, depth int) (jsonDataResult, error) {
	var copied []interface{}
	height := 0
	for i, item := range s {
		r, err := w.convert(item, depth+1)
		if err != nil {
			return jsonDataResult{}, err
		}
		height = max(height, r.height+1)
		if r.changed && copied == nil {
			copied = make([]interface{}, len(s))
			copy(copied, s)
		}
		if copied != nil {
			copied[i] = r.value
		}
	}
	if copied == nil {
		return jsonDataResult{value: s, height: height}, nil
	}
	return jsonDataResult{value: copied, changed: true, height: height}, nil
}

func tooDeepError() error {
	return NewJsonLdError(InvalidInput, fmt.Sprintf("the document is nested deeper than %d levels", maxJSONDepth))
}

// marshalAsJSONData sends a value of any other Go type through encoding/json.
// The decoder keeps numbers as json.Number, as an int64 above 2^53 would
// lose digits as a float64.
func marshalAsJSONData(value interface{}) (interface{}, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, NewJsonLdError(InvalidInput, fmt.Errorf("a %T value cannot be read as JSON: %w", value, err))
	}
	dec := json.NewDecoder(bytes.NewReader(encoded))
	dec.UseNumber()
	var decoded interface{}
	if err := dec.Decode(&decoded); err != nil {
		return nil, NewJsonLdError(InvalidInput, fmt.Errorf("a %T value cannot be read as JSON: %w", value, err))
	}
	return decoded, nil
}

// copyAsJSONData is toJSONData followed by a deep copy, for a value the
// processor changes in place, such as a context it adds to its output.
func copyAsJSONData(value interface{}) (interface{}, error) {
	converted, err := toJSONData(value)
	if err != nil {
		return nil, err
	}
	return CloneDocument(converted), nil
}
