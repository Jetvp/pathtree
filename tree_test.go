package pathtree

import (
	"fmt"
	"reflect"
	"testing"
)

func TestColon(t *testing.T) {
	n := New()

	n.Add("/P:first;/U:[4,10]second", 1)
	n.Add("/P:first;/U:[3]second", 2)
	n.Add("/P:first;/U:second", 3)
	n.Add("/N:first/:second", 4)
	n.Add("/P:first", 5)
	n.Add("/:first/:second;/", 6)
	n.Add("/Archive_:first;_all", 7)
	n.Add("/Archive_:first;_:year;", 8)
	n.Add("/User_:first;//:second;.:third;", 9)
	n.Add("/:first;.:second", 10)
	n.Add("/:first", 11)
	n.Add("/", 12)

	found(t, n, "/", nil, 12)
	found(t, n, "/a", []string{"a"}, 11)
	found(t, n, "/a/", []string{"a"}, 11)
	found(t, n, "/a/b", []string{"a", "b"}, 6)
	found(t, n, "/a/b/", []string{"a", "b"}, 6)
	found(t, n, "/Pa/Ub/", []string{"a", "b"}, 3)
	found(t, n, "/Pa/b/", []string{"Pa", "b"}, 6)
	found(t, n, "/Na/Ub/", []string{"a", "Ub"}, 4)
	found(t, n, "/Na/b/", []string{"a", "b"}, 4)
	found(t, n, "/Pa", []string{"a"}, 5)
	found(t, n, "/Na", []string{"Na"}, 11)
	found(t, n, "/Archive_March_all", []string{"March"}, 7)
	found(t, n, "/Archive_March_2013", []string{"March", "2013"}, 8)
	found(t, n, "/User_Freddy//20130502_0001.jpg", []string{"Freddy", "20130502_0001", "jpg"}, 9)
	found(t, n, "/map.xml", []string{"map", "xml"}, 10)

	notfound(t, n, "/a/b/c")
	notfound(t, n, "/Pa/Ub/c")
}

func TestStar(t *testing.T) {
	n := New()

	n.Add("/first/second/*star", 1)
	n.Add("/:first/*star/", 2)
	n.Add("/*star", 3)
	n.Add("/", 4)

	found(t, n, "/", nil, 4)
	found(t, n, "/a", []string{"a"}, 3)
	found(t, n, "/a/", []string{"a"}, 3)
	found(t, n, "/a/b", []string{"a", "b"}, 2)
	found(t, n, "/a/b/", []string{"a", "b"}, 2)
	found(t, n, "/a/b/c", []string{"a", "b/c"}, 2)
	found(t, n, "/a/b/c/", []string{"a", "b/c"}, 2)
	found(t, n, "/a/b/c/d", []string{"a", "b/c/d"}, 2)
	found(t, n, "/first/second", []string{"first", "second"}, 2)
	found(t, n, "/first/second/", []string{"first", "second"}, 2)
	found(t, n, "/first/second/third", []string{"third"}, 1)
}

func TestMixedTree(t *testing.T) {
	n := New()

	n.Add("/", 0)
	n.Add("/path/to/nowhere", 1)
	n.Add("/path/:i/nowhere", 2)
	n.Add("/:id/to/nowhere", 3)
	n.Add("/:a/:b", 4)
	n.Add("/not/found", 5)
	n.Add("/is:id;really/found/now", 6)

	found(t, n, "/", nil, 0)
	found(t, n, "/path/to/nowhere", nil, 1)
	found(t, n, "/path/to/nowhere/", nil, 1)
	found(t, n, "/path/from/nowhere", []string{"from"}, 2)
	found(t, n, "/walk/to/nowhere", []string{"walk"}, 3)
	found(t, n, "/path/to/", []string{"path", "to"}, 4)
	found(t, n, "/path/to", []string{"path", "to"}, 4)
	found(t, n, "/not/found", []string{"not", "found"}, 4)
	found(t, n, "/isherereally/found/now", []string{"here"}, 6)
	found(t, n, "/isreally/found/now", []string{""}, 6)
	found(t, n, "//now", []string{"", "now"}, 4)
	notfound(t, n, "/path/to/somewhere")
	notfound(t, n, "/path/to/nowhere/else")
	notfound(t, n, "/path")
	notfound(t, n, "/path/")

	notfound(t, n, "")
	notfound(t, n, "xyz")
	notfound(t, n, "/path//to/nowhere")
}

