package trout

import (
	"encoding/base64"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
		{"/", "GET", "get-root"},
		{"/hello/world", "DELETE", "catch-all"},
		{"/ancestor/one", "GET", "ancestor-one"},
		{"/ancestor/two", "GET", "ancestor-two"},
		{"/prefix/foo/bar", "GET", "get-prefix"},
		{"/prefix/foo", "GET", "get-prefix"},
		{"/prefix/static", "GET", "static-prefix"},
		{"/prefix/static/bar/baz", "GET", "static-prefix"},
		{"/prefix", "GET", "get-dynamic"},
	}
	var router Router
	router.Handle404 = testHandler("404")
	router.Handle405 = testHandler("405")
	router.Prefix("/prefix/static").Methods("GET").Handler(testHandler("static-prefix"))
	router.Prefix("/prefix/{id}").Methods("GET").Handler(testHandler("get-prefix"))
	router.Endpoint("/{id}").Methods("GET").Handler(testHandler("get-dynamic"))
	router.Endpoint("/v1").Methods("GET").Handler(testHandler("get-static"))
	router.Endpoint("/{id}").Methods("POST").Handler(testHandler("post-dynamic"))
	router.Endpoint("/").Methods("GET").Handler(testHandler("get-root"))
	router.Endpoint("/ancestor/one").Methods("GET").Handler(testHandler("ancestor-one"))
	router.Endpoint("/ancestor/two").Methods("GET").Handler(testHandler("ancestor-two"))
	router.Endpoint("/hello/world").Handler(testHandler("catch-all"))
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

func TestKeysFromString(t *testing.T) {
	cases := map[string][]key{
		"/{id}/": []key{
			{value: "id", dynamic: true},
		},
		"/v1": []key{
			{value: "v1"},
		},
		"/": []key{
			{value: ""},
		},
		"/ancestor/one": []key{
			{value: "ancestor"},
			{value: "one"},
		},
		"/ancestor/two": []key{
			{value: "ancestor"},
			{value: "two"},
		},
	}
	for in, expect := range cases {
		t.Logf("Testing case %s", in)
		result := keysFromString(in)
		if len(result) != len(expect) {
			t.Errorf("Expected %d results, got %d: %+v", len(expect), len(result), result)
			continue
		}
		for pos, res := range result {
			if !res.equals(expect[pos]) {
				t.Errorf("Expected result %d to be %+v, was %+v", pos, expect[pos], res)
			}
		}
	}
}

var benchRouter Router
var benchTests []string
var benchMethods = [...]string{"GET", "POST", "PUT", "DELETE"}

func init() {
	for i := 0; i < 100; i++ {
		rand.Seed(time.Now().UnixNano())
		depth := rand.Intn(4) + 1
		var route string
		var req string
		for x := 0; x < depth; x++ {
			rand.Seed(time.Now().UnixNano())
			param := rand.Intn(1) == 1
			pieceLength := rand.Intn(24) + 1
			piece := make([]byte, pieceLength)
			rand.Read(piece)
			pieceStr := base64.URLEncoding.EncodeToString(piece)
			req = req + "/" + pieceStr
			if param {
				pieceStr = "{" + pieceStr + "}"
			}
			route = route + "/" + pieceStr
		}
		benchTests = append(benchTests, req)
		endpoint := benchRouter.Endpoint(route)
		rand.Seed(time.Now().UnixNano())
		get := rand.Intn(1) == 1
		rand.Seed(time.Now().UnixNano())
		post := rand.Intn(1) == 1
		catchAll := !get && !post
		var methods []string
		if get {
			methods = append(methods, "GET")
		}
		if post {
			methods = append(methods, "POST")
		}
		if catchAll {
			methods = append(methods, catchAllMethod)
		}
		endpoint.Methods(methods...).Handler(testHandler("benchmark"))
	}
}

func BenchmarkRouting(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		w := httptest.NewRecorder()
		route := benchTests[i%len(benchTests)]
		method := benchMethods[i%len(benchMethods)]
		req, err := http.NewRequest(method, route, nil)
		if err != nil {
			b.Fatalf(err.Error())
		}
		b.StartTimer()
		benchRouter.ServeHTTP(w, req)
	}
}
