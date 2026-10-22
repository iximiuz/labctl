package content

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/config"
	"github.com/iximiuz/labctl/internal/labcli"
)

func TestContentFeedback(t *testing.T) {
	const name = "graphql-vuln-lab-ca696b14"
	for _, tc := range []struct {
		name   string
		kind   content.ContentKind
		format string
		empty  bool
	}{
		{"challenge JSON", content.KindChallenge, "json", false},
		{"tutorial YAML", content.KindTutorial, "yaml", false},
		{"empty challenge JSON", content.KindChallenge, "json", true},
		{"empty tutorial YAML", content.KindTutorial, "yaml", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Basic c2Vzc2lvbjp0b2tlbg==" {
					t.Errorf("missing CLI authorization")
				}
				switch r.URL.Path {
				case "/api/auth/me":
					io.WriteString(w, `{"id":"author-1"}`)
				case "/api/" + tc.kind.Plural() + "/" + name:
					io.WriteString(w, `{"authors":[{"userId":"author-1"}]}`)
				case "/api/content/feedback":
					if r.URL.Query().Get("kind") != tc.kind.String() || r.URL.Query().Get("name") != name {
						t.Errorf("unexpected feedback query: %s", r.URL.RawQuery)
					}
					if tc.empty {
						io.WriteString(w, `{"role":"author","threads":[]}`)
					} else {
						io.WriteString(w, `{"role":"author","threads":[{"id":"thread-1"}]}`)
					}
				case "/api/content/feedback/threads/thread-1":
					io.WriteString(w, `{"id":"thread-1","reporter":{"displayName":"Learner"},"messages":[{"id":"message-1","body":"first line\nsecond line: with colon","attachments":[{"name":"log.txt"}]},{"id":"message-2","body":"Thanks, fixed it."}]}`)
				default:
					t.Errorf("unexpected request to %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			var stdout bytes.Buffer
			cli := labcli.NewCLI(io.NopCloser(strings.NewReader("")), &stdout, io.Discard, "test")
			cli.SetClient(api.NewClient(api.ClientOptions{
				BaseURL: server.URL, APIBaseURL: server.URL + "/api",
				SessionID: "session", AccessToken: "token",
			}))
			cli.SetConfig(&config.Config{BaseURL: server.URL})
			cmd := NewCommand(cli)
			arg := name
			if tc.kind == content.KindChallenge {
				arg = server.URL + "/" + tc.kind.Plural() + "/" + name + "?feedback"
			}
			cmd.SetArgs([]string{"feedback", tc.kind.String(), arg, "--output", tc.format})
			require.NoError(t, cmd.Execute())

			var got struct {
				Kind    content.ContentKind  `json:"kind" yaml:"kind"`
				Name    string               `json:"name" yaml:"name"`
				Threads []api.FeedbackThread `json:"threads" yaml:"threads"`
			}
			if tc.format == "json" {
				require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
			} else {
				require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &got))
			}
			require.Equal(t, tc.kind, got.Kind)
			require.Equal(t, name, got.Name)
			require.NotNil(t, got.Threads)
			if tc.empty {
				require.Empty(t, got.Threads)
				return
			}
			require.Len(t, got.Threads, 1)
			require.Equal(t, "Learner", got.Threads[0].Reporter.DisplayName)
			require.Equal(t, "first line\nsecond line: with colon", got.Threads[0].Messages[0].Body)
			require.Equal(t, "log.txt", got.Threads[0].Messages[0].Attachments[0].Name)
			require.Equal(t, "Thanks, fixed it.", got.Threads[0].Messages[1].Body)
		})
	}
}

func TestContentFeedbackRequiresAuthor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   content.ContentKind
		userID string
		role   string
	}{
		{"missing user ID", content.KindChallenge, "", "author"},
		{"different author", content.KindTutorial, "other-user", "author"},
		{"non-author role", content.KindCourse, "author-1", "reporter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/auth/me":
					io.WriteString(w, `{"id":"`+tc.userID+`"}`)
				case "/api/" + tc.kind.Plural() + "/example":
					io.WriteString(w, `{"authors":[{"userId":"author-1"}]}`)
				case "/api/content/feedback":
					io.WriteString(w, `{"role":"`+tc.role+`","threads":[{"id":"private-thread"}]}`)
				default:
					t.Errorf("unexpected request to %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			var stdout bytes.Buffer
			cli := labcli.NewCLI(io.NopCloser(strings.NewReader("")), &stdout, io.Discard, "test")
			cli.SetClient(api.NewClient(api.ClientOptions{BaseURL: server.URL, APIBaseURL: server.URL + "/api"}))
			require.Error(t, runFeedback(t.Context(), cli, &feedbackOptions{kind: tc.kind, name: "example", output: "yaml"}))
			require.Empty(t, stdout.String())
		})
	}
}

func TestFeedbackContentName(t *testing.T) {
	const baseURL = "https://labs.iximiuz.com"
	for _, input := range []string{
		"graphql-vuln-lab-ca696b14",
		"https://labs.iximiuz.com/challenges/graphql-vuln-lab-ca696b14?feedback",
		"https://labs.iximiuz.com/challenges/graphql-vuln-lab-ca696b14/?feedback=thread-1",
	} {
		name, err := feedbackContentName(content.KindChallenge, input, baseURL)
		require.NoError(t, err)
		require.Equal(t, "graphql-vuln-lab-ca696b14", name)
	}
	name, err := feedbackContentName(content.KindTutorial, "https://labs.iximiuz.com/tutorials/example?feedback", baseURL)
	require.NoError(t, err)
	require.Equal(t, "example", name)
	for _, input := range []string{
		"", "https://labs.iximiuz.com/courses/example", "example?feedback", "../example",
		".", "..", "example/other", "example%2fother", "%2e", "%252e", "bad\x00name",
		"https://labs.iximiuz.com/challenges/%252e%252e%252fauth%252fme",
		"https://labs.iximiuz.com/challenges/%252e",
		"https://labs.iximiuz.com/challenges/example%252fother",
		"https://labs.iximiuz.com/challenges/example%2fother",
		"https://labs.iximiuz.com/challenges/%2e%2e",
		"https://labs.iximiuz.com/challenges/%GG",
		"https://labs.iximiuz.com.attacker.example/challenges/example",
		"https://other.example/challenges/example",
		"http://labs.iximiuz.com/challenges/example",
		"https://labs.iximiuz.com:444/challenges/example",
		"https://labs.iximiuz.com/challenges/example/extra",
	} {
		_, err := feedbackContentName(content.KindChallenge, input, baseURL)
		require.Error(t, err)
	}
	name, err = feedbackContentName(content.KindChallenge, "http://127.0.0.1:8080/challenges/example?feedback", "http://127.0.0.1:8080")
	require.NoError(t, err)
	require.Equal(t, "example", name)
	_, err = feedbackContentName(content.KindChallenge, "http://labs.iximiuz.com/challenges/example", "http://labs.iximiuz.com")
	require.Error(t, err)
}
