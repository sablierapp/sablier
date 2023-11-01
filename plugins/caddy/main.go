package caddy

import (
	"context"
	"io"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(SablierMiddleware{})
}

type SablierMiddleware struct {
	Config  Config
	client  *http.Client
	request *http.Request
}

// CaddyModule returns the Caddy module information.
func (SablierMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.sablier",
		New: func() caddy.Module { return new(SablierMiddleware) },
	}
}

// Provision implements caddy.Provisioner.
func (m *SablierMiddleware) Provision(ctx caddy.Context) error {
	req, err := m.Config.BuildRequest()

	if err != nil {
		return err
	}

	m.request = req
	m.client = &http.Client{}

	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m SablierMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request, next caddyhttp.Handler) error {
	sablierRequest := m.request.Clone(context.TODO())

	resp, err := m.client.Do(sablierRequest)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return nil
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Sablier-Session-Status") == "ready" {
		next.ServeHTTP(rw, req)
	} else {
		forward(resp, rw)
	}
	return nil
}

func forward(resp *http.Response, rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	rw.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	io.Copy(rw, resp.Body)
}

// Interface guards
var (
	_ caddy.Provisioner           = (*SablierMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*SablierMiddleware)(nil)
)
