package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/boring-design/elastic-fruit-cli/agenttools"
	"github.com/boring-design/elastic-fruit-protocol/gen/cloud/v1/cloudv1connect"
)

// Path is where the cloud server serves remote MCP.
const Path = "/mcp"

// inProcessBaseURL is the base URL of the in process API client. The host is
// never dialed, requests go straight to the RPC handler.
const inProcessBaseURL = "http://in-process"

// NewHTTPHandler serves remote MCP over stateless Streamable HTTP. Every
// request must carry "Authorization: Bearer <EFC API token>". Cookies are
// never read, so a browser on another site cannot act with the session of a
// signed in user. Tool calls go in process to rpcHandler, the CloudService
// handler with its auth interceptor, so the token is checked exactly like on
// a direct API call.
func NewHTTPHandler(rpcHandler http.Handler) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		token, _ := bearerToken(r.Header)
		return NewServer(agenttools.New(inProcessClient(rpcHandler, token)))
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// Every request needs a bearer token, so DNS rebinding cannot act
		// for anyone. The server may sit behind a proxy on the same host.
		DisableLocalhostProtection: true,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := bearerToken(r.Header); !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="efc"`)
			http.Error(w, "send an EFC API token as Authorization: Bearer <token>, /mcp does not accept cookies", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
}

// bearerToken returns the token of an "Authorization: Bearer <token>" header.
func bearerToken(header http.Header) (string, bool) {
	scheme, token, found := strings.Cut(header.Get("Authorization"), " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

// inProcessClient returns an API client whose requests are served by
// rpcHandler in this process, with only the bearer token as credential.
func inProcessClient(rpcHandler http.Handler, token string) cloudv1connect.CloudServiceClient {
	httpClient := &http.Client{Transport: inProcessTransport{rpcHandler: rpcHandler, token: token}}
	return cloudv1connect.NewCloudServiceClient(httpClient, inProcessBaseURL)
}

// inProcessTransport hands each request to the RPC handler and buffers the
// answer. The agent tools only make unary calls, so buffering is fine.
type inProcessTransport struct {
	rpcHandler http.Handler
	token      string
}

func (t inProcessTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	inner := request.Clone(request.Context())
	inner.Header.Del("Cookie")
	inner.Header.Set("Authorization", "Bearer "+t.token)
	inner.RequestURI = inner.URL.RequestURI()
	recorder := httptest.NewRecorder()
	t.rpcHandler.ServeHTTP(recorder, inner)
	response := recorder.Result()
	response.Request = request
	return response, nil
}
