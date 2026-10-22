package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iximiuz/labctl/content"
)

func TestSessionCredentialsStayOnConfiguredOrigin(t *testing.T) {
	c := NewClient(ClientOptions{
		BaseURL: "https://labs.iximiuz.com", APIBaseURL: "https://labs.iximiuz.com/api",
		SessionID: "session", AccessToken: "token",
	})
	for _, tc := range []struct {
		url  string
		auth bool
	}{
		{"https://labs.iximiuz.com/api/challenges/example", true},
		{"https://labs.iximiuz.com:443/another-path", true},
		{"https://labs.iximiuz.com.attacker.example/upload", false},
		{"https://labs.iximiuz.com:444/upload", false},
		{"https://sub.labs.iximiuz.com/upload", false},
		{"http://labs.iximiuz.com/api/challenges/example", false},
		{"https://labs.iximiuz.com@attacker.example/upload", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			req, err := c.newRequest(t.Context(), http.MethodGet, tc.url, nil, nil, nil)
			require.NoError(t, err)
			require.Equal(t, tc.auth, req.Header.Get("Authorization") != "")
		})
	}
}

func TestAuthenticatedRedirectsStayOnOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, location string
		follow         bool
	}{
		{"same origin", "https://labs.iximiuz.com/done", true},
		{"subdomain", "https://sub.labs.iximiuz.com/done", false},
		{"HTTPS downgrade", "http://labs.iximiuz.com/done", false},
		{"different port", "https://labs.iximiuz.com:444/done", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient(ClientOptions{
				BaseURL: "https://labs.iximiuz.com", APIBaseURL: "https://labs.iximiuz.com/api",
				SessionID: "session", AccessToken: "token",
			})
			calls := 0
			c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				require.NotEmpty(t, req.Header.Get("Authorization"))
				if calls == 1 {
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{tc.location}},
						Body:       io.NopCloser(strings.NewReader("")),
						Request:    req,
					}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), Request: req}, nil
			})
			req, err := c.newRequest(t.Context(), http.MethodGet, "https://labs.iximiuz.com/api/challenges/example", nil, nil, nil)
			require.NoError(t, err)
			resp, err := c.httpClient.Do(req)
			if tc.follow {
				require.NoError(t, err)
				resp.Body.Close()
				require.Equal(t, 2, calls)
			} else {
				require.ErrorContains(t, err, "different origin")
				require.Equal(t, 1, calls)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPresignedUploadHasNoSessionHeader(t *testing.T) {
	for _, sameOrigin := range []bool{false, true} {
		t.Run(fmt.Sprintf("same origin %t", sameOrigin), func(t *testing.T) {
			payload := []byte("file contents")
			file := filepath.Join(t.TempDir(), "upload.txt")
			require.NoError(t, os.WriteFile(file, payload, 0o600))
			checkUpload := func(r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("upload sent CLI authorization")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != string(payload) {
					t.Errorf("unexpected upload body %q: %v", body, err)
				}
			}

			var uploadURL string
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/content/files":
					if r.Header.Get("Authorization") == "" {
						t.Error("API request missing CLI authorization")
					}
					fmt.Fprintf(w, `{"uploadUrl":%q}`, uploadURL)
				case "/upload":
					checkUpload(r)
				default:
					t.Errorf("unexpected request path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer apiServer.Close()

			uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				checkUpload(r)
			}))
			defer uploadServer.Close()
			uploadURL = uploadServer.URL + "/upload"
			if sameOrigin {
				uploadURL = apiServer.URL + "/upload"
			}

			c := NewClient(ClientOptions{
				BaseURL: apiServer.URL, APIBaseURL: apiServer.URL + "/api",
				SessionID: "session", AccessToken: "token",
			})
			require.NoError(t, c.UploadContentFile(t.Context(), content.KindChallenge, "example", "upload.txt", file))
		})
	}
}

func TestContentGetterEscapesNameInRequestURI(t *testing.T) {
	var requestPath, requestURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath, requestURI = r.URL.Path, r.RequestURI
		io.WriteString(w, `{}`)
	}))
	defer server.Close()
	c := NewClient(ClientOptions{BaseURL: server.URL, APIBaseURL: server.URL + "/api"})
	_, err := c.GetChallenge(t.Context(), "%2e%2e%2fauth%2fme")
	require.NoError(t, err)
	require.Equal(t, "/api/challenges/%2e%2e%2fauth%2fme", requestPath)
	require.Equal(t, "/api/challenges/%252e%252e%252fauth%252fme", requestURI)
}
