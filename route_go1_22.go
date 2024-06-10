//go:build go1.22

package trout

import "net/http"

func setBuiltinRequestPathVar(r *http.Request, name, value string) {
	r.SetPathValue(name, value)
}
