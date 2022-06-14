// Copyright Jetstack Ltd. See LICENSE for details.
package logging

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	UserHeaderClientIPKey = "Remote-Client-IP"
	timestampLayout       = "2006-01-02T15:04:05-0700"
)

// logs the request
func LogSuccessfulRequest(req *http.Request, inboundUser user.Info, outboundUser user.Info) {
	remoteAddr := req.RemoteAddr
	indexOfColon := strings.Index(remoteAddr, ":")
	if indexOfColon > 0 {
		remoteAddr = remoteAddr[0:indexOfColon]
	}

	inboundExtras := ""

	if inboundUser.GetExtra() != nil {
		for key, value := range inboundUser.GetExtra() {
			inboundExtras += key + "=" + strings.Join(value, "|") + " "
		}
	}

	outboundUserLog := ""

	if outboundUser != nil {
		outboundExtras := ""

		if outboundUser.GetExtra() != nil {
			for key, value := range outboundUser.GetExtra() {
				outboundExtras += key + "=" + strings.Join(value, "|") + " "
			}
		}

		outboundUserLog = fmt.Sprintf(" outbound:[%s / %s / %s / %s]", outboundUser.GetName(), strings.Join(outboundUser.GetGroups(), "|"), outboundUser.GetUID(), outboundExtras)
	}

	xFwdFor := findXForwardedFor(req.Header, remoteAddr)

	fmt.Printf("[%s] AuSuccess src:[%s / % s] URI:%s inbound:[%s / %s / %s]%s\n", time.Now().Format(timestampLayout), remoteAddr, xFwdFor, req.RequestURI, inboundUser.GetName(), strings.Join(inboundUser.GetGroups(), "|"), inboundExtras, outboundUserLog)
}

// determines if the x-forwarded-for header is present, if so remove
// the remoteaddr since it is repetitive
func findXForwardedFor(headers http.Header, remoteAddr string) string {
	xFwdFor := headers.Get("x-forwarded-for")
	// clean off remoteaddr from x-forwarded-for
	if xFwdFor != "" {

		newXFwdFor := ""
		oneFound := false
		xFwdForIps := strings.Split(xFwdFor, ",")

		for _, ip := range xFwdForIps {
			ip = strings.TrimSpace(ip)

			if ip != remoteAddr {
				newXFwdFor = newXFwdFor + ip + ", "
				oneFound = true
			}

		}

		if oneFound {
			newXFwdFor = newXFwdFor[0 : len(newXFwdFor)-2]
		}

		xFwdFor = newXFwdFor

	}

	return xFwdFor
}

// logs the failed request
func LogFailedRequest(req *http.Request) {
	remoteAddr := req.RemoteAddr
	indexOfColon := strings.Index(remoteAddr, ":")
	if indexOfColon > 0 {
		remoteAddr = remoteAddr[0:indexOfColon]
	}

	fmt.Printf("[%s] AuFail src:[%s / % s] URI:%s\n", time.Now().Format(timestampLayout), remoteAddr, req.Header.Get(("x-forwarded-for")), req.RequestURI)
}
