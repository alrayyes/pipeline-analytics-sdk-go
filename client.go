// Package pipelineanalytics is a client for the pipeline-analytics REST
// API (github.com/alrayyes/pipeline-analytics).
//
// Every operation except GetVersion and the two webhook receivers requires
// an authenticated session. pipeline-analytics authenticates browsers with
// WebAuthn, not an API token -- there is no headless credential-grant flow
// in the spec. Obtain a session cookie by logging into the dashboard in a
// browser and copying the "session" cookie's value; pass it to New or set
// PIPELINE_ANALYTICS_SESSION. See the README for the full explanation and
// an example.
package pipelineanalytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alrayyes/pipeline-analytics-sdk-go/internal/genclient"
)

// SessionCookieEnvVar is the environment variable New falls back to when
// no session cookie is passed explicitly via WithSessionCookie.
const SessionCookieEnvVar = "PIPELINE_ANALYTICS_SESSION"

// Client is a pipeline-analytics API client. Construct one with New.
type Client struct {
	*genclient.ClientWithResponses
}

type config struct {
	sessionCookie string
	httpClient    genclient.HttpRequestDoer
	retry         retryConfig
}

// Option configures a Client constructed by New.
type Option func(*config)

// WithSessionCookie sets the session cookie sent on every request,
// overriding PIPELINE_ANALYTICS_SESSION.
func WithSessionCookie(cookie string) Option {
	return func(c *config) { c.sessionCookie = cookie }
}

// WithHTTPClient replaces the underlying HTTP client. The retry transport
// (WithRetry) wraps whatever Transport this client has set, so pass a
// client with a custom Transport already configured (proxies, mTLS, ...)
// rather than replacing RoundTrip yourself afterward.
func WithHTTPClient(hc genclient.HttpRequestDoer) Option {
	return func(c *config) { c.httpClient = hc }
}

// WithRetry overrides the default retry policy. maxRetries is the number
// of additional attempts after the first; base is the starting backoff
// before jitter and any server-sent Retry-After.
func WithRetry(maxRetries int, base time.Duration) Option {
	return func(c *config) { c.retry = retryConfig{maxRetries: maxRetries, base: base} }
}

// New builds a Client against baseURL (the pipeline-analytics instance's
// origin, e.g. "https://pipeline-analytics.example.com").
func New(baseURL string, opts ...Option) (*Client, error) {
	cfg := config{
		sessionCookie: os.Getenv(SessionCookieEnvVar),
		retry:         defaultRetryConfig,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	base := cfg.httpClient
	if base == nil {
		base = &http.Client{}
	}
	httpClient, ok := base.(*http.Client)
	if !ok {
		// A caller-supplied HttpRequestDoer that isn't a *http.Client (a
		// test double, usually) skips the retry transport -- there's no
		// http.RoundTripper to wrap.
		httpClient = nil
	}

	doer := base
	if httpClient != nil {
		wrapped := *httpClient
		wrapped.Transport = &retryTransport{
			next:   transportOrDefault(httpClient.Transport),
			config: cfg.retry,
		}
		doer = &wrapped
	}

	genOpts := []genclient.ClientOption{
		genclient.WithHTTPClient(doer),
		genclient.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			if cfg.sessionCookie != "" {
				// Secure/HttpOnly/SameSite are Set-Cookie response
				// attributes; they don't apply to a Cookie header we send.
				req.AddCookie(&http.Cookie{Name: "session", Value: cfg.sessionCookie}) //nolint:gosec // outgoing request cookie, not a response Set-Cookie
			}

			return nil
		}),
	}

	gc, err := genclient.NewClientWithResponses(baseURL, genOpts...)
	if err != nil {
		return nil, fmt.Errorf("build pipeline-analytics client: %w", err)
	}

	return &Client{ClientWithResponses: gc}, nil
}

func transportOrDefault(t http.RoundTripper) http.RoundTripper {
	if t != nil {
		return t
	}

	return http.DefaultTransport
}

// APIError is returned by DecodeError for any pipeline-analytics response
// carrying an error body -- every 4xx/5xx in the spec shares the same
// {code, message} shape.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("pipeline-analytics: %d %s: %s (request %s)", e.StatusCode, e.Code, e.Message, e.RequestID)
	}

	return fmt.Sprintf("pipeline-analytics: %d %s: %s", e.StatusCode, e.Code, e.Message)
}

// DecodeError builds an *APIError from a generated response's status,
// body and headers, or returns nil when statusCode isn't an error.
// Every operation's generated response type carries these three fields
// under the same names (StatusCode via HTTPResponse, Body, and the
// headers on HTTPResponse), so this one helper covers all of them:
//
//	resp, err := client.ListReposWithResponse(ctx, nil)
//	if err != nil {
//		return err // transport failure, already retried
//	}
//	if apiErr := pipelineanalytics.DecodeError(resp.HTTPResponse.StatusCode, resp.Body, resp.HTTPResponse.Header); apiErr != nil {
//		return apiErr
//	}
func DecodeError(statusCode int, body []byte, headers http.Header) *APIError {
	if statusCode < 400 {
		return nil
	}
	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &parsed)

	return &APIError{
		StatusCode: statusCode,
		Code:       parsed.Code,
		Message:    parsed.Message,
		RequestID:  headers.Get("X-Request-Id"),
	}
}
