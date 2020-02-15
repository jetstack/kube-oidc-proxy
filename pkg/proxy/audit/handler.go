// Copyright Jetstack Ltd. See LICENSE for details.
package audit

import (
	"net/http"
)

// This struct is used to implement an http.Handler interface. This will not
// actually serve but instead implements auditing during unauthenticated
// requests. It is expected that consumers of this type will call `ServeHTTP`
// when an unauthenticated request is received.
type unauthenticatedHandler struct{}

func NewUnauthenticatedHandler(a *Audit) http.Handler {
	u := new(unauthenticatedHandler)
	return a.WithUnauthorized(u)
}

func (u *unauthenticatedHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
