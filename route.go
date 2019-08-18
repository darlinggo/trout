package trout

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	catchAllMethod = "*"
)

var (
	default404Handler = http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Page Not Found"))
		return
	}))
	default405Handler = http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", strings.Join(r.Header[http.CanonicalHeaderKey("Trout-Methods")], ", "))
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("405 Method Not Allowed"))
		return
	}))
)

// RequestVars returns easy-to-access mappings of parameters to values for URL
// templates. Any {parameter} in your URL template will be available in the
// returned Header as a slice of strings, one for each instance of the
// {parameter}. In the case of a parameter name being used more than once in
// the same URL template, the values will be in the slice in the order they
// appeared in the template.
//
// Values can easily be accessed by using the .Get() method of the returned
// Header, though to access multiple values, they must be accessed through the
// map. All parameters use http.CanonicalHeaderKey for their formatting.  When
// using .Get(), the parameter name will be transformed automatically. When
// utilising the Header as a map, the parameter name needs to have
// http.CanonicalHeaderKey applied manually.
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

// Router defines a set of Endpoints that map requests to the http.Handlers.
// The http.Handler assigned to Handle404, if set, will be called when no
// Endpoint matches the current request. The http.Handler assigned to
// Handle405, if set, will be called when an Endpoint matches the current
// request, but has no http.Handler set for the HTTP method that the request
// used. Should either of these properties be unset, a default http.Handler
// will be used.
//
// The Router type is safe for use with empty values, but makes no attempt at
// concurrency-safety in adding Endpoints or in setting properties. It should
// also be noted that the adding Endpoints while simultaneously routing
// requests will lead to undefined and (almost certainly) undesirable
// behaviour. Routers are intended to be initialised with a set of Endpoints,
// and then start serving requests. Using them outside of this use case is
// unsupported.
type Router struct {
	Handle404 http.Handler
	Handle405 http.Handler
	prefix    string
	trie      *trie
}

// get404 returns the http.Handler `router` should use when serving a 404 page
func (router *Router) get404() http.Handler {
	h := default404Handler
	if router.Handle404 != nil {
		h = router.Handle404
	}
	return h
}

// get405 returns the http.Handler `router` should use when serving a 405 page
func (router *Router) get405() http.Handler {
	h := default405Handler
	if router.Handle405 != nil {
		h = router.Handle405
	}
	return h
}

// route represents an endpoint match from the router, which should be served,
// and all the data needed to serve it.
//
// A route may not necessarily support the method a request used, but if it
// does not, no endpoint that uses those methods was matched. The methods
// property should be checked, and a 405 returned if a method is unsupported.
type route struct {
	// the http.Handler that needs to be served
	handler http.Handler
	// the pattern that was matched
	pattern string
	// the parsed parameters from the pattern
	params map[string][]string
	// the methods this endpoint can serve
	methods []string
}

// route uses the pieces of the request URL and the method of the request to
// find a route that should be used to serve the request.
//
// routes are chosen based on a weighting; see `scoreNode` for more details on
// the algorithm. routes that can support the supplied method are always chosen
// over routes that cannot; if a route that cannot support the supplied method
// is returned, it is safe to assume no route can.
func (router *Router) route(pieces []string, method string) *route {
	result := &route{}
	nodes := router.trie.findNodes(pieces)
	if nodes == nil || len(nodes) < 1 {
		return nil
	}
	node := pickNode(nodes, pieces, method)
	if node == nil {
		return nil
	}
	result.params = router.trie.vars(node, pieces)
	result.pattern = strings.TrimSuffix(router.prefix, "/") + router.trie.pathString(node)
	for method := range node.methods {
		result.methods = append(result.methods, method)
	}
	var ok bool
	result.handler, ok = node.methods[method]
	if !ok {
		result.handler = node.methods[catchAllMethod]
	}
	return result
}

// pickNode selects a node that has the highest score, according to
// `scoreNode`, to serve a request.
func pickNode(nodes []*node, pieces []string, method string) *node {
	var maxScore float64
	var bestNode *node
	for _, node := range nodes {
		if node == nil {
			continue
		}

		// if this node has no terminator/methods associated with it,
		// it can't be picked
		if node.terminator == nil {
			continue
		}

		score := scoreNode(node, pieces, 0)

		// any path that can serve the specified method should score
		// higher than paths that cannot
		if _, ok := node.terminator.methods[method]; !ok {
			score = score - math.Pow10(len(pieces)+1)
		}
		if bestNode == nil || score > maxScore {
			maxScore = score
			bestNode = node
		}
	}
	if bestNode == nil {
		return nil
	}
	return bestNode.terminator
}

