# Importing trout
    import "darlinggo.co/trout"

# About trout

`trout` is an opinionated router that is biased towards RESTful services. It does not rely on global, mutable state and has no dependencies. `trout` also goes out of its way to enable servers to correctly respond with an `http.StatusMethodNotAllowed` error instead of an `http.StatusNotFound` error when the endpoint can be matched but is not configured to respond to the HTTP method the request used. It also takes pains to make sure that `OPTIONS` requests can be fulfilled, by making the methods an endpoint is configured with available to the `http.Handler`. In general, the over-arching goals of `trout` are:

* Be simple. Rely only on the standard library, and provide the barest possible functionality to get the job done.
* Be transparent. Allow `http.Handlers` to get to information that will help them do their jobs.
* Be intuitive. Value sane defaults and clear behaviours.

Docs can be found on [GoDoc.org](https://godoc.org/darlinggo.co/trout).

If you're using trout, we encourage you to join the [trout mailing list](https://groups.google.com/a/darlinggo.co/group/trout), which will be our main mode of communication.

# Using trout

## Creating a router

Creating a router is straight-forward: the `trout.Router` type's zero value is an acceptable router with no endpoints. Adding endpoints is a matter of calling methods on the variable.

```go
var router trout.Router
router.Endpoint("/posts/{slug}/comments/{id}").Handler(postsHandler)
```

That routemr will now match the URL `/posts/WHATEVERYOUTYPE/comments/123`. You can replace `WHATEVERYOUTYPE` and `123` with any string that doesn't contain a `/`. All requests matching this pattern will be handled by `postsHandler`.

The `Endpoint` method is basic: it accepts a string to match the URL against. Strings get broken down into resources; resources are split by the `/` character. Resources come in two flavours: static and dynamic. A static resource will match the resource text exactly; `posts` and `comments` in the example above are static resources. Dynamic resources are just placeholders; they match any text at all; `WHATEVERYOUTYPE` and `123` are dynamic resources in the example above.

The `Endpoint` method returns a `trout.Endpoint`, which can have an `http.Handler` associated with it by calling its `Handler` method, and passing the `http.Handler` you want to use as the handler for requests that match the endpoint.

## Working with HTTP methods

```go
var router trout.Router
router.Endpoint("/posts/{slug}").Methods("GET", "POST").Handler(postsHandler)
```

The example above associates `postsHandler` with the `/posts/{slug}` endpoint, but _only_ for requests made using the `GET` or `POST` HTTP method. All other requests will return an `http.StatusMethodNotAllowed` error. Any number of methods can be passed to the `Methods`... errr... method.

## Working with variables

Now that a handler has been matched, we need to get the values that filled the dynamic resources placeholders in the URL. The `trout.RequestVars` helper function can be used to return the values the were used.

Variables in `trout` are passed as request headers. All the `trout` parameters are set as `Trout-Param-RESOURCETEXT`, where `RESOURCETEXT` is the text you entered between `{` and `}` in the endpoint. For example, `/posts/{slug}/comments/{id}` would have `Trout-Param-Slug` and `Trout-Param-Id` set in the request headers. `trout.RequestVars(r)` simply returns all headers that begin with `Trout-Param-`, and strip that prefix, returning an `http.Header` object. So calling `trout.RequestVars(r)` in our example would return an `http.Header` object with keys for `Id` and `Slug`.

In the event that the same text is reused as a dynamic resource in multiple parts of the endpoint, both values will still be available, because each key in an `http.Header` corresponds to a slice of values. For example, if the endpoint is `/posts/{id}/comments/{id}`, the `http.Header` returned from `trout.RequestVars(r)` will contain just a single `Id` key, and it would hold two values. The values will always be in the same order they were in in the URL.

## Setting the 404 and 405 responses

By default, `trout` will respond with `http.StatusNotFound` when no endpoint can fulfill the request, and `http.StatusMethodNotAllowed` when none of the endpoints that can fulfill the request are configured to respond to the HTTP method used. These defaults will also write a default error message as the response body. Furthermore, for `http.StatusMethodNotAllowed` responses, the `Allow` header will be set on the response, containing the methods the endpoint is configured to respond to.

Sometimes, however, you want something besides these defaults. In that case, you can set the `Handle404` property on your router to the `http.Handler` you want to use for requests where the endpoint can't be found, and `Handle405` to the `http.Handler` you want to use for requests where an endpoint is matched, but isn't configured to respond to the HTTP method used.

## Getting extra information

`trout` sets two extra request headers when routing:

* `Trout-Timer` is set to the number of nanoseconds it took to route the request. This allows you to monitor how much of your response time is spent on routing.
* `Trout-Pattern` is set to the endpoint text that the request matched, which makes it easier to determine which endpoint resulted in the handler being called. This is particularly useful when using placeholders, as the value will always be the same, no matter what text is placed in the placeholder. This makes it easier to monitor at an endpoint-granularity.

## Understanding routing

There are times when multiple endpoints can be used to serve the same request. A simplified example would be `/version` and `/{id}` both being endpoints on a router. When a request is made to `/version`, which should be used?

Trout resolves this by trying to find the most specific route that can handle the request. Endpoints get a score based on how many placeholders exist in the endpoint, and how close to the beginning of the endpoint they are. The more placeholders an endpoint has, and the earlier in the endpoint they are, the lower the score is. The highest scoring endpoint serves the request.
