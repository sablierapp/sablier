// Command schemagen generates a JSON Schema for theme.TemplateData and writes
// it to the specified output file. It is intended to be run via go generate.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/sablierapp/sablier/pkg/theme"
)

func main() {
	out := flag.String("out", "", "output file path (writes to stdout if empty)")
	flag.Parse()

	r := &jsonschema.Reflector{
		// Inline the top-level struct rather than wrapping it in a $ref.
		ExpandedStruct: true,
	}

	schema := r.Reflect(&theme.TemplateData{})
	// Set a stable, canonical ID so theme authors can reference the schema by URL.
	schema.ID = "https://sablierapp.dev/theme.schema.json"

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')

	if *out == "" {
		os.Stdout.Write(data) //nolint:errcheck
		return
	}

	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	log.Printf("schema written to %s", *out)
}
