//go:build contract

// Package contract runs the client against a Prism mock server generated
// from pipeline-analytics's own pinned spec (see .github/workflows/ci.yml's
// "contract" job) -- never a hand-rolled stub, per
// rules/sdk-generation.md's "testing against the spec, not a hand-written
// stub". This proves the client's requests/responses conform to the
// spec; it says nothing about whether the real server still matches that
// spec.
//
// Operations are discovered by reflection over the generated client and
// called with generated arguments, not hand-written calls. A regeneration
// that adds a query parameter or a whole operation then needs no edit here:
// a hand-written call stopped compiling the day /api/pipelines gained
// optional parameters (#25).
package contract

import (
	"context"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	pipelineanalytics "github.com/alrayyes/pipeline-analytics-sdk-go"
)

// bodyOperations are operations that take a request body. The generic call
// has no valid body to send, and a zero one fails the mock's schema
// validation, so each needs a hand-written test of its own. An operation
// with a body that isn't listed here fails TestContract_EveryOperationIsAccountedFor,
// naming it, rather than going untested. Every entry is a known gap: none of
// these had a contract test before the generic one, either.
var bodyOperations = map[string]string{
	"AddCredential":    "needs a valid WebAuthn credential body",
	"DiscoverRepos":    "needs a valid discovery request body",
	"ForgejoWebhook":   "needs a valid webhook payload",
	"GithubWebhook":    "needs a valid webhook payload",
	"McpEndpoint":      "needs a valid JSON-RPC body",
	"RegisterRepo":     "needs a valid registration body",
	"UpdateSettings":   "needs a valid settings body",
	"WebauthnLogin":    "needs a valid assertion body",
	"WebauthnRegister": "needs a valid attestation body",
}

// enumSamples gives a valid value for each required enum parameter type, by
// Go type name: reflection can't list a string type's allowed values, and
// "sample" fails the mock's enum validation. A new required enum shows up as
// one operation failing with a 4xx, naming it; add its type here.
var enumSamples = map[string]string{
	"Forge": "github",
}

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

// operationNames lists every generated operation by name (without the
// WithResponse suffix), skipping the WithBody variants oapi-codegen emits
// beside a JSON-body operation.
func operationNames(client *pipelineanalytics.Client) []string {
	typ := reflect.TypeOf(client.ClientWithResponses)

	var names []string

	for i := range typ.NumMethod() {
		name := typ.Method(i).Name
		if strings.HasSuffix(name, "WithResponse") && !strings.Contains(name, "WithBody") {
			names = append(names, strings.TrimSuffix(name, "WithResponse"))
		}
	}

	return names
}

// takesBody reports whether the operation has a request-body parameter.
// Path parameters are strings and query parameters arrive as a pointer to a
// params struct, so anything passed by value as a struct, map or slice is a
// JSON body.
func takesBody(method reflect.Value) bool {
	typ := method.Type()

	for i := 1; i < typ.NumIn(); i++ {
		if typ.IsVariadic() && i == typ.NumIn()-1 {
			continue
		}

		switch typ.In(i).Kind() {
		case reflect.Struct, reflect.Map, reflect.Slice:
			return true
		default:
		}
	}

	return false
}

// sampleValue builds a value for one generated parameter. A pointer to a
// params struct becomes a populated struct: oapi-codegen makes optional
// query parameters pointers and required ones plain fields, so filling the
// plain fields satisfies the mock's required-parameter validation whatever
// the spec adds next.
func sampleValue(t *testing.T, op string, typ reflect.Type) reflect.Value {
	t.Helper()

	switch typ.Kind() {
	case reflect.String:
		value := reflect.New(typ).Elem()
		if sample, ok := enumSamples[typ.Name()]; ok {
			value.SetString(sample)
		} else {
			value.SetString("sample")
		}

		return value
	case reflect.Int, reflect.Int32, reflect.Int64:
		value := reflect.New(typ).Elem()
		value.SetInt(1)

		return value
	case reflect.Ptr:
		if typ.Elem().Kind() != reflect.Struct {
			return reflect.Zero(typ)
		}

		params := reflect.New(typ.Elem())

		for i := range params.Elem().NumField() {
			field := params.Elem().Field(i)
			if field.Kind() == reflect.String || field.CanInt() {
				field.Set(sampleValue(t, op, field.Type()))
			}
		}

		return params
	default:
		t.Fatalf("%s: no sample value for a parameter of type %s -- teach sampleValue about it", op, typ)

		return reflect.Value{}
	}
}

// call invokes <op>WithResponse with a sample value for every parameter after
// the context, and returns the response's status and body.
func call(t *testing.T, client *pipelineanalytics.Client, op string) (int, []byte) {
	t.Helper()

	method := reflect.ValueOf(client.ClientWithResponses).MethodByName(op + "WithResponse")
	typ := method.Type()

	args := []reflect.Value{reflect.ValueOf(context.Background())}

	for i := 1; i < typ.NumIn(); i++ {
		if typ.IsVariadic() && i == typ.NumIn()-1 {
			break // request editors: none
		}

		args = append(args, sampleValue(t, op, typ.In(i)))
	}

	results := method.Call(args)
	if err, _ := results[1].Interface().(error); err != nil {
		t.Fatalf("%sWithResponse: %v", op, err)
	}

	response := results[0].Elem()
	httpResponse, _ := response.FieldByName("HTTPResponse").Interface().(*http.Response)
	body, _ := response.FieldByName("Body").Interface().([]byte)

	if httpResponse == nil {
		t.Fatalf("%sWithResponse: no HTTP response", op)
	}

	return httpResponse.StatusCode, body
}

func TestContract_Operations(t *testing.T) {
	client := mustClient(t)

	for _, op := range operationNames(client) {
		if takesBody(reflect.ValueOf(client.ClientWithResponses).MethodByName(op + "WithResponse")) {
			continue // see bodyOperations
		}

		t.Run(op, func(t *testing.T) {
			status, body := call(t, client, op)
			if status < 200 || status > 299 {
				t.Fatalf("%sWithResponse: status %d, body %s", op, status, body)
			}
		})
	}
}

// TestContract_EveryOperationIsAccountedFor fails, naming the operation, when
// a regeneration adds one the generic test can't call and bodyOperations
// doesn't list -- so a new operation is never silently untested.
func TestContract_EveryOperationIsAccountedFor(t *testing.T) {
	client := mustClient(t)

	var unaccounted []string

	for _, op := range operationNames(client) {
		method := reflect.ValueOf(client.ClientWithResponses).MethodByName(op + "WithResponse")
		if _, listed := bodyOperations[op]; takesBody(method) && !listed {
			unaccounted = append(unaccounted, op)
		}
	}

	slices.Sort(unaccounted)

	if len(unaccounted) > 0 {
		t.Fatalf("operations with a request body and no contract test: %s -- write one in contract_test.go and list the operation in bodyOperations",
			strings.Join(unaccounted, ", "))
	}
}
