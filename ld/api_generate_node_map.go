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
	"strings"
)

// GenerateNodeMap recursively flattens the subjects in the given JSON-LD expanded
// input into a node map.
func (api *JsonLdApi) GenerateNodeMap(element interface{}, graphMap map[string]interface{}, activeGraph string,
	issuer *IdentifierIssuer, activeSubject interface{}, activeProperty string, list map[string]interface{}) (map[string]interface{}, error) {
	return api.generateNodeMap(newNodeValues(), element, graphMap, activeGraph, issuer, activeSubject, activeProperty, list)
}

func (api *JsonLdApi) generateNodeMap(nv *nodeValues, element interface{}, graphMap map[string]interface{}, activeGraph string,
	issuer *IdentifierIssuer, activeSubject interface{}, activeProperty string, list map[string]interface{}) (map[string]interface{}, error) {

	// recurse through array
	if elementList, isList := element.([]interface{}); isList {
		// if element is an array, process each entry in element recursively by passing item for element,
		// node map, active graph, active subject, active property, and list.
		for _, item := range elementList {
			var err error
			list, err = api.generateNodeMap(nv, item, graphMap, activeGraph, issuer, activeSubject, activeProperty, list)
			if err != nil {
				return nil, err
			}
		}
		return list, nil
	}

	// add non-object to list
	elem, isMap := element.(map[string]interface{})
	if !isMap {
		return nil, fmt.Errorf("expected map or list to GenerateNodeMap, got %T", element)
	}

	var graph map[string]interface{}
	if graphVal, found := graphMap[activeGraph]; found {
		graph = graphVal.(map[string]interface{})
	} else {
		graph = make(map[string]interface{})
		graphMap[activeGraph] = graph
	}

	var subjectNode map[string]interface{}
	if activeSubject == nil {
		subjectNode = graph
	} else if subject, isString := activeSubject.(string); isString {
		subjectNode, _ = graph[subject].(map[string]interface{})
	} else {
		subjectNode = make(map[string]interface{})
	}

	// transform bnode types
	if typeVal, hasType := elem["@type"]; hasType {
		types := Arrayify(typeVal)
		newTypes := make([]interface{}, len(types))
		for i, t := range types {
			typeStr := t.(string)
			if strings.HasPrefix(typeStr, "_:") { // use IsBlankNodeValue()
				typeStr = issuer.GetId(typeStr)
			}
			newTypes[i] = typeStr
		}
		if IsValue(element) {
			elem["@type"] = newTypes[0]
		} else {
			elem["@type"] = newTypes
		}
	}

	// A value or list object reached with no subject node sits directly in
	// a graph, e.g. a string under a @graph container term (JSON-LD 1.1 API
	// §5.1.2 step 13.8.3). §7.2.2 gives it no place, and like any
	// free-floating value (§5.1.2 step 19.1) it is dropped.
	if IsValue(element) {
		if list == nil {
			if subjectNode != nil {
				nv.add(subjectNode, activeProperty, element)
			}
		} else {
			list["@list"] = append(list["@list"].([]interface{}), element)
		}
		return list, nil
	} else if IsList(element) {
		result := map[string]interface{}{
			"@list": []interface{}{},
		}
		var err error
		result, err = api.generateNodeMap(nv, elem["@list"], graphMap, activeGraph, issuer, activeSubject, activeProperty, result)
		if err != nil {
			return nil, err
		}
		if list == nil {
			if subjectNode != nil {
				nv.add(subjectNode, activeProperty, result)
			}
		} else {
			list["@list"] = append(list["@list"].([]interface{}), result)
		}
		return list, nil
	}

	// element is a node object

	// An @id that expanded to null (a keyword-like IRI, §5.2.2 step 2) names
	// no node; §7.2.2 does not cover it. As in jsonld.js, a value of a
	// property is dropped with its properties, while a list item, a @reverse
	// value and a node at graph level get a blank node identifier.
	id, hasID := elem["@id"]
	_, isReverse := activeSubject.(map[string]interface{})
	if hasID && id == nil && activeProperty != "" && list == nil && !isReverse {
		return list, nil
	}
	if id == nil {
		id = issuer.GetId("")
	} else if strings.HasPrefix(id.(string), "_:") {
		id = issuer.GetId(id.(string))
	}

	nodeVal, found := graph[id.(string)]
	if !found {
		nodeVal = map[string]interface{}{
			"@id": id,
		}
		graph[id.(string)] = nodeVal
	}
	node := nodeVal.(map[string]interface{})

	if _, isMap := activeSubject.(map[string]interface{}); isMap {
		// if subject is a hash, then we're processing a reverse-property relationship.
		nv.add(node, activeProperty, activeSubject)
	} else if activeProperty != "" {
		ref := map[string]interface{}{
			"@id": id,
		}
		if list == nil {
			nv.add(subjectNode, activeProperty, ref)
		} else {
			list["@list"] = append(list["@list"].([]interface{}), ref)
		}
	}

	if typeVal, hasType := elem["@type"]; hasType {
		nv.add(node, "@type", typeVal)
	}

	if elemIdx, hasIndex := elem["@index"]; hasIndex {
		if nodeIdx, found := node["@index"]; found && nodeIdx != elemIdx {
			return nil, NewJsonLdError(ConflictingIndexes, "conflicting @index property detected")
		}
		node["@index"] = elemIdx
	}

	// handle reverse properties
	if reverseVal, hasReverse := elem["@reverse"]; hasReverse {
		referencedNode := map[string]interface{}{
			"@id": id,
		}
		reverseMap := reverseVal.(map[string]interface{})
		for reverseProperty, values := range reverseMap {
			for _, v := range values.([]interface{}) {
				_, err := api.generateNodeMap(nv, v, graphMap, activeGraph, issuer, referencedNode, reverseProperty, nil)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if graphVal, hasGraph := elem["@graph"]; hasGraph {
		_, err := api.generateNodeMap(nv, graphVal, graphMap, id.(string), issuer, "", "", nil)
		if err != nil {
			return nil, err
		}
	}

	if includedVal, hasIncluded := elem["@included"]; hasIncluded {
		_, err := api.generateNodeMap(nv, includedVal, graphMap, activeGraph, issuer, "", "", nil)
		if err != nil {
			return nil, err
		}
	}

	for _, property := range GetOrderedKeys(elem) {
		if property == "@id" || property == "@type" || property == "@index" || property == "@reverse" ||
			property == "@graph" || property == "@included" {
			// already processed
			continue
		}

		value := elem[property]

		// if property is a bnode, assign it a new id
		if strings.HasPrefix(property, "_:") {
			property = issuer.GetId(property)
		}

		if _, found := node[property]; !found {
			node[property] = []interface{}{}
		}
		if _, err := api.generateNodeMap(nv, value, graphMap, activeGraph, issuer, id.(string), property, nil); err != nil {
			return nil, err
		}
	}

	return list, nil
}
