package trout

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const catchAllMethod = "*"

type trie struct {
	branch *branch
	sync.RWMutex
}

type branch struct {
	parent           *branch
	staticChildren   map[string]*branch
	wildcardChildren []*branch
	key              string
	isParam          bool
	methods          map[string]http.Handler
	depth            int
	priority         float64
}

type candidateBranch struct {
	b             *branch
	matchedPieces int
}

func (t trie) debugWalk(input []string, indentLevel string, root *branch) []string {
	key := root.key
	if root.isParam {
		key = "{" + key + "}"
	}
	input = append(input, indentLevel+"/"+key+"\t["+strconv.FormatFloat(root.priority, 'f', 4, 64)+"]")
	for method, h := range root.methods {
		input = append(input, fmt.Sprintf("%s\t- %s: %s", indentLevel, method, h))
	}
	for _, child := range root.staticChildren {
		input = t.debugWalk(input, indentLevel+"\t", child)
	}
	for _, child := range root.wildcardChildren {
		input = t.debugWalk(input, indentLevel+"\t", child)
	}
	return input
}

func (t trie) debug() string {
	return strings.Join(t.debugWalk([]string{}, "", t.branch), "\n")
}

// match takes input as a slice of string keys, and attempts
// to find a path through the trie that satisfies those keys.
// If successful, the path will be returned as keys for
// the children used, starting at the root branch of the trie.
//
// if a suitable path is found, the boolean returned will be
// true. Otherwise, it will be false.
func (t *trie) match(input []string, method string) *branch {
	if t.branch == nil {
		t.branch = &branch{}
	}
	// candidates tracks the paths that we think could be viable
	candidates := []candidateBranch{{b: t.branch}}
	// candidateLeafs tracks the paths that turned out to be viable
	var candidateLeafs []*branch
	for len(candidates) > 0 {
		// pop off the next candidate
		candidate := candidates[len(candidates)-1]
		candidates = candidates[:len(candidates)-1]

		// follow it as far as we can
		results, matchedPieces := candidate.b.match(input[candidate.matchedPieces:])

		// if there are no results, bail
		if len(results) == 0 {
			continue
		}

		// if we have unmatched input, add the results to the candidates and carry on
		if matchedPieces+candidate.matchedPieces < len(input) {
			for _, result := range results {
				candidates = append(candidates, candidateBranch{b: result, matchedPieces: candidate.matchedPieces + matchedPieces})
			}
			continue
		}

		// if we have no unmatched input, they're candidate leafs
		candidateLeafs = append(candidateLeafs, results...)
	}

	var match *branch
	var matchedLeafs []*branch
	for _, candidate := range candidateLeafs {
		// prematurely set a result so if we don't get a method
		// match, we can return a 405
		match = candidate

		// check if the candidates can handle the method we're looking for
		if _, ok := candidate.methods[method]; ok {
			matchedLeafs = append(matchedLeafs, candidate)
			continue
		}
		if _, ok := candidate.methods[catchAllMethod]; ok {
			matchedLeafs = append(matchedLeafs, candidate)
			continue
		}
	}

	var highScore float64
	for _, result := range matchedLeafs {
		if result.priority > highScore {
			highScore = result.priority
			match = result
		}
	}

	return match
}

func (b *branch) match(input []string) ([]*branch, int) {
	var matches []*branch
	if len(input) < 1 {
		return matches, 0
	}
	// do we have a precise match for this portion of the route?
	child, ok := b.staticChildren[input[0]]
	if ok {
		matches = append(matches, child)
	}

	// do we have a wildcard match for this portion of the route?
	for _, candidate := range b.wildcardChildren {
		if candidate.check(input[0], len(input) == 1) {
			matches = append(matches, candidate)
		}
	}
	var inputMatched int
	if len(matches) > 0 {
		inputMatched = 1
	}
	return matches, inputMatched
}

// insert a new branch as a child of the branch this is called on. Key
// will be used to match the branch, and if param is true, any input
// will be considered to match the branch. For params, key is used as
// the variable name for the value.
func (b *branch) addChild(key string, param bool) *branch {
	child := &branch{key: key, parent: b, isParam: param, depth: b.depth + 1}
	child.priority = child.score()
	if param {
		b.wildcardChildren = append(b.wildcardChildren, child)
		return child
	}
	if b.staticChildren == nil {
		b.staticChildren = map[string]*branch{key: child}
		return child
	}
	b.staticChildren[key] = child
	return child
}

// return true if the input should be considered a match for the branch
// isLast is true if this is the last piece of input
func (b *branch) check(input string, isLast bool) bool {
	// if it's the last piece of input and we don't have any handlers
	// this isn't a match
	if isLast && len(b.methods) == 0 {
		return false
	}
	// params match everything!
	if b.isParam && b.key != "" {
		return true
	}
	// last resort -- is it an exact match?
	if b.key == input {
		return true
	}
	return false
}

func (b *branch) vars(input []string) map[string][]string {
	bvars := map[string][]string{}
	if b.parent != nil {
		parentVars := b.parent.vars(input[:len(input)-1])
		for k, v := range parentVars {
			bvars[k] = append(bvars[k], v...)
		}
	}
	if b.isParam {
		bvars[b.key] = append(bvars[b.key], input[len(input)-1])
	}
	return bvars
}

func (b *branch) score() float64 {
	score := 2.0
	if b.isParam {
		score = 1.0
	}
	score = score / float64(b.depth+1)
	if b.parent != nil {
		score += b.parent.priority
	}
	return score
}

func (b *branch) pathString() string {
	var res string
	if b.parent != nil {
		res = b.parent.pathString()
	}
	res += "/"
	if b.isParam {
		res += "{"
	}
	res += b.key
	if b.isParam {
		res += "}"
	}
	return res
}

// set the http.Handler for a given method on the current branch
func (b *branch) setHandler(method string, handler http.Handler) {
	if b.methods == nil {
		b.methods = map[string]http.Handler{}
	}
	b.methods[method] = handler
}
