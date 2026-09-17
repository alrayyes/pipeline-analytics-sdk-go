package pipelineanalytics_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	pipelineanalytics "github.com/alrayyes/pipeline-analytics-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cookieEchoServer(t *testing.T, gotCookie *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil {
			*gotCookie = c.Value
		}
		_, _ = w.Write([]byte(`{"version":"dev"}`))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestSessionCookieSource(t *testing.T) {
	// Not parallel: t.Setenv mutates process-global state the "env
	// fallback" case depends on, which a concurrent subtest would race.
	cases := map[string]struct {
		opts []pipelineanalytics.Option
		env  string
		want string
	}{
		"option only":          {opts: []pipelineanalytics.Option{pipelineanalytics.WithSessionCookie("from-option")}, want: "from-option"},
		"env fallback":         {env: "from-env", want: "from-env"},
		"option overrides env": {opts: []pipelineanalytics.Option{pipelineanalytics.WithSessionCookie("from-option")}, env: "from-env", want: "from-option"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(pipelineanalytics.SessionCookieEnvVar, tc.env)
			}

			var gotCookie string
			srv := cookieEchoServer(t, &gotCookie)

			client, err := pipelineanalytics.New(srv.URL, tc.opts...)
			require.NoError(t, err)

			_, err = client.GetVersionWithResponse(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.want, gotCookie)
		})
	}
}

func TestDecodeError(t *testing.T) {
	t.Parallel()

	t.Run("parses the error body", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"code":"not_found","message":"no such repo"}`)
		headers := http.Header{"X-Request-Id": []string{"req-123"}}

		apiErr := pipelineanalytics.DecodeError(http.StatusNotFound, body, headers)

		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
		assert.Equal(t, "not_found", apiErr.Code)
		assert.Equal(t, "no such repo", apiErr.Message)
		assert.Equal(t, "req-123", apiErr.RequestID)
	})

	t.Run("nil on a successful status", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, pipelineanalytics.DecodeError(http.StatusOK, nil, http.Header{}))
	})
}

func TestRetry(t *testing.T) {
	t.Parallel()

	t.Run("retries a 5xx then succeeds", testRetriesA5xxThenSucceeds)
	t.Run("never retries a non-429 4xx", testNeverRetriesANon429FourXX)
	t.Run("honors Retry-After ahead of its own backoff", testHonorsRetryAfter)
	t.Run("gives up after maxRetries", testGivesUpAfterMaxRetries)
}

func testRetriesA5xxThenSucceeds(t *testing.T) {
	t.Parallel()

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}
		_, _ = w.Write([]byte(`{"version":"dev"}`))
	}))
	t.Cleanup(srv.Close)

	client, err := pipelineanalytics.New(srv.URL, pipelineanalytics.WithRetry(3, 0))
	require.NoError(t, err)

	resp, err := client.GetVersionWithResponse(context.Background())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.HTTPResponse.StatusCode)
	assert.Equal(t, 3, attempts)
}

func testNeverRetriesANon429FourXX(t *testing.T) {
	t.Parallel()

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"bad_request","message":"nope"}`))
	}))
	t.Cleanup(srv.Close)

	client, err := pipelineanalytics.New(srv.URL, pipelineanalytics.WithRetry(3, 0))
	require.NoError(t, err)

	_, err = client.GetVersionWithResponse(context.Background())
	require.NoError(t, err) // a well-formed 4xx isn't a transport error

	assert.Equal(t, 1, attempts)
}

func testHonorsRetryAfter(t *testing.T) {
	t.Parallel()

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}
		_, _ = w.Write([]byte(`{"version":"dev"}`))
	}))
	t.Cleanup(srv.Close)

	client, err := pipelineanalytics.New(srv.URL, pipelineanalytics.WithRetry(2, 0))
	require.NoError(t, err)

	resp, err := client.GetVersionWithResponse(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.HTTPResponse.StatusCode)
}

func testGivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client, err := pipelineanalytics.New(srv.URL, pipelineanalytics.WithRetry(2, 0))
	require.NoError(t, err)

	resp, err := client.GetVersionWithResponse(context.Background())
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.HTTPResponse.StatusCode)
	assert.Equal(t, 3, attempts) // first try + 2 retries
}

func TestListReposIteratesAcrossPages(t *testing.T) {
	t.Parallel()
	pages := [][]byte{
		[]byte(`{"repos":[{"id":"1","forge":"github","identifier":"a/a","tokenMasked":"****1","ingestionStatus":"active"}],"hasMore":true}`),
		[]byte(`{"repos":[{"id":"2","forge":"github","identifier":"b/b","tokenMasked":"****2","ingestionStatus":"active"}],"hasMore":false}`),
	}
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(pages[call])
		call++
	}))
	t.Cleanup(srv.Close)

	client, err := pipelineanalytics.New(srv.URL)
	require.NoError(t, err)

	var ids []string
	it := client.ListRepos(context.Background(), nil)
	for repo := range it.All() {
		ids = append(ids, repo.Id)
	}

	require.NoError(t, it.Err())
	assert.Equal(t, []string{"1", "2"}, ids)
}
