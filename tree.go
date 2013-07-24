// pathtree implements a tree for fast path lookup.
//
// Restrictions
//
//   - Paths must be a '/'-separated list of strings, like a URL or Unix filesystem.
//   - All paths must begin with a '/'.
//   - Path elements may not contain a '/'.
//   - Path elements containing multiple ':[min,max]varible;' will be interpreted as wildcards.
//   - Path elements beginning '*' will be interpreted as an ongoing wildcard.
//   - Trailing slashes are inconsequential and included on reverse.
//   - Paths can have multiple options for padding values between wildcards split with "|"
//
// Wildcards
//
// Wildcards are named path elements that may match any strings in that
// location with surrounding padding elements that preceed and end it.
// Different kinds of wildcards are permitted:
//   - :[min,max]var; - will match any single path element between legths min
//     and max which must be numeric.
//   - :[length]var; - will match any single path element of the set length
//     which be numeric.
//   - :var; - will match any single path element of and length.
//   - *var - names beginning with '*' will match one or more path elements.
//            (however, no path elements may come after a star wildcard)
// For backwards compadability the trailing ';' of the last wildcard can be left
// off if there is no padding after it.
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
// the path.  They contain a slice of padding elements before each wildcard and
// a slice of the wildcards.
package pathtree

import (
	"errors"
	"strconv"
	"strings"
)

type Node struct {
	edges  map[string]Edge // the various path elements leading out of this node with wildcard elements.
	leaf   *Leaf           // if set, this is a terminal node for this leaf.
	star   *Leaf           // if set, this path ends in a star.
	leafs  int             // counter for # leafs in the tree
	parent *Edge           // two way traversing
}

type Leaf struct {
	Value     interface{} // the value associated with this node
	Wildcards []Wildcard  // the wildcard names, in order they appear in the path
	order     int         // the order this leaf was added
	parent    *Node       // two way traversing
	slashend  bool        // if the path ends with a slash
}

type Edge struct {
	node      *Node      // node for this wildcard element
	padding   [][]string // possible padding elements between each var
	wildcards []Wildcard // wildcard elements being the vars
	wildend   bool       // if it ends with a wildcard
	minorder  int        // minimum order value in this path
	parent    *Node      // two way traversing
}

type Wildcard struct {
	Name string // name of the wildcard
	Min  int    // min size (0 for none)
	Max  int    // max size (0 for none)
}

// New returns a new path tree.
func New() *Node {
	return &Node{edges: make(map[string]Edge)}
}

// Adds a new wildcard element to the node and returns the node
func (n *Node) addEdge(padding [][]string, wildcards []Wildcard, representation string, wildend bool, order int) *Node {
	element := Edge{node: New(), padding: padding, wildcards: wildcards, wildend: wildend, minorder: order, parent: n}
	element.node.parent = &element
	n.edges[representation] = element
	return element.node
}

// Add a path and its associated value to the tree.
//   - key must begin with "/"
//   - key must not duplicate any existing key.
// Returns an error if those conditions do not hold.
func (n *Node) Add(key string, val interface{}) (leaf *Leaf, err error) {
	if key[0] != '/' {
		return nil, errors.New("Path must begin with /")
	}
	n.leafs++
	elements, slashend := splitPath(key)
	return n.add(n.leafs, elements, nil, slashend, val)
}

func (n *Node) add(order int, elements []string, wildcards []Wildcard, slashend bool, val interface{}) (leaf *Leaf, err error) {
	// Create leaf at the end
	if len(elements) == 0 {
		if n.leaf != nil {
			return nil, errors.New("duplicate path")
		}
		n.leaf = &Leaf{
			order:     order,
			Value:     val,
			Wildcards: wildcards,
			parent:    n,
			slashend:  slashend,
		}
		return n.leaf, nil
	}

	var el string
	el, elements = elements[0], elements[1:]

	// Handle stars
	if len(el) > 0 && el[0] == '*' {
		if n.star != nil {
			return nil, errors.New("duplicate path")
		}
		n.star = &Leaf{
			order:     order,
			Value:     val,
			Wildcards: append(wildcards, Wildcard{el[1:], 0, 0}),
			parent:    n,
			slashend:  slashend,
		}
		return n.star, nil
	}

	// Handle wildcards
	// remove any ending wildcard charicter
	if (len(el) > 0) && (el[len(el)-1] == ';') {
		el = el[:len(el)-1]
	}
	parts := splitInput(el)
	variables := make([]Wildcard, len(parts)/2)
	paddings := make([][]string, len(variables)+len(parts)%2)

	// Split appart padding and variables (padding first even if empty)
	in := false
	key := 0
	for _, value := range parts {
		if in == false {
			paddings[key] = splitPad(value)
		} else {
			variables[key] = decodeWildcard(value)
			key++
		}
		in = !in
	}

	// Create string representation for map
	wildend := len(paddings) == len(variables)

	// Test if map contains representation else create it
	item, ok := n.edges[el]
	var node *Node
	if ok {
		node = item.node
		if item.minorder > order {
			item.minorder = order
		}
	} else {
		node = n.addEdge(paddings, variables, el, wildend, order)
	}

	return node.add(order, elements, append(wildcards, variables...), slashend, val)
}