// scoreNode assigns a raw score to how good a match a node is for a given set
// of pieces. A higher score is a better match.
//
// paths that have a 1:1 match between pieces and nodes should score higher
//     - this should be taken care of by having more nodes to score
// nodes that are dynamic should score lower than static matches
// nodes that are prefixes should score lower than static matches
// nodes that are prefixes should score lower than nodes that are dynamic
//     - this should be taken care of by having more nodes to score
// nodes earlier in the path should be worth more than nodes later in the path
func scoreNode(node *node, pieces []string, power int) float64 {
	var score float64
	if node.parent != nil {
		parPower := power + 1
		score = scoreNode(node.parent, pieces[:len(pieces)-1], parPower)
	}
	if node.value.nul {
		return score
	}
	myScore := 1
	if !node.value.dynamic && !node.value.prefix {
		myScore++
	}
	score += math.Pow10(power) * float64(myScore)
	return score
}

func (router *Router) getHandler(r *http.Request) http.Handler {
	// do our time tracking
	start := time.Now()
	defer func() {
		r.Header.Set("Trout-Timer", strconv.FormatInt(time.Now().Sub(start).Nanoseconds(), 10))
	}()

	// if our router is nil, everything's a 404
	if router.trie == nil {
		return router.get404()
	}

	// break the request URL down into pieces
	u := strings.TrimPrefix(r.URL.Path, router.prefix)
	pieces := strings.Split(strings.ToLower(strings.Trim(u, "/")), "/")

	// find the best match for our pieces and request method
	route := router.route(pieces, r.Method)

	// if we're nil, nothing was found, it's a 404
	if route == nil {
		return router.get404()
	}

	// if anything was found all, let's set our diagnostic headers
	r.Header[http.CanonicalHeaderKey("Trout-Methods")] = route.methods
	r.Header.Set("Trout-Pattern", route.pattern)
	for key, vals := range route.params {
		r.Header[http.CanonicalHeaderKey("Trout-Param-"+key)] = vals
	}

	// if no handler is set, it could be because there's no handler for
	// this endpoint, which we can safely assume is a 404
	if route.handler == nil {
		if len(route.methods) < 1 {
			return router.get404()
		}
		// but it could also mean that there's an endpoint that just
		// doesn't support the method we used, which is a 405
		return router.get405()
	}

	// after all that, if we still haven't found a problem, use the handler
	// we have
	return route.handler
}

// ServeHTTP finds the best handler for the request, using the 404 or 405
// handlers if necessary, and serves the request.
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.getHandler(r).ServeHTTP(w, r)
}

// SetPrefix sets a string prefix for the Router that won't be taken into
// account when matching Endpoints. This is usually set whenever the Router is
// not passed directly to http.ListenAndServe, and is sent through some sort of
// muxer first. It should be set to whatever string the muxer is using when
// passing requests to the Router.
//
// This function is not concurrency-safe; it should not be used while the
// Router is actively serving requests.
func (router *Router) SetPrefix(prefix string) {
	router.prefix = prefix
}

// Endpoint defines a single URL template that requests can be matched against.
// It is only valid to instantiate an Endpoint by calling `Router.Endpoint`.
// Endpoints, on their own, are only useful for calling their methods, as they
// don't do anything until an http.Handler is associated with them.
type Endpoint node

// Endpoint defines a new Endpoint on the Router. The Endpoint should be a URL
// template, using curly braces to denote parameters that should be filled at
// runtime. For example, `{id}` denotes a parameter named `id` that should be
// filled with whatever the request has in that space.
//
// Parameters are always `/`-separated strings. There is no support for regular
// expressions or other limitations on what may be in those strings. A
// parameter is simply defined as "whatever is between these two / characters".
//
// Endpoints are always case-insensitive and coerced to lowercase. Endpoints
// will only match requests with URLs that match the entire Endpoint and have
// no extra path elements.
func (router *Router) Endpoint(e string) *Endpoint {
	if router.trie == nil {
		router.trie = &trie{
			root: &node{
				children: map[string]*node{},
			},
		}
	}
	keys := keysFromString(e)
	node := router.trie.add(keys, map[string]http.Handler{})
	return (*Endpoint)(node)
}

