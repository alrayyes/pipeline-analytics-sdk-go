package pipelineanalytics

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

type retryConfig struct {
	maxRetries int
	base       time.Duration
}

var defaultRetryConfig = retryConfig{maxRetries: 3, base: 250 * time.Millisecond}

// retryTransport retries a 5xx or 429 response with exponential backoff
// and jitter, honoring a server-sent Retry-After ahead of its own
// schedule. It never retries any other 4xx -- those won't succeed on a
// second attempt, and retrying only delays the real error reaching the
// caller (rules/sdk-generation.md's "Client shape").
type retryTransport struct {
	next   http.RoundTripper
	config retryConfig
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; ; attempt++ {
		attemptReq := req
		if attempt > 0 {
			attemptReq, err = rewind(req)
			if err != nil {
				// The body can't be replayed (no GetBody) -- return
				// whatever the previous attempt produced rather than
				// risk sending a truncated or duplicate body.
				return resp, err
			}
		}

		resp, err = t.next.RoundTrip(attemptReq)
		if err != nil {
			err = fmt.Errorf("pipeline-analytics: round trip: %w", err)
		}

		if !t.shouldRetry(attempt, resp, err) {
			return resp, err
		}

		wait := retryDelay(resp, attempt, t.config.base)
		timer := time.NewTimer(wait)
		select {
		case <-req.Context().Done():
			timer.Stop()

			return resp, fmt.Errorf("pipeline-analytics: %w", req.Context().Err())
		case <-timer.C:
		}

		if resp != nil {
			_ = resp.Body.Close()
		}
	}
}

func (t *retryTransport) shouldRetry(attempt int, resp *http.Response, err error) bool {
	if attempt >= t.config.maxRetries {
		return false
	}
	if err != nil {
		// A network-level failure (connection reset, timeout) is
		// retry-worthy the same as a 5xx.
		return true
	}

	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
}

// rewind clones req with a fresh, unread body for a retry attempt.
func rewind(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return clone, nil
	}
	if req.GetBody == nil {
		return nil, errNonReplayableBody
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("pipeline-analytics: rewind request body for retry: %w", err)
	}

	clone.Body = body

	return clone, nil
}

var errNonReplayableBody = &nonReplayableBodyError{}

type nonReplayableBodyError struct{}

func (*nonReplayableBodyError) Error() string {
	return "pipeline-analytics: request body can't be replayed for a retry"
}

// retryDelay honors Retry-After (seconds or HTTP-date, per RFC 9110)
// ahead of the client's own exponential-backoff-with-jitter schedule.
func retryDelay(resp *http.Response, attempt int, base time.Duration) time.Duration {
	if resp != nil {
		if d, ok := retryAfter(resp.Header.Get("Retry-After")); ok {
			return d
		}
	}
	backoff := base * time.Duration(1<<attempt)
	jitter := time.Duration(rand.Int64N(int64(backoff) + 1)) //nolint:gosec // retry jitter, not security-sensitive

	return backoff/2 + jitter/2
}

func retryAfter(header string) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(header); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if when, err := http.ParseTime(header); err == nil {
		if d := time.Until(when); d > 0 {
			return d, true
		}

		return 0, true
	}

	return 0, false
}
