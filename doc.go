/*
Package trout provides an opinionated router that's implemented
using a basic trie.

The router is opinionated and biased towards basic RESTful services. Its main
constraint is that its URL templating is very basic and has no support for
regular expressions or anything other than a direct equality comparison or
prefix match, unlike many routing libraries.

The router is specifically designed to support users that want to return
correct information with OPTIONS requests, so it enables users to retrieve a
list of HTTP methods an Endpoint or Prefix is configured to respond to. It will
not return the methods an Endpoint or Prefix is implicitly configured to
respond to by associating a Handler with the Endpoint or Prefix itself. These
HTTP methods can be accessed through the Trout-Methods header that is injected
into the http.Request object. Each method will be its own string in the slice.

The router is also specifically designed to differentiate between a
404 (Not Found) response and a 405 (Method Not Allowed) response. It
will use the configured Handle404 http.Handler when no Endpoint or Prefix is found
that matches the http.Request's Path property. It will use the
configured Handle405 http.Handler when an Endpoint or Prefix is found for the
http.Request's Path, but the http.Request's Method has no Handler
associated with it. Setting a default http.Handler for the Endpoint or Prefix will
result in the Handle405 http.Handler never being used for that Endpoint.
*/
package trout