// keysFromString parses `in` and returns the keys that represent it.
func keysFromString(in string) []key {
	in = strings.Trim(in, "/")
	in = strings.ToLower(in)
	pieces := strings.Split(in, "/")
	keys := make([]key, 0, len(pieces))
	for _, piece := range pieces {
		k := key{
			value: piece,
		}
		if strings.HasPrefix(piece, "{") && strings.HasSuffix(piece, "}") {
			k.dynamic = true
			k.value = piece[1 : len(piece)-1]
		}
		keys = append(keys, k)
	}
	return keys
}

// Handler sets the default http.Handler for `e`, to be used for all requests
// that `e` matches that don't match a method explicitly set for `e` using the
// Methods method.
//
// Handler is not concurrency-safe, and should not be used while the Router `e`
// belongs to is actively routing traffic.
func (e *Endpoint) Handler(h http.Handler) {
	(*node)(e).methods[catchAllMethod] = h
}

// Prefix defines a URL template that requests can be matched against. It is
// only valid to instantiate a prefix by calling `Router.Prefix`. Prefixes, on
// their own, are only useful for calling their methods, as they don't do
// anything until an http.Handler is associated with them.
//
// Unlike Endpoints, Prefixes will match any request that starts with their
// prefix, no matter whether or not the request is for a URL that is longer
// than the Prefix.
type Prefix node

// Prefix defines a new Prefix on the Router. The Prefix should be a URL
// template, using curly braces to denote parameters that should be filled at
// runtime. For example, `{id}` denotes a parameter named `id` that should be
// filled with whatever the request has in that space.
//
// Parameters are always `/`-separated strings. There is no support for regular
// expressions or other limitations on what may be in those strings. A
// parameter is simply defined as "whatever is between these two / characters".
//
// Prefixes are always case-insensitive and coerced to lowercase. Prefixes will
// only match requests with URLs that match the entire Prefix, but the URL may
// have additional path elements after the Prefix and still be considered a
// match.
func (router *Router) Prefix(p string) *Prefix {
	if router.trie == nil {
		router.trie = &trie{
			root: &node{
				children: map[string]*node{},
			},
		}
	}
	keys := keysFromString(p)
	last := keys[len(keys)-1]
	last.prefix = true
	keys[len(keys)-1] = last
	node := router.trie.add(keys, map[string]http.Handler{})
	return (*Prefix)(node)
}

// Handler sets the default http.Handler for `p`, to be used for all requests
// that `p` matches that don't match a method explicitly set for `p` using the
// Methods method.
//
// Handler is not concurrency-safe, and should not be used while the Router `p`
// belongs to is actively routing traffic.
func (p *Prefix) Handler(h http.Handler) {
	(*node)(p).methods[catchAllMethod] = h
}

// Methods defines a pairing of an Endpoint to HTTP request methods, to map
// designate specific http.Handlers for requests matching that Endpoint made
// using the specified methods. It is only valid to instantiate Methods by
// calling `Endpoint.Methods`. Methods, on their own, are only useful for
// calling the `Methods.Handler` method, as they don't modify the Router until
// their `Methods.Handler` method is called.
type Methods struct {
	n *node
	m []string
}

// Methods returns a Methods object that will enable the mapping of the passed
// HTTP request methods to the Endpoint. On its own, this function does not
// modify anything. It should, instead, be used as a friendly shorthand to get
// to the Methods.Handler method.
func (e *Endpoint) Methods(m ...string) Methods {
	return Methods{
		n: (*node)(e),
		m: m,
	}
}

// Methods returns a Methods object that will enable the mapping of the passed
// HTTP request methods to the Prefix. On its own, this function does not
// modify anything. It should, instead, be used as a friendly shorthand to get
// to the Methods.Handler method.
func (p *Prefix) Methods(m ...string) Methods {
	return Methods{
		n: (*node)(p),
		m: m,
	}
}

// Handler associates an http.Handler with the Endpoint associated with `m`, to
// be used whenever a request that matches the Endpoint also matches one of the
// Methods associated with `m`.
//
// Handler is not concurrency-safe. It should not be called while the Router
// that owns the Endpoint that `m` belongs to is actively serving traffic.
func (m Methods) Handler(h http.Handler) {
	for _, method := range m.m {
		m.n.methods[method] = h
	}
}
