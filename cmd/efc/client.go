package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1/cloudv1connect"
)

// apiTimeout bounds one API call. Reading a job log can take a while.
const apiTimeout = 2 * time.Minute

// newAPIClient returns an API client that sends the token as a bearer token,
// only to the host of apiURL. Redirects to another host are refused, so the
// token never leaves that host. An empty token sends no Authorization
// header, enough for GetServiceInfo.
func newAPIClient(apiURL, token string) cloudv1connect.CloudServiceClient {
	baseURL := strings.TrimRight(apiURL, "/")
	apiHost := ""
	if parsed, err := url.Parse(baseURL); err == nil {
		apiHost = parsed.Host
	}
	httpClient := &http.Client{
		Timeout:   apiTimeout,
		Transport: bearerTransport{token: token, apiHost: apiHost, next: http.DefaultTransport},
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			if request.URL.Host != apiHost {
				return fmt.Errorf("refusing a redirect from %s to another host %s", apiHost, request.URL.Host)
			}
			return nil
		},
	}
	return cloudv1connect.NewCloudServiceClient(httpClient, baseURL)
}

// bearerTransport adds "Authorization: Bearer <token>" to requests for the API host.
type bearerTransport struct {
	token   string
	apiHost string
	next    http.RoundTripper
}

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.token == "" || request.URL.Host != t.apiHost {
		return t.next.RoundTrip(request)
	}
	withToken := request.Clone(request.Context())
	withToken.Header.Set("Authorization", "Bearer "+t.token)
	return t.next.RoundTrip(withToken)
}
