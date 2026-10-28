// Package client is a Go client for the Sablier HTTP API, generated from the
// OpenAPI spec (docs/static/openapi.json) with oapi-codegen.
//
// It is a separate module so reverse-proxy plugins can depend on it without
// pulling in Sablier's provider dependencies. Regenerate it with
// `make client` after changing the spec.
package client

//go:generate go tool -modfile=../tools.mod oapi-codegen -config oapi-codegen.yaml -o client.gen.go ../docs/static/openapi.json
