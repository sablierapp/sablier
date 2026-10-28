package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
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

const problemMediaType = "application/problem+json"

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

// WithUserAgent sets the User-Agent header sent to Sablier.
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
		userAgent:  "sablier-go-client",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Response is Sablier's answer to a strategy call. Any HTTP response,
// including 4xx and 5xx, is a Response: callers decide whether to forward it.
type Response struct {
	StatusCode int
	Header     http.Header
	// Body is the waiting page for the dynamic strategy, and JSON otherwise.
	Body []byte
}

// Ready reports whether Sablier says the session is ready. Error responses
// carry no session status and are never ready.
func (r *Response) Ready() bool {
	return r.Header.Get(SessionStatusHeader) == SessionStatusReady
}

// Session decodes the session returned by the blocking and poke strategies.
func (r *Response) Session() (*ApiSessionResponse, error) {
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sablier answered %d, not a session", r.StatusCode)
	}
	var session ApiSessionResponse
	if err := json.Unmarshal(r.Body, &session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &session, nil
}

// Problem decodes the RFC 7807 problem detail of an error response. It
// returns nil when the response is not a problem.
func (r *Response) Problem() *Rfc7807Problem {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != problemMediaType {
		return nil
	}
	var problem Rfc7807Problem
	if err := json.Unmarshal(r.Body, &problem); err != nil {
		return nil
	}
	return &problem
}

// StartDynamic requests a session and returns the themed waiting page.
func (c *Client) StartDynamic(ctx context.Context, params StartDynamicParams) (*Response, error) {
	return c.get(ctx, pathStartDynamic, params)
}

// StartBlocking requests a session and waits until it is ready or the
// timeout elapses.
func (c *Client) StartBlocking(ctx context.Context, params StartBlockingParams) (*Response, error) {
	return c.get(ctx, pathStartBlocking, params)
}

// StartPoke requests a session and returns its status without waiting.
func (c *Client) StartPoke(ctx context.Context, params StartPokeParams) (*Response, error) {
	return c.get(ctx, pathStartPoke, params)
}

// get calls a strategy endpoint. The error is only for failures to get an
// HTTP response: transport errors, cancellation, or an unreadable body.
func (c *Client) get(ctx context.Context, path string, params any) (*Response, error) {
	u := c.baseURL.JoinPath(path)
	u.RawQuery = encodeQuery(params).Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sablier response: %w", err)
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}, nil
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
		switch value := field.Elem(); value.Kind() {
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