func TestReverse(t *testing.T) {
	n := New()

	l1, _ := n.Add("/P:first;/U:second/", 1)
	l2, _ := n.Add("/Archive_:first;_all", 2)
	l3, _ := n.Add("/Archive_:first;_:[2,4]year;", 3)
	l4, _ := n.Add("/User_:first;//:second;.:third;", 4)
	l5, _ := n.Add("/", 5)

	if l1.parent != nil {
		reverse(t, n, l1, map[string]string{"first": "a"}, "/Pa/U/", map[string]string{}, []string{"[0,0]second"})
	} else {
		t.Errorf("Error generating reverse test leaf 1")
	}

	if l2.parent != nil {
		reverse(t, n, l2, map[string]string{"first": "March", "year": "2013"}, "/Archive_March_all", map[string]string{"year": "2013"}, nil)
	} else {
		t.Errorf("Error generating reverse test leaf 2")
	}

	if l3.parent != nil {
		reverse(t, n, l3, map[string]string{"first": "March", "year": "2013"}, "/Archive_March_2013", map[string]string{}, nil)
		reverse(t, n, l3, map[string]string{"first": "March", "year": "1"}, "/Archive_March_", map[string]string{"year": "1"}, []string{"[2,4]year"})
		reverse(t, n, l3, map[string]string{"first": "March", "year": "11023"}, "/Archive_March_", map[string]string{"year": "11023"}, []string{"[2,4]year"})
	} else {
		t.Errorf("Error generating reverse test leaf 3")
	}

	if l4.parent != nil {
		reverse(t, n, l4, map[string]string{"first": "Freddy", "second": "index", "third": "html", "fourth": "arg=build"}, "/User_Freddy//index.html", map[string]string{"fourth": "arg=build"}, nil)
	} else {
		t.Errorf("Error generating reverse test leaf 4")
	}

	if l5.parent != nil {
		reverse(t, n, l5, map[string]string{}, "/", map[string]string{}, nil)
	} else {
		t.Errorf("Error generating reverse test leaf 5")
	}
}

func BenchmarkTree100(b *testing.B) {
	n := New()
	n.Add("/", "root")

	// Exact matches
	for i := 0; i < 100; i++ {
		depth := i%5 + 1
		key := ""
		for j := 0; j < depth-1; j++ {
			key += fmt.Sprintf("/dir%d", j)
		}
		key += fmt.Sprintf("/resource%d", i)
		n.Add(key, "literal")
		// b.Logf("Adding %s", key)
	}

	// Wildcards at each level if no exact matches work.
	for i := 0; i < 5; i++ {
		var key string
		for j := 0; j < i; j++ {
			key += fmt.Sprintf("/dir%d", j)
		}
		key += "/:var"
		n.Add(key, "var")
		// b.Logf("Adding %s", key)
	}

	n.Add("/public/*filepath", "static")
	// b.Logf("Adding /public/*filepath")

	queries := map[string]string{
		"/": "root",
		"/dir0/dir1/dir2/dir3/resource4":    "literal",
		"/dir0/dir1/resource97":             "literal",
		"/dir0/variable":                    "var",
		"/dir0/dir1/dir2/dir3/variable":     "var",
		"/public/stylesheets/main.css":      "static",
		"/public/images/icons/an-image.png": "static",
	}

	for query, answer := range queries {
		leaf, _ := n.Find(query)
		if leaf == nil {
			b.Errorf("Failed to find leaf for querY %s", query)
			return
		}
		if leaf.Value.(string) != answer {
			b.Errorf("Incorrect answer for querY %s: expected: %s, actual: %s",
				query, answer, leaf.Value.(string))
			return
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N/len(queries); i++ {
		for k, _ := range queries {
			n.Find(k)
		}
	}
}

func notfound(t *testing.T, n *Node, p string) {
	if leaf, _ := n.Find(p); leaf != nil {
		t.Errorf("Should not have found: %s", p)
	}
}

func found(t *testing.T, n *Node, p string, expectedExpansions []string, val interface{}) {
	leaf, expansions := n.Find(p)
	if leaf == nil {
		t.Errorf("Didn't find: %s", p)
		return
	}
	if !reflect.DeepEqual(expansions, expectedExpansions) {
		t.Errorf("%s: Wildcard expansions (actual) %v != %v (expected)", p, expansions, expectedExpansions)
	}
	if leaf.Value != val {
		t.Errorf("%s: Value (actual) %v != %v (expected)", p, leaf.Value, val)
	}
}

func reverse(t *testing.T, n *Node, l *Leaf, vars map[string]string, path string, unused map[string]string, missing []string) {
	r_path, r_unused, r_missing := n.Reverse(l, vars)
	if r_path != path {
		t.Errorf("%s: Path (actual) %v != %v (expected)", l.Value, r_path, path)
	}
	if !reflect.DeepEqual(r_unused, unused) {
		t.Errorf("%s: Unused expansions (actual) %v != %v (expected)", l.Value, r_unused, unused)
	}
	if !reflect.DeepEqual(r_missing, missing) {
		t.Errorf("%s: Missing expansions (actual) %v != %v (expected)", l.Value, r_missing, missing)
	}
}
