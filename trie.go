package trout

import (
	"net/http"
	"sync"
)

// key represents the text inside a node, the piece of the URL that the node is
// representing.
type key struct {
	// value is the text itself
	value string
	// dynamic signifies whether the text is a placeholder for another
	// value or is what we're trying to match against
	dynamic bool
	// prefix signifies whether a key should be considered a prefix
	// matcher, matching all subsequent keys
	prefix bool
	// nul signifies whether a key should be considered a null key, used to
	// terminate an endpoint, or whether other keys follow it
	nul bool
}

// equals returns whether `k` should be considered equivalent to `other` or
// not.
func (k key) equals(other key) bool {
	if k.value != other.value {
		return false
	}
	if k.dynamic != other.dynamic {
		return false
	}
	if k.prefix != other.prefix {
		return false
	}
	if k.nul != other.nul {
		return false
	}
	return true
}

// String fulfills the Stringer interface, returning a representation of `k`
// that can be used as a string. nul keys will be represented by "{::NULL:}",
// while dynamic keys will be surrounded by "{" and "}" and prefix keys will
// end in "::prefix"}. Static keys will be displayed as normal.
func (k key) String() string {
	if k.nul {
		return "{::NULL::}"
	}
	res := ""
	if k.dynamic {
		res += "{"
	}
	res += k.value
	if k.prefix {
		res += "::prefix"
	}
	if k.dynamic {
		res += "}"
	}
	return res
}

// node represents a single part of an endpoint or URL within our router. If a
// URL is split by /, each piece is a node, and each piece is the child of the
// node that came before it. This allows us to build a trie of these pieces
// that can efficiently match URLs even when a large number of patterns exists.
type node struct {
	value        key
	term         bool
	depth        int
	parent       *node
	terminator   *node
	children     map[string]*node
	wildChildren []*node
	methods      map[string]http.Handler
	middleware   map[string][]func(http.Handler) http.Handler
}

// newChild inserts a new child node under `n` and
// returns the child.
func (n *node) newChild(value key, term bool) *node {
	newNode := &node{
		value:      value,
		term:       term,
		depth:      n.depth + 1,
		children:   map[string]*node{},
		methods:    map[string]http.Handler{},
		middleware: map[string][]func(http.Handler) http.Handler{},
		parent:     n,
	}
	if value.dynamic {
		n.wildChildren = append(n.wildChildren, newNode)
	} else if term {
		n.terminator = newNode
	} else {
		n.children[value.value] = newNode
	}
	return newNode
}

// trie is the data structure holding all our nodes. It will be used as the
// main data structure of our router.
type trie struct {
	root *node
	sync.RWMutex
}

// add inserts the nodes necessary to construct the supplied path.
func (t *trie) add(path []key, methods map[string]http.Handler) *node {
	n := t.root

	t.Lock()
	defer t.Unlock()

	for _, piece := range path {
		var match bool
		if !piece.dynamic {
			if static, ok := n.children[piece.value]; ok {
				n = static
				match = true
			}
		} else {
			for _, wild := range n.wildChildren {
				if wild.value.equals(piece) {
					n = wild
					match = true
					break
				}
			}
		}
		if !match {
			n = n.newChild(piece, false)
		}
	}
	if n.terminator != nil {
		return n.terminator
	}
	n = n.newChild(key{nul: true}, true)
	return n
}

// findNodes runs the findNodes function on the root node of `t`
// with concurrency safety.
func (t *trie) findNodes(path []string) []*node {
	t.RLock()
	defer t.RUnlock()
	return findNodes(t.root, path)
}

// findNodes returns all terminating nodes that could match the
// supplied input. Because of wildcards and prefixes, there may
// be multiple results, and it's up to the caller to determine
// which is best.
func findNodes(n *node, path []string) []*node {
	if n == nil {
		return nil
	}
	var results []*node
	if n.value.prefix {
		return []*node{n}
	}
	var nextPath []string
	if len(path) > 1 {
		nextPath = path[1:]
	}
	static, ok := n.children[path[0]]
	if ok {
		if len(nextPath) < 1 {
			if static.terminator != nil {
				results = append(results, static)
			}
		} else {
			staticResults := findNodes(static, nextPath)
			if staticResults != nil {
				results = append(results, staticResults...)
			}
		}
	}
	for _, wild := range n.wildChildren {
		if len(nextPath) < 1 {
			if wild.terminator != nil {
				results = append(results, wild)
			}
			continue
		}
		wildResults := findNodes(wild, nextPath)
		if wildResults != nil {
			results = append(results, wildResults...)
		}
	}
	return results
}

// vars runs the vars function with concurrency safety as long
// as `n` is a descendent of the root node of `t`.
func (t *trie) vars(n *node, input []string) map[string][]string {
	t.RLock()
	defer t.RUnlock()
	return vars(n, input)
}

// vars returns a mapping of dynamic path key names to
// the values assigned to them. Values assigned to them
// should be in the order they appear in the input when
// key names are reused within a single path.
func vars(n *node, input []string) map[string][]string {
	if len(input) < 1 {
		return map[string][]string{}
	}
	if n == nil {
		return map[string][]string{}
	}
	if n.value.nul {
		n = n.parent
	}
	if n == nil {
		return map[string][]string{}
	}
	params := vars(n.parent, input[:len(input)-1])
	if n.value.dynamic {
		params[n.value.value] = append(params[n.value.value], input[len(input)-1])
	}
	return params
}

// pathString runs the pathString function with concurrency
// safety as long as `n` is a descendent of the root node of
// `t`.
func (t *trie) pathString(n *node) string {
	t.RLock()
	defer t.RUnlock()
	return pathString(n)
}

// pathString returns a representation of the path to
// the passed node.
func pathString(n *node) string {
	if n == nil {
		return ""
	}
	res := pathString(n.parent)
	if n.value.nul || n.value.String() == "" {
		return res
	}
	res += "/" + n.value.String()
	return res
}
