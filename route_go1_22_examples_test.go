//go:build go1.22

package trout_test

import (
	"net/http"

	"darlinggo.co/trout/v2"
)

func ExampleRouter_Endpoint_pathValues() {
	postsHandler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// we populate the PathValue request property introduced
			// in Go 1.22 when you build with Go 1.22.
			id := r.PathValue("id")
			_, err := w.Write([]byte(id))
			if err != nil {
				panic(err)
			}
		})

	var router trout.Router
	router.Endpoint("/posts/{id}").Handler(postsHandler)

	req, _ := http.NewRequest("GET", "http://example.com/posts/foo", nil)
	router.ServeHTTP(exampleResponseWriter{}, req)

	// Output:
	// foo
}
