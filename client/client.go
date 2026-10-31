package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// SessionStatusHeader is the response header in which the strategy endpoints
// report whether the session is ready.
const SessionStatusHeader = "X-Sablier-Session-Status"

// Values of SessionStatusHeader.
const (
	SessionStatusReady    = "ready"
	SessionStatusNotReady = "not-ready"
)

// Paths of the strategy endpoints, checked against the spec by TestPathsMatchSpec.
const (
	pathStartDynamic  = "/api/strategies/dynamic"
	pathStartBlocking = "/api/strategies/blocking"
	pathStartPoke     = "/api/strategies/poke"
)

// Client calls the Sablier API. It is safe for concurrent use.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	userAgent  string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used to call Sablier. It defaults to
// http.DefaultClient. Timeouts are the caller's choice: set them on the HTTP
// client or on the context passed to each call.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

// WithUserAgent sets the User-Agent header sent to Sablier. It defaults to
// "sablier-go-client/" followed by Version.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) { c.userAgent = userAgent }
}

// New returns a client for the Sablier server at baseURL, such as
// "http://sablier:10000". baseURL may include the path prefix Sablier is
// served under.
func New(baseURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Sablier URL %q: %w", baseURL, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid Sablier URL %q: want http(s)://host[:port][/path]", baseURL)
	}

	c := &Client{
		baseURL:    u,
		httpClient: http.DefaultClient,
		userAgent:  "sablier-go-client/" + Version,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Response is Sablier's HTTP response. Its body has already been read, and
// Body reads it again from the start, so callers can forward it as is. The
// exception is the response to Events, whose Body is the event stream.
type Response struct {
	*http.Response
}

// Ready reports whether Sablier says the session is ready. Error responses
// carry no session status and are never ready.
func (r *Response) Ready() bool {
	return r.Header.Get(SessionStatusHeader) == SessionStatusReady
}

// StartDynamic requests a session and returns the themed waiting page.
func (c *Client) StartDynamic(ctx context.Context, params StartDynamicParams) ([]byte, *Response, error) {
	resp, body, err := c.get(ctx, pathStartDynamic, params)
	if err != nil {
		return nil, resp, err
	}
	return body, resp, nil
}

// StartBlocking requests a session and waits until it is ready or the
// timeout elapses.
func (c *Client) StartBlocking(ctx context.Context, params StartBlockingParams) (*ApiSessionResponse, *Response, error) {
	return c.getSession(ctx, pathStartBlocking, params)
}

// StartPoke requests a session and returns its status without waiting.
func (c *Client) StartPoke(ctx context.Context, params StartPokeParams) (*ApiSessionResponse, *Response, error) {
	return c.getSession(ctx, pathStartPoke, params)
}

// getSession calls an endpoint that answers with a session.
func (c *Client) getSession(ctx context.Context, path string, params any) (*ApiSessionResponse, *Response, error) {
	resp, body, err := c.get(ctx, path, params)
	if err != nil {
		return nil, resp, err
	}
	var session ApiSessionResponse
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, resp, fmt.Errorf("decode session: %w", err)
	}
	return &session, resp, nil
}

// get calls an endpoint and reads its body. The Response is nil only when
// Sablier could not be reached; a non-2xx answer is an *ErrorResponse.
func (c *Client) get(ctx context.Context, path string, params any) (*Response, []byte, error) {
	httpResp, err := c.do(ctx, path, params, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, body, err := readResponse(httpResp)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, checkResponse(resp, body)
}

// do sends a GET request to an endpoint and returns the unread response.
func (c *Client) do(ctx context.Context, path string, params any, header http.Header) (*http.Response, error) {
	u := c.baseURL.JoinPath(path)
	u.RawQuery = encodeQuery(params).Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for name, values := range header {
		req.Header[name] = values
	}
	req.Header.Set("User-Agent", c.userAgent)

	return c.httpClient.Do(req)
}

// readResponse reads and closes the body of httpResp, then makes Body read
// the same bytes again.
func readResponse(httpResp *http.Response) (*Response, []byte, error) {
	body, err := io.ReadAll(httpResp.Body)
	_ = httpResp.Body.Close()
	httpResp.Body = io.NopCloser(bytes.NewReader(body))
	resp := &Response{Response: httpResp}
	if err != nil {
		return resp, nil, fmt.Errorf("read sablier response: %w", err)
	}
	return resp, body, nil
}

// encodeQuery turns a generated *Params struct into query parameters, named
// after each field's form tag (the parameter name in the spec). Unset fields
// are omitted: the server applies its defaults only to absent parameters.
// Arrays are sent as one parameter per element (names=a&names=b).
func encodeQuery(params any) url.Values {
	q := url.Values{}
	v := reflect.ValueOf(params)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("form"), ",")
		field := v.Field(i)
		if name == "" || field.IsNil() {
			continue
		}
		// Not `switch value := field.Elem(); value.Kind()`: Yaegi, which
		// Traefik runs plugins with, matches no case in that form.
		value := field.Elem()
		switch value.Kind() {
		case reflect.String:
			q.Add(name, value.String())
		case reflect.Bool:
			q.Add(name, strconv.FormatBool(value.Bool()))
		case reflect.Slice:
			for j := 0; j < value.Len(); j++ {
				q.Add(name, value.Index(j).String())
			}
		default:
			// The spec gained a parameter type this encoder does not know;
			// TestEveryParamIsEncoded fails before this can ship.
			panic(fmt.Sprintf("client: cannot encode %s parameter %q", value.Kind(), name))
		}
	}
	return q
}
