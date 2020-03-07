// Copyright Jetstack Ltd. See LICENSE for details.
package audit

import (
	"net/http"
)

// This struct is used to implement an http.Handler interface. This will not
// actually serve but instead implements auditing during unauthenticated
// requests. It is expected that consumers of this type will call `ServeHTTP`
// when an unauthenticated request is received.
type unauthenticatedHandler struct {
	serveFunc func(http.ResponseWriter, *http.Request)
}

func NewUnauthenticatedHandler(a *Audit, serveFunc func(http.ResponseWriter, *http.Request)) http.Handler {
	u := &unauthenticatedHandler{
		serveFunc: serveFunc,
	}

	// if auditor is nil then return without wrapping
	if a == nil {
		return u
	}

	return a.WithUnauthorized(u)
}

func (u *unauthenticatedHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	u.serveFunc(rw, r)
}
