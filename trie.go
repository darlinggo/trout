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

func (t *trie) match(input []string) ([]int, bool) {
	if t.branch == nil {
		t.branch = &branch{}
	}
	b := t.branch
	path := []int{}
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
				// can't find it, bail
				return path, false
			}
			// no match, backup
			path, offset = backup(path)
			offset = offset + 1 // we want the next index from the previously matched one
			b = b.parent        // we need to be picking a branch from our parent, again
			i = i - 2           // back up to the choice before the one that got us here
		} else {
			path = append(path, offset)
			b = b.children[offset]
			offset = 0
		}
	}
	return path, true
}

type branch struct {
	parent   *branch
	children []*branch
	key      string
	isParam  bool
	methods  map[string]http.Handler
}

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

func (b *branch) check(input string) bool {
	if b.isParam && b.key != "" {
		return true
	}
	if b.key == input {
		return true
	}
	return false
}

func (b *branch) setHandler(method string, handler http.Handler) {
	if b.methods == nil {
		b.methods = map[string]http.Handler{}
	}
	b.methods[method] = handler
}

func pickNextBranch(b *branch, offset int, input string) int {
	count := len(b.children)
	for i := offset; i < count; i++ {
		if b.children[i].check(input) {
			return i
		}
	}
	return -1
}

func backup(path []int) ([]int, int) {
	last := len(path) - 1
	pos := path[last]
	path = path[:last]
	return path, pos
}
