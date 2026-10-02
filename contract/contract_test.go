//go:build contract

// Package contract runs the client against a Prism mock server generated
// from pipeline-analytics's own pinned spec (see .github/workflows/ci.yml's
// "contract" job) -- never a hand-rolled stub, per
// rules/sdk-generation.md's "testing against the spec, not a hand-written
// stub". This proves the client's requests/responses conform to the
// spec; it says nothing about whether the real server still matches that
// spec.
package contract

import (
	"context"
	"net/http"
	"os"
	"testing"

	pipelineanalytics "github.com/alrayyes/pipeline-analytics-sdk-go"
)

func mustClient(t *testing.T) *pipelineanalytics.Client {
	t.Helper()
	baseURL := os.Getenv("PIPELINE_ANALYTICS_BASE_URL")
	if baseURL == "" {
		t.Fatal("PIPELINE_ANALYTICS_BASE_URL must point at a running Prism mock (see ci.yml's contract job)")
	}
	client, err := pipelineanalytics.New(baseURL, pipelineanalytics.WithSessionCookie("prism-does-not-check-this"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return client
}

func TestContract_GetVersion(t *testing.T) {
	client := mustClient(t)
	resp, err := client.GetVersionWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GetVersionWithResponse: %v", err)
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("GetVersionWithResponse: status %d, body %s", resp.HTTPResponse.StatusCode, resp.Body)
	}
}

func TestContract_ListPipelines(t *testing.T) {
	client := mustClient(t)
	resp, err := client.ListPipelinesWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListPipelinesWithResponse: %v", err)
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("ListPipelinesWithResponse: status %d, body %s", resp.HTTPResponse.StatusCode, resp.Body)
	}
}

func TestContract_GetGitHubRateLimitInsights(t *testing.T) {
	client := mustClient(t)
	resp, err := client.GetGitHubRateLimitInsightsWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GetGitHubRateLimitInsightsWithResponse: %v", err)
	}
	if resp.HTTPResponse.StatusCode != http.StatusOK {
		t.Fatalf("GetGitHubRateLimitInsightsWithResponse: status %d, body %s", resp.HTTPResponse.StatusCode, resp.Body)
	}
}
