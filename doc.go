/*
Package trout provides an opinionated router that's implemented
using a basic trie.

The router is opinionated and biased towards basic RESTful
services. Its main constraint is that its URL templating is very
basic and has no support for regular expressions, prefix matching,
or anything other than a direct equality comparison, unlike many
routing libraries.

The router is specifically designed to support users that want to
return correct information with HEAD requests, so it enables users
to retrieve a list of HTTP methods an Endpoint is configured to
respond to. It will not return the configurations an Endpoint is
implicitly configured to respond to by associated a Handler with the
Endpoint itself. These HTTP methods can be accessed through the
Trout-Methods header that is injected into the http.Request object.
Each method will be its own string in the slice.

The router is also specifically designed to differentiate between a
404 (Not Found) response and a 405 (Method Not Allowed) response. It
will use the configured Handle404 http.Handler when no Endpoint is found
that matches the http.Request's Path property. It will use the
configured Handle405 http.Handler when an Endpoint is found for the
http.Request's Path, but the http.Request's Method has no Handler
associated with it. Setting a default http.Handler for the Endpoint will
result in the Handle405 http.Handler never being used for that Endpoint.

To map an Endpoint to a http.Handler:

	var router trout.Router
	router.Endpoint("/posts/{slug}/comments/{id}").Handler(postsHandler)

All requests that match that URL structure will be passed to the postsHandler,
no matter what HTTP method they use.

To map specific Methods to a http.Handler:

	var router trout.Router
	router.Endpoint("/posts/{slug}").Methods("GET", "POST").Handler(postsHandler)

Only requests that match that URL structure will be passed to the postsHandler,
and only if they use the GET or POST HTTP method.

To access the URL parameter values inside a request, use the RequestVars helper:

	func handler(w http.ResponseWriter, r *http.Request) {
		vars := trout.RequestVars(r)
		...
	}

This will return an http.Header object containing the parameter values. They are
passed into the http.Handler by injecting them into the http.Request's Header property,
with the header key of "Trout-Params-{parameter}". The RequestVars helper is just a
convenience function to strip the prefix. Parameters are always passed without the curly
braces. Finally, if a parameter name is used multiple times in a single URL template, values
will be stored in the slice in the order they appeared in the template:

	// for the template /posts/{id}/comments/{id}
	// filled with /posts/hello-world/comments/1
	vars := trout.RequestVars(r)
	fmt.Println(vars.Get("id")) // outputs `hello-world`
	fmt.Println(vars[http.CanonicalHeaderKey("id")]) // outputs `["hello-world", "1"]`
	fmt.Println(vars[http.CanonicalHeaderKey("id"})][1]) // outputs `1`
*/
package trout
