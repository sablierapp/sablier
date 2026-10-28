// Package client is a Go client for the Sablier HTTP API.
//
// It uses only the standard library, so reverse-proxy plugins can depend on
// it without pulling in anything else. The request and response types in
// types.gen.go are generated from the OpenAPI spec (docs/static/openapi.json)
// with oapi-codegen; regenerate them with `make client` after changing the
// spec. client.go is the hand-written HTTP layer.
package client

//go:generate go tool -modfile=../tools.mod oapi-codegen -config oapi-codegen.yaml -o types.gen.go ../docs/static/openapi.json
