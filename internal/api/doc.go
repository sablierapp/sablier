// Package api implements the Sablier HTTP API.
//
// The annotations below are the source for the generated OpenAPI contract
// (see `make openapi` / cmd/openapigen); they are parsed by swaggo/swag.
//
// @title                     Sablier API
// @version                   1.0
// @description               The Sablier HTTP API. Reverse-proxy integrations call these endpoints to start instances on demand and check session readiness.
// @description               The two strategy endpoints are the heart of Sablier: `dynamic` serves a themed waiting page that self-refreshes, while `blocking` holds the request until the session is ready.
// @contact.name              Sablier
// @contact.url               https://github.com/sablierapp/sablier
// @license.name              AGPL-3.0
// @license.url               https://github.com/sablierapp/sablier/blob/main/LICENSE
// @externalDocs.description  Documentation
// @externalDocs.url          https://sablierapp.dev/
// @BasePath                  /
package api

//go:generate go run ../../cmd/openapigen -dir . -g doc.go -out ../../docs/static/openapi.json
