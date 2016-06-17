package trout

import (
	"net/http"
	"sort"
	"sync"
)

type trie struct {
	branch *branch
	sync.RWMutex
}

// match takes input as a slice of string keys, and attempts
// to find a path through the trie that satisfies those keys.
// If successful, the path will be returned as offsets for
// the children of the root branch of the trie.
//
// if a suitable path is found, the boolean returned will be
// true. Otherwise, it will be false.
func (t *trie) match(input []string) ([]int, bool) {
	if t.branch == nil {
		t.branch = &branch{}
	}
	b := t.branch
	var path []int
	offset := 0
	num := len(input)
	for i := 0; i < num; i++ {
		// if we're on a nil branch, we're at the end of our line
		if b == nil {
			return path, false
		}
		offset = pickNextBranch(b, offset, input[i])
		if offset == -1 {
			if len(path) == 0 {
				// we're at the very first branch, bail out
				// there's nowhere left to look
				return path, false
			}
			// no match, backup
			path, offset = backup(path)
			offset = offset + 1 // the branch that we chose last led us to a dead end, pick the next one
			b = b.parent        // we need to pick the next child of our parent, the one right after the one we just searched
			i = i - 2           // and we need to search it for the word we had matched
			// (we subtract two to account for the +1 completing the loop adds and the +1 from matching it the first time)
			// basically, we're saying "redo that match, but without this path"
			continue
		}
		path = append(path, offset)
		b = b.children[offset]
		offset = 0
	}
	return path, true
}

// find the first child of branch after offset that matches input
func pickNextBranch(b *branch, offset int, input string) int {
	count := len(b.children)
	for i := offset; i < count; i++ {
		if b.children[i].check(input) {
			return i
		}
	}
	return -1
}

// return the path before our most recent match, and the slice position
// of the branch that matched it
func backup(path []int) ([]int, int) {
	last := len(path) - 1
	pos := path[last]
	path = path[:last]
	return path, pos
}

type branch struct {
	parent   *branch
	children []*branch
	key      string
	isParam  bool
	methods  map[string]http.Handler
}

// sort our branches: lexically by key, with ties being determined by
// "parameters sort to the end" rules.
func (b *branch) Less(i, j int) bool {
	if b.children[i].key == b.children[j].key {
		return b.children[j].isParam && !b.children[i].isParam
	}
	return b.children[i].key < b.children[j].key
}

func (b *branch) Len() int {
	return len(b.children)
}

func (b *branch) Swap(i, j int) {
	b.children[i], b.children[j] = b.children[j], b.children[i]
}

// insert a new branch as a child of the branch this is called on. Key
// will be used to match the branch, and if param is true, any input
// will be considered to match the branch. For params, key is used as
// the variable name for the value.
func (b *branch) addChild(key string, param bool) *branch {
	child := &branch{key: key, parent: b, isParam: param}
	if b.children == nil {
		b.children = []*branch{child}
		return child
	}
	b.children = append(b.children, child)
	sort.Sort(b)
	return child
}

// return true if the input should be considered a match for the branch
func (b *branch) check(input string) bool {
	if b.isParam && b.key != "" {
		return true
	}
	if b.key == input {
		return true
	}
	return false
}

// set the http.Handler for a given method on the current branch
func (b *branch) setHandler(method string, handler http.Handler) {
	if b.methods == nil {
		b.methods = map[string]http.Handler{}
	}
	b.methods[method] = handler
}
