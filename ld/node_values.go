package ld

import (
	"encoding/json"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// sameValue reports whether two JSON values are equal: same type and value,
// maps and arrays compared entry by entry. Unlike ==, it does not panic on
// a JSON literal, whose @value is a map or an array.
func sameValue(a, b interface{}) bool {
	ka, okA := jsonKey(a)
	kb, okB := jsonKey(b)
	if okA && okB {
		return ka == kb
	}
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	if ta != tb {
		return false
	}
	if ta != nil && !ta.Comparable() {
		return reflect.DeepEqual(a, b)
	}
	return a == b
}

// jsonKey encodes a JSON value so that two values have the same key exactly
// when sameValue holds. It reports false for a value it cannot encode, such
// as NaN or a non-JSON Go type; such a value equals nothing of another type.
func jsonKey(v interface{}) (string, bool) {
	var sb strings.Builder
	if !writeJSONKey(&sb, v) {
		return "", false
	}
	return sb.String(), true
}

func writeJSONKey(sb *strings.Builder, v interface{}) bool {
	switch x := v.(type) {
	case nil:
		sb.WriteByte('n')
	case bool:
		if x {
			sb.WriteByte('t')
		} else {
			sb.WriteByte('f')
		}
	case string:
		sb.WriteByte('s')
		sb.WriteString(strconv.Quote(x))
	case float64:
		return writeFloatKey(sb, x)
	case json.Number:
		// toRDF writes a json.Number from its float64 value, so 1, 1.0 and the
		// float64 1 are one value. Text that does not parse keeps its own key.
		if f, err := x.Float64(); err == nil {
			return writeFloatKey(sb, f)
		}
		sb.WriteByte('N')
		sb.WriteString(strconv.Quote(string(x)))
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteByte('{')
		for _, k := range keys {
			sb.WriteString(strconv.Quote(k))
			if !writeJSONKey(sb, x[k]) {
				return false
			}
		}
		sb.WriteByte('}')
	case []interface{}:
		sb.WriteByte('[')
		for _, e := range x {
			if !writeJSONKey(sb, e) {
				return false
			}
		}
		sb.WriteByte(']')
	default:
		return false
	}
	return true
}

// valueEntries are the entries that make two value objects equivalent. Like
// jsonld.js, @direction is not one: toRDF drops it, so values that differ
// only by direction would make the same quad twice.
var valueEntries = [...]string{"@value", "@type", "@language", "@index"}

// valueKey returns the key under which CompareValues finds v equal to
// another value. It reports false for a list object, which equals nothing,
// and for a value it cannot encode, such as an int in a map built by hand.
func valueKey(v interface{}) (string, bool) {
	m, isMap := v.(map[string]interface{})
	if !isMap {
		k, ok := jsonKey(v)
		return "p" + k, ok
	}
	_, hasValue := m["@value"]
	id, hasID := m["@id"]
	switch {
	case hasValue && hasID:
		return "", false
	case hasValue:
		var sb strings.Builder
		sb.WriteByte('v')
		for _, e := range valueEntries {
			if !writeJSONKey(&sb, m[e]) {
				return "", false
			}
		}
		return sb.String(), true
	case hasID:
		k, ok := jsonKey(id)
		return "i" + k, ok
	}
	return "", false
}

// nodeValues adds values to node map properties, dropping a value equal to
// one already there in constant time. AddValue finds duplicates by scanning
// the property's array, which is quadratic for a property with many values.
// The index lives for one algorithm run, and learns a property's existing
// values the first time it touches that property.
type nodeValues struct {
	seen map[nodeProperty]map[string]struct{}
}

type nodeProperty struct {
	node     unsafe.Pointer // identity of the node map
	property string
}

func newNodeValues() *nodeValues {
	return &nodeValues{seen: make(map[nodeProperty]map[string]struct{})}
}

// add has the effect of AddValue(node, property, value, true, false, false,
// false): the property becomes an array, and value is appended unless an
// equal value is already in it. An array value adds each of its items.
func (nv *nodeValues) add(node map[string]interface{}, property string, value interface{}) {
	if items, isArray := value.([]interface{}); isArray {
		if _, found := node[property]; !found && len(items) == 0 {
			node[property] = []interface{}{}
		}
		for _, item := range items {
			nv.add(node, property, item)
		}
		return
	}
	existing, found := node[property]
	values, isArray := existing.([]interface{})
	if found && !isArray {
		// not a shape the node map builds; keep AddValue's handling
		AddValue(node, property, value, true, false, false, false)
		return
	}
	key := nodeProperty{node: reflect.ValueOf(node).UnsafePointer(), property: property}
	set, indexed := nv.seen[key]
	if !indexed {
		set = make(map[string]struct{}, len(values))
		for _, v := range values {
			if k, ok := valueKey(v); ok {
				set[k] = struct{}{}
			}
		}
		nv.seen[key] = set
	}
	k, ok := valueKey(value)
	if !ok {
		// CompareValues may still find it equal to a value of its own Go
		// type, so scan as AddValue does
		AddValue(node, property, value, true, false, false, false)
		return
	}
	if _, dup := set[k]; dup {
		return
	}
	set[k] = struct{}{}
	if values == nil {
		values = []interface{}{}
	}
	node[property] = append(values, value)
}

func writeFloatKey(sb *strings.Builder, x float64) bool {
	if math.IsNaN(x) {
		return false
	}
	if x == 0 {
		x = 0 // -0 == 0, as for ==
	}
	sb.WriteByte('d')
	sb.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	sb.WriteByte(';')
	return true
}
