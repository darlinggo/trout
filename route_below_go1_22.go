//go:build !go1.22

package trout

import "net/http"

func setBuiltinRequestPathVar(_ *http.Request, _, _ string) {
}
