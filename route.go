package trout

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	default404Handler = http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
		return
	}))
	default405Handler = http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", strings.Join(r.Header[http.CanonicalHeaderKey("Trout-Methods")], ", "))
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("405 Method Not Allowed"))
		return
	}))
)

// RequestVars returns easy-to-access mappings of parameters to values for URL templates. Any {parameter} in
// your URL template will be available in the returned Header as a slice of strings, one for each instance of
// the {parameter}. In the case of a parameter name being used more than once in the same URL template, the
// values will be in the slice in the order they appeared in the template.
//
// Values can easily be accessed by using the .Get() method of the returned Header, though to access multiple
// values, they must be accessed through the map. All parameters use http.CanonicalHeaderKey for their formatting.
// When using .Get(), the parameter name will be transformed automatically. When utilising the Header as a map,
// the parameter name needs to have http.CanonicalHeaderKey applied manually.
func RequestVars(r *http.Request) http.Header {
	res := http.Header{}
	for h, v := range r.Header {
		stripped := strings.TrimPrefix(h, http.CanonicalHeaderKey("Trout-Param-"))
		if stripped != h {
			res[stripped] = v
		}
	}
	return res
}

// Router defines a set of Endpoints that map requests to the http.Handlers. The http.Handler assigned to
// Handle404, if set, will be called when no Endpoint matches the current request. The http.Handler assigned
// to Handle405, if set, will be called when an Endpoint matches the current request, but has no http.Handler
// set for the HTTP method that the request used. Should either of these properties be unset, a default
// http.Handler will be used.
//
// The Router type is safe for use with empty values, but makes no attempt at concurrency-safety in adding
// Endpoints or in setting properties. It should also be noted that the adding Endpoints while simultaneously
// routing requests will lead to undefined and (almost certainly) undesirable behaviour. Routers are intended
// to be initialised with a set of Endpoints, and then start serving requests. Using them outside of this use
// case is unsupported.
type Router struct {
	t         *trie
	Handle404 http.Handler
	Handle405 http.Handler
	prefix    string
}

func (router *Router) serve404(w http.ResponseWriter, r *http.Request, t time.Time) {
	h := default404Handler
	if router.Handle404 != nil {
		h = router.Handle404
	}
	r.Header.Set("Trout-Timer", strconv.FormatInt(time.Now().Sub(t).Nanoseconds(), 10))
	h.ServeHTTP(w, r)
}

func (router *Router) serve405(w http.ResponseWriter, r *http.Request, t time.Time) {
	h := default405Handler
	if router.Handle405 != nil {
		h = router.Handle405
	}
	r.Header.Set("Trout-Timer", strconv.FormatInt(time.Now().Sub(t).Nanoseconds(), 10))
	h.ServeHTTP(w, r)
}

// getHandler returns the handler, the key for that handler, the map of values for any parameters
// in that handler, and the list of methods that handler is equipped to respond to.
func (router Router) getHandler(pieces []string, method string) (http.Handler, string, map[string][]string, []string) {
	router.t.RLock()
	defer router.t.RUnlock()
	branches := make([]*branch, len(pieces))
	path, ok := router.t.match(pieces)
	if !ok {
		return nil, "", nil, nil
	}
	b := router.t.branch
	for i, pos := range path {
		b = b.children[pos]
		branches[i] = b
	}
	v := vars(branches, pieces)
	ms := make([]string, 0, len(b.methods))
	for m := range b.methods {
		ms = append(ms, m)
	}
	h := b.methods[""]
	if han, ok := b.methods[method]; ok {
		h = han
	}
	match := pattern(branches)
	return h, match, v, ms
}

// ServeHTTP serves the request by matching it to a Handler.
func (router Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if router.t == nil {
		router.serve404(w, r, start)
		return
	}
	u := strings.TrimPrefix(r.URL.Path, router.prefix)
	pieces := strings.Split(strings.ToLower(strings.Trim(u, "/")), "/")
	h, match, params, methods := router.getHandler(pieces, r.Method)
	if h == nil {
		if len(methods) < 1 {
			router.serve404(w, r, start)
			return
		}
		router.serve405(w, r, start)
		return
	}
	for key, vals := range params {
		r.Header[http.CanonicalHeaderKey("Trout-Param-"+key)] = vals
	}
	r.Header[http.CanonicalHeaderKey("Trout-Methods")] = methods
	r.Header.Set("Trout-Pattern", match)
	r.Header.Set("Trout-Timer", strconv.FormatInt(time.Now().Sub(start).Nanoseconds(), 10))
	h.ServeHTTP(w, r)
}

// SetPrefix sets a string prefix for the Router that won't be taken into account when matching Endpoints.
// This is usually set to whatever path is associated with the http.Handler serving the Router.
func (router *Router) SetPrefix(prefix string) {
	router.prefix = prefix
}

