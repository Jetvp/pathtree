// pathtree implements a tree for fast path lookup.
//
// Restrictions
//
//   - Paths must be a '/'-separated list of strings, like a URL or Unix filesystem.
//   - All paths must begin with a '/'.
//   - Path elements may not contain a '/'.
//   - Path elements beginning with a ':' or '*' will be interpreted as wildcards.
//   - Trailing slashes are inconsequential.
//
// Wildcards
//
// Wildcards are named path elements that may match any strings in that
// location.  Two different kinds of wildcards are permitted:
//   - :var - names beginning with ':' will match any single path element.
//   - *var - names beginning with '*' will match one or more path elements.
//            (however, no path elements may come after a star wildcard)
//
// Algorithm
//
// Paths are mapped to the tree in the following way:
//   - Each '/' is a Node in the tree. The root node is the leading '/'.
//   - Each Node has edges to other nodes. The edges are named according to the
//     possible path elements at that depth in the path.
//   - Any Node may have an associated Leaf.  Leafs are terminals containing the
//     data associated with the path as traversed from the root to that Node.
//
// Edges are implemented as a map from the path element name to the next node in
// the path.
package pathtree

import (
	"errors"
	"strings"
)

type Node struct {
	edges     map[string]*Node    // the various path elements leading out of this node.
	wildcards map[string]Wildcard // if set, this slice holds nodes for paths with wildcard elements.
	leaf      *Leaf               // if set, this is a terminal node for this leaf.
	star      *Leaf               // if set, this path ends in a star.
	leafs     int                 // counter for # leafs in the tree
}

type Leaf struct {
	Value     interface{} // the value associated with this node
	Wildcards []string    // the wildcard names, in order they appear in the path
	order     int         // the order this leaf was added
}

type Wildcard struct {
	node     *Node    // node for this wildcard element
	padding  []string // padding elements between each var
	wildend  bool     // if it ends with a wildcard
	minorder int      // minimum order value in this path
}

// New returns a new path tree.
func New() *Node {
	return &Node{edges: make(map[string]*Node), wildcards: make(map[string]Wildcard)}
}

// Adds a new wildcard element to the node and returns the node
func (n *Node) addWildcard(padding []string, representation string, wildend bool, order int) *Node {
	element := Wildcard{node: New(), padding: padding, wildend: wildend, minorder: order}
	n.wildcards[representation] = element
	return element.node
}

// Add a path and its associated value to the tree.
//   - key must begin with "/"
//   - key must not duplicate any existing key.
// Returns an error if those conditions do not hold.
func (n *Node) Add(key string, val interface{}) error {
	if key[0] != '/' {
		return errors.New("Path must begin with /")
	}
	n.leafs++
	return n.add(n.leafs, splitPath(key), nil, val)
}

func (n *Node) add(order int, elements, wildcards []string, val interface{}) error {
	if len(elements) == 0 {
		if n.leaf != nil {
			return errors.New("duplicate path")
		}
		n.leaf = &Leaf{
			order:     order,
			Value:     val,
			Wildcards: wildcards,
		}
		return nil
	}

	var el string
	el, elements = elements[0], elements[1:]

	// Handle stars
	if len(el) > 0 && el[0] == '*' {
		if n.star != nil {
			return errors.New("duplicate path")
		}
		n.star = &Leaf{
			order:     order,
			Value:     val,
			Wildcards: append(wildcards, el[1:]),
		}
		return nil
	}

	// Handle wildcards
	if pos := strings.LastIndex(el, ":"); pos != -1 {
		// Remove any final closing :
		if el[len(el)-1] == ':' {
			el = el[:len(el)-1]
		}
		parts := strings.Split(el, ":")
		paddings := make([]string, len(parts)/2+len(parts)%2)
		variables := make([]string, len(parts)/2)

		// Split appart padding and variables (padding first even if empty)
		for key, value := range parts {
			if key%2 == 0 {
				paddings[key/2] = value
			} else {
				variables[key/2] = value
			}
		}

		// Create string representation for map
		wildend := len(paddings) == len(variables)
		representation := strings.Join(paddings, ":")
		if wildend {
			representation = representation + ":"
		}

		// Test if map contains representation else create it
		item, ok := n.wildcards[representation]
		var node *Node
		if ok {
			node = item.node
			if item.minorder > order {
				item.minorder = order
			}
		} else {
			node = n.addWildcard(paddings, representation, wildend, order)
		}

		return node.add(order, elements, append(wildcards, variables...), val)
	}

	// It's a normal path element.
	e, ok := n.edges[el]
	if !ok {
		e = New()
		n.edges[el] = e
	}

	return e.add(order, elements, wildcards, val)
}

// Find a given path. Any wildcards traversed along the way are expanded and
// returned, along with the value.
func (n *Node) Find(key string) (leaf *Leaf, expansions []string) {
	if len(key) == 0 || key[0] != '/' {
		return nil, nil
	}

	return n.find(splitPath(key), nil)
}

func (n *Node) find(elements, exp []string) (leaf *Leaf, expansions []string) {
	if len(elements) == 0 {
		return n.leaf, exp
	}

	// If this node has a star, calculate the star expansions in advance.
	var starExpansion string
	if n.star != nil {
		starExpansion = strings.Join(elements, "/")
	}

	// Peel off the next element and look up the associated edge.
	var el string
	el, elements = elements[0], elements[1:]
	if nextNode, ok := n.edges[el]; ok {
		if testleaf, testexpansions := nextNode.find(elements, exp); testleaf != nil {
			leaf, expansions = testleaf, testexpansions
		}
	}

	// Handle wildards
	for _, value := range n.wildcards {
		// Only check if tree contrains lower order item
		if leaf != nil && leaf.order < value.minorder {
			continue
		}

		found := bool(true)
		variables := make([]string, 0, 0)
		input := el

		// Check all padding elements are present
		for count, pad := range value.padding {
			pos := strings.Index(input, pad)

			if (pos == -1) || (count == 0 && pos > 0) {
				found = false
				break
			}

			if count != 0 {
				variables = append(variables, input[:pos])
			}
			input = input[pos+len(pad):]
		}

		if !found {
			continue
		}

		if value.wildend {
			variables = append(variables, input)
		}

		// Set leaf if it meets lower levels
		if testleaf, testexpansions := value.node.find(elements, append(exp, variables...)); testleaf != nil {
			if leaf == nil || leaf.order > testleaf.order {
				leaf, expansions = testleaf, testexpansions
			}
		}
	}

	// Handle star
	if n.star != nil && (leaf == nil || leaf.order > n.star.order) {
		leaf = n.star
		expansions = append(exp, starExpansion)
	}

	return
}

func splitPath(key string) []string {
	elements := strings.Split(key, "/")
	if elements[0] == "" {
		elements = elements[1:]
	}
	if elements[len(elements)-1] == "" {
		elements = elements[:len(elements)-1]
	}
	return elements
}
