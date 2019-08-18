package trout_test

import (
	"fmt"
	"net/http"
	"os"

	"darlinggo.co/trout/v2"
)

type exampleResponseWriter struct{}

func (e exampleResponseWriter) Header() http.Header {
	return http.Header{}
}

func (e exampleResponseWriter) Write(in []byte) (int, error) {
	return os.Stdout.Write(append(in, '\n'))
}

func (e exampleResponseWriter) WriteHeader(statusCode int) {
}

func ExampleEndpoint_Handler() {
	// usually your handler is defined elsewhere
	// here we're defining a dummy for demo purposes
	postsHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("matched"))
		})

	var router trout.Router
	router.Endpoint("/posts/{slug}/comments/{id}").Handler(postsHandler)

	// all requests to /posts/FOO/comments/BAR will be routed to
	// postsHandler

	// normally this is done by passing router to http.ListenAndServe,
	// or using http.Handle("/", router), but we don't want to run a
	// server, we just want to call the router by hand right now
	req, _ := http.NewRequest("GET",
		"http://example.com/posts/foo/comments/bar", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// the handler responds to any HTTP method
	req, _ = http.NewRequest("POST",
		"http://example.com/posts/foo/comments/bar", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// routes that don't match return a 404
	req, _ = http.NewRequest("GET", "http://example.com/posts/foo", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	req, _ = http.NewRequest("PUT", "http://example.com/users/bar", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// endpoints don't match on prefix
	req, _ = http.NewRequest("GET", "http://example.com/posts/foo/comments/bar/id/baz", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// Output:
	// matched
	// matched
	// 404 Page Not Found
	// 404 Page Not Found
	// 404 Page Not Found
}

func ExamplePrefix_Handler() {
	// usually your handler is defined elsewhere
	// here we're defining a dummy for demo purposes
	postsHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("matched"))
		})

	var router trout.Router
	router.Prefix("/posts/{slug}").Handler(postsHandler)

	// all requests that begin with /posts/FOO will be routed to
	// postsHandler

	// normally this is done by passing router to http.ListenAndServe,
	// or using http.Handle("/", router), but we don't want to run a
	// server, we just want to call the router by hand right now

	// an exact match still works
	req, _ := http.NewRequest("GET", "http://example.com/posts/foo", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// but now anything using that prefix matches, too
	req, _ = http.NewRequest("GET", "http://example.com/posts/foo/comments/bar", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// Output:
	// matched
	// matched
}

func ExampleMethods_Handler() {
	// usually your handler is defined elsewhere
	// here we're defining a dummy for demo purposes
	postsHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("matched"))
		})

	var router trout.Router
	router.Endpoint("/posts/{slug}").Methods("GET", "POST").Handler(postsHandler)

	// only requests to /posts/FOO that are made with the GET or POST
	// methods will be routed to postsHandler. Every other method will get
	// a 405.

	// normally this is done by passing router to http.ListenAndServe,
	// or using http.Handle("/", router), but we don't want to run a
	// server, we just want to call the router by hand right now
	req, _ := http.NewRequest("GET", "http://example.com/posts/foo", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	req, _ = http.NewRequest("POST", "http://example.com/posts/foo", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// this will return a 405
	req, _ = http.NewRequest("PUT", "http://example.com/posts/foo", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// Output:
	// matched
	// matched
	// 405 Method Not Allowed
}

func ExampleRequestVars() {
	postsHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// RequestVars returns an http.Header object
			vars := trout.RequestVars(r)

			// you can use Get, but if a parameter name is
			// repeated, you'll only get the first instance
			// of it.
			firstID := vars.Get("id")

			// you can access all the instances of a parameter name
			// using the map index. Just remember to use
			// http.CanonicalHeaderKey.
			allIDs := vars[http.CanonicalHeaderKey("id")]
			secondID := allIDs[1]

			w.Write([]byte(fmt.Sprintf("%s\n%v\n%s",
				firstID, allIDs, secondID)))
		})

	var router trout.Router
	router.Endpoint("/posts/{id}/comments/{id}").Handler(postsHandler)

	req, _ := http.NewRequest("GET", "http://example.com/posts/foo/comments/bar", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// Output:
	// foo
	// [foo bar]
	// bar
}