// Find a given path. Any wildcards traversed along the way are expanded and
// returned, along with the value.
func (n *Node) Find(key string) (leaf *Leaf, expansions []string) {
	if len(key) == 0 || key[0] != '/' {
		return nil, nil
	}

	elements, _ := splitPath(key)
	return n.find(elements, nil)
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

	// Handle star
	if n.star != nil && (leaf == nil || leaf.order > n.star.order) {
		leaf = n.star
		expansions = append(exp, starExpansion)
	}

	// Handle wildards
	for _, value := range n.edges {
		// Only check if tree contrains lower order item
		if leaf != nil && leaf.order < value.minorder {
			continue
		}

		found := false
		variables := make([]string, 0, 0)
		input := el

		// Check all padding elements are present and exit at first failure
		for count, pads := range value.padding {
			for _, pad := range pads {
				pos := strings.Index(input, pad)

				if (pos == -1) || (count == 0 && pos > 0) {
					found = false
					continue
				}

				if count != 0 {
					item := &value.wildcards[count-1]
					if (item.Min != 0 && pos < item.Min) || (item.Max != 0 && pos > item.Max) {
						found = false
						continue
					} else {
						variables = append(variables, input[:pos])
					}
				}
				input = input[pos+len(pad):]
				found = true
				break
			}

			if !found {
				break
			}
		}

		if !found {
			continue
		}

		if value.wildend {
			item := &value.wildcards[len(value.wildcards)-1]
			if value.wildend && ((item.Min != 0 && len(input) < item.Min) || (item.Max != 0 && len(input) > item.Max)) {
				continue
			}
			variables = append(variables, input)
		}

		// Set leaf if it meets lower levels
		if testleaf, testexpansions := value.node.find(elements, append(exp, variables...)); testleaf != nil {
			if leaf == nil || leaf.order > testleaf.order {
				leaf, expansions = testleaf, testexpansions
			}
		}
	}

	return
}

// Reverse a given leaf into a path traversing up the tree. Any wildcards along
// the way are replaced using the variable map and unused elements are returned.
// err is nil on success, returns an array of missing wildcard elements not found
// in the variable map or an empty array if leaf is invalid.
func (n *Node) Reverse(leaf *Leaf, variables map[string]string) (path string, unused map[string]string, err []string) {
	if leaf == nil || leaf.parent == nil {
		return "", variables, make([]string, 0, 0)
	}

	return leaf.parent.reverse("", variables, nil, leaf.slashend)
}

func (n *Node) reverse(exp string, variables map[string]string, missed []string, slashend bool) (path string, unused map[string]string, err []string) {
	// Return if we have reached the end of a tree
	if n.parent == nil {
		if slashend {
			exp += "/"
		}
		return exp, variables, missed
	}

	// Generate edge path from padding and variables
	edge := n.parent
	var output string
	for key, value := range edge.wildcards {
		item, ok := variables[value.Name]
		if !ok || (value.Min != 0 && len(item) < value.Min) || (value.Max != 0 && len(item) > value.Max) {
			item = ""
			ok = false
			missed = append(missed, "["+strconv.Itoa(value.Min)+","+strconv.Itoa(value.Max)+"]"+value.Name)
		}

		output = output + edge.padding[key][0] + item

		if ok {
			delete(variables, value.Name)
		}
	}

	// Generate total output and add any final padding
	if edge.wildend {
		exp = "/" + output + exp
	} else {
		exp = "/" + output + edge.padding[len(edge.padding)-1][0] + exp
	}

	return edge.parent.reverse(exp, variables, missed, slashend)
}

func splitPath(key string) (parts []string, slashend bool) {
	elements := strings.Split(key, "/")
	slashend = false
	if elements[0] == "" {
		elements = elements[1:]
	}
	if elements[len(elements)-1] == "" {
		elements = elements[:len(elements)-1]
		slashend = true
	}
	return elements, slashend
}

func splitInput(s string) []string {
	if s == "" {
		return []string{s}
	}
	start := 0
	n := strings.Count(s, ":") + strings.Count(s, ";") + 1
	a := make([]string, n)
	na := 0
	in := false
	for i := 0; i+1 <= len(s) && na+1 < n; i++ {
		if (s[i] == ':' && in == false) || (s[i] == ';' && in == true) {
			a[na] = s[start:i]
			na++
			in = !in
			start = i + 1
		}
	}
	a[na] = s[start:]
	return a[0 : na+1]
}

func splitPad(s string) []string {
	if s == "" {
		return []string{s}
	}
	start := 0
	n := strings.Count(s, "|") + 1
	a := make([]string, n)
	na := 0
	for i := 0; i+1 <= len(s) && na+1 < n; i++ {
		if s[i] == '|' {
			a[na] = s[start:i]
			na++
			start = i + 1
		}
	}
	a[na] = s[start:]
	return a[0 : na+1]
}

func decodeWildcard(s string) Wildcard {
	if s[0] == '[' && len(s) > 2 {
		min, max := 0, 0
		var err error

		pos1 := strings.Index(s[1:], ",")
		pos2 := strings.Index(s[1:], "]")

		if pos1 == -1 && pos2 > 0 && len(s) > pos2+2 {
			if min, err = strconv.Atoi(s[1 : pos2+1]); err == nil {
				return Wildcard{s[pos2+2:], min, min}
			}
		}

		if pos1 > 0 && pos2 > pos1+1 && len(s) > pos2+2 {
			if min, err = strconv.Atoi(s[1 : pos1+1]); err == nil {
				if max, err = strconv.Atoi(s[pos1+2 : pos2+1]); err == nil {
					return Wildcard{s[pos2+2:], min, max}
				}
			}
		}
	}

	return Wildcard{s, 0, 0}
}
