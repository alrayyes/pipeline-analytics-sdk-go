# pipeline-analytics-sdk-go

[![CI](https://github.com/alrayyes/pipeline-analytics-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/alrayyes/pipeline-analytics-sdk-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/alrayyes/pipeline-analytics-sdk-go.svg)](https://pkg.go.dev/github.com/alrayyes/pipeline-analytics-sdk-go)
[![Codecov](https://codecov.io/gh/alrayyes/pipeline-analytics-sdk-go/graph/badge.svg)](https://codecov.io/gh/alrayyes/pipeline-analytics-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A Go client for [pipeline-analytics](https://github.com/alrayyes/pipeline-analytics)'s
REST API, generated from its OpenAPI spec with
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen). It saves you
from hand-rolling HTTP requests, retries and pagination against the API
yourself.

## Requirements

- Go 1.27 or later.
- A running pipeline-analytics instance.
- A session cookie for that instance (see "Authentication" below) — every
  endpoint except `GetVersion` and the two webhook receivers needs one.

## Installation

```sh
go get github.com/alrayyes/pipeline-analytics-sdk-go@latest
```

Pin an exact tagged version (`@v0.1.0`) once one exists, rather than tracking
`@latest` in anything but a quick trial.

## Authentication

pipeline-analytics authenticates browsers with
[WebAuthn](https://webauthn.guide/), not an API token — there's no headless
credential-grant flow in its spec, so this SDK can't log in for you. Get a
session cookie by logging into the dashboard in a browser, opening dev
tools, and copying the `session` cookie's value. Pass it to `New` or set
`PIPELINE_ANALYTICS_SESSION` in the environment:

```go
client, err := pipelineanalytics.New("https://pipeline-analytics.example.com",
    pipelineanalytics.WithSessionCookie(os.Getenv("PIPELINE_ANALYTICS_SESSION")),
)
```

A session cookie expires the same way it would in a browser; there's
nothing in this SDK to refresh it automatically.

## Usage

`GetVersion` needs no session and is a good first call to prove the client
reaches the server at all:

```go
package main

import (
	"context"
	"fmt"
	"log"

	pipelineanalytics "github.com/alrayyes/pipeline-analytics-sdk-go"
)

func main() {
	client, err := pipelineanalytics.New("https://pipeline-analytics.example.com")
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.GetVersionWithResponse(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("server version:", resp.JSON200.Version)
}
```

Listing tracked repos needs a session, and demonstrates the pagination
iterator and error handling:

```go
client, err := pipelineanalytics.New(
	"https://pipeline-analytics.example.com",
	pipelineanalytics.WithSessionCookie(os.Getenv("PIPELINE_ANALYTICS_SESSION")),
)
if err != nil {
	log.Fatal(err)
}

it := client.ListRepos(context.Background(), nil)
for repo := range it.All() {
	fmt.Println(repo.Identifier, repo.IngestionStatus)
}
if err := it.Err(); err != nil {
	var apiErr *pipelineanalytics.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized {
		log.Fatal("session cookie expired or invalid")
	}
	log.Fatal(err)
}
```

Every other operation follows the generated `ClientWithResponses` pattern —
`client.<Operation>WithResponse(ctx, ...)` returns a typed response whose
`JSON200`/`JSON4xx` fields hold the decoded body. Use `DecodeError` to turn
a failed response into a `*pipelineanalytics.APIError` uniformly:

```go
resp, err := client.GetRepoUsageWithResponse(ctx, repoID, nil)
if err != nil {
	return err // transport failure -- already retried
}
if apiErr := pipelineanalytics.DecodeError(resp.HTTPResponse.StatusCode, resp.Body, resp.HTTPResponse.Header); apiErr != nil {
	return apiErr
}
```

The client retries a `429` or `5xx` response with exponential backoff and
jitter (honoring a server-sent `Retry-After`), and never retries any other
`4xx`. Tune it with `WithRetry`, or swap the underlying `*http.Client`
entirely with `WithHTTPClient`.

## Regenerating the client

See [CONTRIBUTING.md](CONTRIBUTING.md) — the generated code is pinned to a
specific pipeline-analytics commit and shouldn't drift from it silently.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for building, testing and the release
process.

## License

[MIT](LICENSE) — a permissive license for the client, independent of
pipeline-analytics' own AGPL-3.0.
