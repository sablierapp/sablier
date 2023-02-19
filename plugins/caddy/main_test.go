package caddy_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	plugin "github.com/acouvreur/sablier/plugins/caddy"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func TestSablierMiddleware_ServeHTTP(t *testing.T) {
	type fields struct {
		Next              caddyhttp.Handler
		SablierMiddleware *plugin.SablierMiddleware
	}
	type sablier struct {
		headers map[string]string
		body    string
	}
	tests := []struct {
		name     string
		fields   fields
		sablier  sablier
		expected string
	}{
		{
			name: "sablier service is ready",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					_, err := fmt.Fprint(w, "response from service")
					return err
				}),
				SablierMiddleware: &plugin.SablierMiddleware{
					Config: plugin.Config{
						SessionDuration: &oneMinute,
						Dynamic:         &plugin.DynamicConfiguration{},
					},
				},
			},
			expected: "response from service",
		},
		{
			name: "sablier service is not ready",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "not-ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					_, err := fmt.Fprint(w, "response from service")
					return err
				}),
				SablierMiddleware: &plugin.SablierMiddleware{
					Config: plugin.Config{
						SessionDuration: &oneMinute,
						Dynamic:         &plugin.DynamicConfiguration{},
					},
				},
			},
			expected: "response from sablier",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.sablier.headers {
					w.Header().Add(key, value)
				}
				w.Write([]byte(tt.sablier.body))
			}))
			defer sablierMockServer.Close()

			tt.fields.SablierMiddleware.Config.SablierURL = sablierMockServer.URL

			err := tt.fields.SablierMiddleware.Provision(caddy.Context{})
			if err != nil {
				panic(err)
			}

			req := httptest.NewRequest(http.MethodGet, "/my-nginx", nil)
			w := httptest.NewRecorder()

			tt.fields.SablierMiddleware.ServeHTTP(w, req, tt.fields.Next)

			res := w.Result()
			defer res.Body.Close()
			data, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("expected error to be nil got %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("expected %s got %v", tt.expected, string(data))
			}
		})
	}
}