// Endpoint defines a new Endpoint on the Router. The Endpoint should be a URL template, using curly braces
// to denote parameters that should be filled at runtime. For example, `{id}` denotes a parameter named `id`
// that should be filled with whatever the request has in that space.
//
// Parameters are always `/`-separated strings. There is no support for regular expressions or other limitations
// on what may be in those strings. A parameter is simply defined as "whatever is between these two / characters".
func (router *Router) Endpoint(e string) *Endpoint {
	e = strings.Trim(e, "/")
	e = strings.ToLower(e)
	pieces := strings.Split(e, "/")
	if router.t == nil {
		router.t = &trie{}
	}
	router.t.Lock()
	defer router.t.Unlock()
	if router.t.branch == nil {
		router.t.branch = &branch{
			parent:   nil,
			children: []*branch{},
			key:      "",
			isParam:  false,
			methods:  map[string]http.Handler{},
		}
	}
	// find the path that gets us closest to our destination
	// so we can build off that
	closest := findClosestLeaf(pieces, router.t.branch)
	b := router.t.branch
	for _, pos := range closest {
		b = b.children[pos]
	}
	// if we already have the entire path, just return the
	// last branch and call it a day
	if len(closest) == len(pieces) {
		return (*Endpoint)(b)
	}
	// starting from the last known branch, add branches until
	// we've got a branch for each piece of the input
	offset := len(closest)
	for i := offset; i < len(pieces); i++ {
		piece := pieces[i]
		var isParam bool
		if len(piece) > 0 && piece[0:1] == "{" && piece[len(piece)-1:] == "}" {
			isParam = true
			piece = piece[1 : len(piece)-1]
		}
		b = b.addChild(piece, isParam)
	}
	return (*Endpoint)(b)
}

// given a list of branches and a list of pieces matching those
// branches, return a map of parameters. Keys will be the key for
// the branches where isParam is true, and values will be the piece
// that matched that key. Because keys can be repeated, values are
// always slices, and are always returned in the order the pieces
// occurred in.
func vars(path []*branch, pieces []string) map[string][]string {
	v := map[string][]string{}
	for pos, p := range path {
		if !p.isParam {
			continue
		}
		_, ok := v[p.key]
		if !ok {
			v[p.key] = []string{pieces[pos]}
			continue
		}
		v[p.key] = append(v[p.key], pieces[pos])
	}
	return v
}

// pattern returns the raw template for a given set of branches,
// as it would've been entered to create the branch.
func pattern(path []*branch) string {
	results := []string{}
	for _, p := range path {
		key := p.key
		if p.isParam {
			key = "{" + key + "}"
		}
		results = append(results, key)
	}
	return strings.Join(results, "/")
}

// find the longest chain of branches that match the pieces
// supplied, and return the positions of those branches in
// multi-dimensional array format. Pieces will be in template
// format, and this is used to find shared ancestors, not to
// route.
func findClosestLeaf(pieces []string, b *branch) []int {
	offset := 0
	path := []int{}
	longest := []int{}
	num := len(pieces)
	for i := 0; i < num; i++ {
		piece := pieces[i]
		var isParam bool
		if len(piece) > 0 && piece[0:1] == "{" && piece[len(piece)-1:] == "}" {
			isParam = true
			piece = piece[1 : len(piece)-1]
		}
		offset = pickNextRoute(b, offset, piece, isParam)
		if offset == -1 {
			if len(path) == 0 {
				// exhausted our options, bail
				break
			}
			// no match, maybe save this and backup
			if len(path) > len(longest) {
				longest = append([]int{}, path...) // copy them over so they don't get modified
			}
			path, offset = backup(path)
			offset = offset + 1
			b = b.parent
			i = i - 2
			continue
		}
		path = append(path, offset)
		b = b.children[offset]
		offset = 0
	}
	if len(longest) < len(path) {
		// defensively copy
		longest = append([]int{}, path...)
	}
	return longest
}

// return true if the input should be considered a match for the branch
// the same as branch.check, with the wrinkle that param branches only
// match when variable is true.
//
// this is used when trying to add to the trie, and we need to find the
// best route to add to.
func pickNextRoute(b *branch, offset int, input string, variable bool) int {
	count := len(b.children)
	for i := offset; i < count; i++ {
		if b.children[i].key == input && b.children[i].isParam == variable {
			return i
		}
	}
	return -1
}

// Endpoint defines a single URL template that requests can be matched against. It uses
// URL parameters to accept variables in the URL structure and make them available to
// the Handlers associated with the Endpoint.
type Endpoint branch

// Handler associates the passed http.Handler with the Endpoint. This http.Handler will be
// used for all requests, regardless of the HTTP method they are using, unless overridden by
// the Methods method. Endpoints without a http.Handler associated with them will not be
// considered matches for requests, unless the request was made using an HTTP method that the
// Endpoint has an http.Handler mapped to.
func (e *Endpoint) Handler(h http.Handler) {
	(*branch)(e).setHandler("", h)
}

// Methods returns a Methods object that will enable the mapping of the passed HTTP
// request methods to a Methods object. On its own, this function does not modify anything. It
// should, instead, be used as a friendly shorthand to get to the Methods.Handler method.
func (e *Endpoint) Methods(m ...string) Methods {
	return Methods{
		e: e,
		m: m,
	}
}

// Methods defines a pairing of an Endpoint to the HTTP request methods that should be mapped to
// specific http.Handlers. Its sole purpose is to enable the Methods.Handler method.
type Methods struct {
	e *Endpoint
	m []string
}

// Handler maps a Methods object to a specific http.Handler. This overrides the http.Handler
// associated with the Endpoint to only handle specific HTTP method(s).
func (m Methods) Handler(h http.Handler) {
	b := (*branch)(m.e)
	for _, method := range m.m {
		b.setHandler(method, h)
	}
}
