// Package client is a Go client for the Sablier HTTP API.
//
// It uses only the standard library, so reverse-proxy plugins can depend on
// it without pulling in anything else. The request and response types in
// types.gen.go are generated from the OpenAPI spec (docs/static/openapi.json)
// with oapi-codegen; regenerate them with `make client` after changing the
// spec. client.go is the hand-written HTTP layer.
//
// Like go-github, each call returns its typed result, the *Response, and an
// error. The Response is nil only when Sablier could not be reached. When
// Sablier answers with a status other than 2xx, the error is an
// *ErrorResponse and the Response is still returned, with its body readable,
// so a proxy can forward it:
//
//	session, resp, err := c.StartBlocking(ctx, client.StartBlockingParams{Group: &group})
//	if errResp, ok := err.(*client.ErrorResponse); ok {
//		// Sablier answered errResp.Response.StatusCode, see errResp.Problem.
//	}
//
// The client is tested under Yaegi, the interpreter Traefik runs plugins
// with (see yaegitest).
package client

//go:generate go tool -modfile=../tools.mod oapi-codegen -config oapi-codegen.yaml -o types.gen.go ../docs/static/openapi.json
