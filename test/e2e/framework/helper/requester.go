package helper

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

type Requester struct {
	transport http.RoundTripper
	token     string
	client    *http.Client
}

func (h *Helper) NewRequester(transport http.RoundTripper, token string) *Requester {
	r := &Requester{
		token: token,
	}

	r.client = http.DefaultClient
	r.client.Transport = r

	return r
}

func (r *Requester) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("bearer %s", r.token))
	return r.transport.RoundTrip(req)
}

func (r *Requester) Get(target string) ([]byte, int, error) {
	resp, err := r.client.Get(target)
	if err != nil {
		return nil, 0, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil
}
