//go:build contract

package contract

import (
	"context"
	"net/http"
	"strings"
	"testing"

	pipelineanalytics "github.com/alrayyes/pipeline-analytics-sdk-go"
	"github.com/alrayyes/pipeline-analytics-sdk-go/internal/genclient"
)

// bodyCase is the contract test for one operation that takes a request body:
// a body valid against the spec's schema must get the documented success
// status, and a body that breaks the schema must get a 4xx from the mock,
// never a 2xx.
//
// invalid is raw JSON sent through the generated <op>WithBody variant, since
// the typed method can't express a body of the wrong shape. valid may be raw
// JSON too: a typed body stops compiling when a regeneration changes a
// field's optionality, which leaves main red instead of failing the spec.
type bodyCase struct {
	// wantStatus is the success status the spec documents for the operation.
	wantStatus int
	valid      func(context.Context, *pipelineanalytics.Client) (int, error)
	invalid    func(context.Context, *pipelineanalytics.Client, string) (int, error)
	// invalidBody breaks the operation's schema.
	invalidBody string
}

// bodyCases lists every operation that takes a request body, by generated
// operation name. TestContract_EveryOperationIsAccountedFor fails, naming it,
// when a regeneration adds a body operation that isn't here.
var bodyCases = map[string]bodyCase{
	"AddCredential": {
		wantStatus: http.StatusCreated,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.AddCredentialWithResponse(ctx, &genclient.AddCredentialParams{}, genclient.WebAuthnAttestationResponse{"id": "sample", "type": "public-key"})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.AddCredentialWithBodyWithResponse(ctx, &genclient.AddCredentialParams{}, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `[]`,
	},
	"DiscoverRepos": {
		wantStatus: http.StatusOK,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.DiscoverReposWithBodyWithResponse(ctx, jsonType, strings.NewReader(`{"forge":"github","token":"sample"}`))

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.DiscoverReposWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `{"forge":"nope"}`, // not a forge in the spec
	},
	"ForgejoWebhook": {
		wantStatus: http.StatusAccepted,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.ForgejoWebhookWithResponse(ctx, &genclient.ForgejoWebhookParams{XForgejoSignature: "sample", XForgejoEvent: "action_run_recover"}, genclient.ForgejoWebhookJSONRequestBody{"action": "completed"})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.ForgejoWebhookWithBodyWithResponse(ctx, &genclient.ForgejoWebhookParams{XForgejoSignature: "sample", XForgejoEvent: "action_run_recover"}, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `[]`,
	},
	"GithubWebhook": {
		wantStatus: http.StatusAccepted,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.GithubWebhookWithResponse(ctx, &genclient.GithubWebhookParams{XHubSignature256: "sha256=sample", XGitHubEvent: "workflow_run"}, genclient.GithubWebhookJSONRequestBody{"action": "completed"})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.GithubWebhookWithBodyWithResponse(ctx, &genclient.GithubWebhookParams{XHubSignature256: "sha256=sample", XGitHubEvent: "workflow_run"}, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `[]`,
	},
	"McpEndpoint": {
		wantStatus: http.StatusOK,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.McpEndpointWithResponse(ctx, genclient.McpEndpointJSONRequestBody{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.McpEndpointWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `[]`,
	},
	"SaveForgeToken": {
		wantStatus: http.StatusOK,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.SaveForgeTokenWithBodyWithResponse(ctx, jsonType, strings.NewReader(`{"forge":"github","token":"sample"}`))

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.SaveForgeTokenWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `{"forge":"github"}`, // token is required
	},
	"QuarantineStep": {
		wantStatus: http.StatusOK,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			note := "waits on the shared database"
			r, err := c.QuarantineStepWithResponse(ctx, "sample", "test", genclient.QuarantineStepJSONRequestBody{Note: &note})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.QuarantineStepWithBodyWithResponse(ctx, "sample", "test", jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `{"note":123}`, // a note is a string
	},
	"RegisterRepo": {
		wantStatus: http.StatusCreated,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.RegisterRepoWithBodyWithResponse(ctx, jsonType, strings.NewReader(`{"forge":"github","identifier":"alrayyes/pipeline-analytics","token":"sample"}`))

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.RegisterRepoWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `{"forge":"github","token":"sample"}`, // identifier is required
	},
	"UpdateSettings": {
		wantStatus: http.StatusOK,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			theme := genclient.SettingsUpdateThemeDark
			r, err := c.UpdateSettingsWithResponse(ctx, genclient.SettingsUpdate{Theme: &theme})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.UpdateSettingsWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `{"theme":"neon"}`, // not in the enum
	},
	"WebauthnLogin": {
		wantStatus: http.StatusOK,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.WebauthnLoginWithResponse(ctx, genclient.WebAuthnAssertionResponse{"id": "sample", "type": "public-key"})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.WebauthnLoginWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `[]`,
	},
	"WebauthnRegister": {
		wantStatus: http.StatusCreated,
		valid: func(ctx context.Context, c *pipelineanalytics.Client) (int, error) {
			r, err := c.WebauthnRegisterWithResponse(ctx, genclient.WebAuthnAttestationResponse{"id": "sample", "type": "public-key"})

			return statusOf(r, err)
		},
		invalid: func(ctx context.Context, c *pipelineanalytics.Client, body string) (int, error) {
			r, err := c.WebauthnRegisterWithBodyWithResponse(ctx, jsonType, strings.NewReader(body))

			return statusOf(r, err)
		},
		invalidBody: `[]`,
	},
}

const jsonType = "application/json"

// statusOf reads the HTTP status off a generated response, whatever its type,
// so each case's closure stays one line.
func statusOf(response interface{ StatusCode() int }, err error) (int, error) {
	if err != nil {
		return 0, err
	}

	return response.StatusCode(), nil
}

func TestContract_BodyOperations(t *testing.T) {
	client := mustClient(t)

	for op, tc := range bodyCases {
		t.Run(op+"/valid body", func(t *testing.T) {
			status, err := tc.valid(context.Background(), client)
			if err != nil {
				t.Fatalf("%s: %v", op, err)
			}

			if status != tc.wantStatus {
				t.Fatalf("%s with a valid body: status %d, want %d", op, status, tc.wantStatus)
			}
		})

		t.Run(op+"/invalid body", func(t *testing.T) {
			status, err := tc.invalid(context.Background(), client, tc.invalidBody)
			if err != nil {
				t.Fatalf("%s: %v", op, err)
			}

			if status < 400 || status > 499 {
				t.Fatalf("%s with the body %s: status %d, want a 4xx from the mock", op, tc.invalidBody, status)
			}
		})
	}
}
