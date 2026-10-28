package client

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestEveryParamIsEncoded sets every field of each generated params struct
// and checks it reaches the query under its spec name, so a parameter type
// that encodeQuery does not handle fails here rather than in a plugin.
func TestEveryParamIsEncoded(t *testing.T) {
	for _, params := range []any{StartDynamicParams{}, StartBlockingParams{}, StartPokeParams{}, StreamInstanceEventsParams{}} {
		v := reflect.New(reflect.TypeOf(params)).Elem()
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			value := reflect.New(field.Type().Elem())
			if value.Elem().Kind() == reflect.Slice {
				value.Elem().Set(reflect.MakeSlice(value.Elem().Type(), 1, 1))
			}
			field.Set(value)
		}

		q := encodeQuery(v.Interface())

		for i := 0; i < v.NumField(); i++ {
			name, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("form"), ",")
			if _, ok := q[name]; !ok {
				t.Errorf("%s.%s is not sent as %q", v.Type().Name(), v.Type().Field(i).Name, name)
			}
		}
	}
}

// TestPathsMatchSpec checks the hand-written endpoint paths against the
// OpenAPI spec the types are generated from, by operationId.
func TestPathsMatchSpec(t *testing.T) {
	raw, err := os.ReadFile("../docs/static/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	pathOf := map[string]string{}
	for path, ops := range spec.Paths {
		if op, ok := ops["get"]; ok {
			pathOf[op.OperationID] = path
		}
	}

	for operationID, path := range map[string]string{
		"startDynamic":         pathStartDynamic,
		"startBlocking":        pathStartBlocking,
		"startPoke":            pathStartPoke,
		"streamInstanceEvents": pathStreamInstanceEvents,
	} {
		if pathOf[operationID] != path {
			t.Errorf("%s: client uses %q, spec says %q", operationID, path, pathOf[operationID])
		}
	}
}
