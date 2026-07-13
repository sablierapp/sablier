// Command openapigen generates the Sablier OpenAPI 3.0 specification.
//
// It parses the swaggo annotations in internal/api into a Swagger 2.0 document
// in memory, then converts it to OpenAPI 3.0 with kin-openapi and writes the
// result. Intended to be run via `go generate ./...` / `make openapi`, so the
// contract never drifts from the handler annotations.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/swaggo/swag"
)

func main() {
	dir := flag.String("dir", "internal/api", "package directory to scan for annotations")
	general := flag.String("g", "doc.go", "file holding the general API annotations")
	out := flag.String("out", "", "output file path (writes to stdout if empty)")
	flag.Parse()

	// Parse the swaggo annotations into a Swagger 2.0 document. Route swag's
	// progress logs to stderr so stdout carries only the spec JSON; the
	// `make check-openapi` freshness test pipes stdout into `diff`.
	p := swag.New(swag.SetParseDependency(1), swag.SetDebugger(log.New(os.Stderr, "", log.LstdFlags)))
	if err := p.ParseAPI(*dir, *general, 100); err != nil {
		log.Fatalf("parse api annotations: %v", err)
	}
	v2JSON, err := json.Marshal(p.GetSwagger())
	if err != nil {
		log.Fatalf("marshal swagger 2.0: %v", err)
	}

	// Convert Swagger 2.0 -> OpenAPI 3.0.
	var doc2 openapi2.T
	if err := json.Unmarshal(v2JSON, &doc2); err != nil {
		log.Fatalf("load swagger 2.0: %v", err)
	}
	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		log.Fatalf("convert to openapi 3.0: %v", err)
	}

	// swag emits enums (e.g. for time.Duration's internal constants) in a
	// non-deterministic order and adds x-enum-* noise. Normalise so the output
	// is byte-stable (required for the check-openapi freshness test).
	normalize(doc3)

	if err := doc3.Validate(context.Background()); err != nil {
		log.Fatalf("validate openapi 3.0: %v", err)
	}

	data, err := json.MarshalIndent(doc3, "", "  ")
	if err != nil {
		log.Fatalf("marshal openapi 3.0: %v", err)
	}
	data = append(data, '\n')

	if *out == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	log.Printf("OpenAPI %s spec written to %s", doc3.OpenAPI, *out)
}

// normalize makes the generated document byte-stable across runs: it sorts
// every schema enum and removes swag's non-standard x-enum-* extensions.
func normalize(doc *openapi3.T) {
	if doc.Components == nil {
		return
	}
	for _, ref := range doc.Components.Schemas {
		normalizeSchema(ref)
	}
}

func normalizeSchema(ref *openapi3.SchemaRef) {
	// Only descend into inline schemas; named components are handled by the caller.
	if ref == nil || ref.Ref != "" || ref.Value == nil {
		return
	}
	s := ref.Value
	if len(s.Enum) > 0 {
		if s.Type != nil && (s.Type.Is("integer") || s.Type.Is("number")) {
			// Drop numeric enums: swag emits Go's internal constants (e.g. for
			// time.Duration) with duplicate values in a non-deterministic order,
			// and they are meaningless in the API contract.
			s.Enum = nil
		} else {
			sort.Slice(s.Enum, func(i, j int) bool {
				return fmt.Sprintf("%v", s.Enum[i]) < fmt.Sprintf("%v", s.Enum[j])
			})
		}
	}
	delete(s.Extensions, "x-enum-varnames")
	delete(s.Extensions, "x-enum-descriptions")
	for _, p := range s.Properties {
		normalizeSchema(p)
	}
	normalizeSchema(s.Items)
	for _, a := range s.AllOf {
		normalizeSchema(a)
	}
	for _, a := range s.AnyOf {
		normalizeSchema(a)
	}
	for _, a := range s.OneOf {
		normalizeSchema(a)
	}
	if s.AdditionalProperties.Schema != nil {
		normalizeSchema(s.AdditionalProperties.Schema)
	}
}
