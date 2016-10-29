package trout

import (
	"fmt"
	"net/http"
	"testing"
)

type testHandler string

func (t testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(t))
}

func TestRouting(t *testing.T) {
	type testCase struct {
		url, method, handler string
	}
	cases := []testCase{
		{"/v1", "GET", "get-static"},
		{"/v1/", "GET", "get-static"},
		{"/hello", "GET", "get-dynamic"},
		{"/hello/", "GET", "get-dynamic"},
		{"/v1", "POST", "post-dynamic"},
		{"/v1/", "POST", "post-dynamic"},
	}
	var router Router
	router.Handle404 = testHandler("404")
	router.Handle405 = testHandler("405")
	router.Endpoint("/{id}").Methods("GET").Handler(testHandler("get-dynamic"))
	router.Endpoint("/v1").Methods("GET").Handler(testHandler("get-static"))
	router.Endpoint("/{id}").Methods("POST").Handler(testHandler("post-dynamic"))
	fmt.Println(router.t.debug())
	for _, c := range cases {
		r, err := http.NewRequest(c.method, c.url, nil)
		if err != nil {
			t.Fatalf("Error creating request for %s %s: %+v", c.method, c.url, err)
		}
		h := router.getHandler(r)
		res := string(h.(testHandler))
		if res != c.handler {
			t.Errorf("Expected to route \"%s %s\" to %s, routed to %s", c.method, c.url, c.handler, res)
		}

	}
}
