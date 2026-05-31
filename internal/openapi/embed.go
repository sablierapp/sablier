package openapi

import _ "embed"

// Spec is the OpenAPI 3.1 specification for the Sablier HTTP API,
// embedded at build time from the YAML source file.
//
//go:embed openapi.yaml
var Spec []byte
