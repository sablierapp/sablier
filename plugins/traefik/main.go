package traefik

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
	"time"
)

type WebSocketEvent int

const (
	WebSocketRead WebSocketEvent = iota
	WebSocketWrite
	WebSocketClose
)

var wsEventChan = make(chan WebSocketEvent, 10)

type SablierMiddleware struct {
	client      *http.Client
	request     *http.Request
	next        http.Handler
	useRedirect bool
	config      *Config
}

// New function creates the configuration
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	req, err := config.BuildRequest(name)
	if err != nil {
		return nil, err
	}

	return &SablierMiddleware{
		request: req,
		client:  &http.Client{},
		next:    next,
		// there is no way to make blocking work in traefik without redirect so let's make it default
		useRedirect: config.Blocking != nil,
		config:      config,
	}, nil
}

func (sm *SablierMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	sablierRequest := sm.request.Clone(context.TODO())
	fmt.Println("=== sablierRequest", sablierRequest)

	resp, err := sm.client.Do(sablierRequest)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	conditonalResponseWriter := newResponseWriter(rw)

	if isWebsocketRequest(req) {
		// FIXME dynamic make no sense for websocket since client return error
		fmt.Println("=== websocket request")
		go monitorWebSocketActivity(sablierRequest, sm)
		conditonalResponseWriter.websocket = true
	}

	useRedirect := false

	if resp.Header.Get("X-Sablier-Session-Status") == "ready" {
		// Check if the backend already received request data
		trace := &httptrace.ClientTrace{
			WroteHeaders: func() {
				conditonalResponseWriter.ready = true
			},
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				conditonalResponseWriter.ready = true
			},
		}
		newCtx := httptrace.WithClientTrace(req.Context(), trace)
		sm.next.ServeHTTP(conditonalResponseWriter, req.WithContext(newCtx))
		useRedirect = sm.useRedirect
	}

	if conditonalResponseWriter.ready == false {
		conditonalResponseWriter.ready = true
		if useRedirect {
			conditonalResponseWriter.Header().Set("Location", req.URL.String())

			status := http.StatusFound
			if req.Method != http.MethodGet {
				status = http.StatusTemporaryRedirect
			}

			conditonalResponseWriter.WriteHeader(status)
			_, err := conditonalResponseWriter.Write([]byte(http.StatusText(status)))
			if err != nil {
				http.Error(conditonalResponseWriter, err.Error(), http.StatusInternalServerError)
			}
		} else {
			conditonalResponseWriter.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			io.Copy(conditonalResponseWriter, resp.Body)
		}
	}
}

func newResponseWriter(rw http.ResponseWriter) *responseWriter {
	return &responseWriter{
		responseWriter: rw,
		headers:        make(http.Header),
	}
}

type responseWriter struct {
	responseWriter http.ResponseWriter
	headers        http.Header
	ready          bool
	websocket      bool
}

func (r *responseWriter) Header() http.Header {
	if r.ready {
		return r.responseWriter.Header()
	}
	return r.headers
}

func (r *responseWriter) Write(buf []byte) (int, error) {
	if r.ready == false {
		return len(buf), nil
	}
	return r.responseWriter.Write(buf)
}

func (r *responseWriter) WriteHeader(code int) {
	// TODO need to check for code 101? Is it possible that after error connection won't be websocket
	if code != http.StatusSwitchingProtocols {
		r.websocket = false
	}
	fmt.Println("=== code", code)
	if r.ready == false && code == http.StatusServiceUnavailable {
		// We get a 503 HTTP Status Code when there is no backend server in the pool
		// to which the request could be sent.  Also, note that r.ready
		// will never return false in case there was a connection established to
		// the backend server and so we can be sure that the 503 was produced
		// inside Traefik already
		return
	}

	headers := r.responseWriter.Header()
	for header, value := range r.headers {
		headers[header] = value
	}

	r.responseWriter.WriteHeader(code)
}

func (r *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.responseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("%T is not a http.Hijacker", r.responseWriter)
	}
	if r.websocket {
		fmt.Println("=== hijack for websocket")
		conn, bufio, err := hijacker.Hijack()
		return newConnWrapper(conn), bufio, err
	} else {
		return hijacker.Hijack()
	}
}

func (r *responseWriter) Flush() {
	if flusher, ok := r.responseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func isWebsocketRequest(req *http.Request) bool {
	return containsHeader(req, "Connection", "upgrade") && containsHeader(req, "Upgrade", "websocket")
}

func containsHeader(req *http.Request, name, value string) bool {
	items := strings.Split(req.Header.Get(name), ",")
	for _, item := range items {
		if value == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func newConnWrapper(c net.Conn) *conn {
	return &conn{
		conn: c,
	}
}

type conn struct {
	conn net.Conn
}

// LocalAddr implements net.Conn.
func (c *conn) LocalAddr() net.Addr {
	panic("unimplemented")
}

// RemoteAddr implements net.Conn.
func (c *conn) RemoteAddr() net.Addr {
	panic("unimplemented")
}

// SetDeadline implements net.Conn.
func (c *conn) SetDeadline(t time.Time) error {
	panic("unimplemented")
}

// SetReadDeadline implements net.Conn.
func (c *conn) SetReadDeadline(t time.Time) error {
	panic("unimplemented")
}

// SetWriteDeadline implements net.Conn.
func (c *conn) SetWriteDeadline(t time.Time) error {
	panic("unimplemented")
}

func (c *conn) Read(b []byte) (n int, err error) {
	n, err = c.conn.Read(b)
	if err == nil {
		wsEventChan <- WebSocketRead // Notify about the read operation
	}
	return
}

func (c *conn) Write(b []byte) (n int, err error) {
	n, err = c.conn.Write(b)
	if err == nil {
		wsEventChan <- WebSocketWrite // Notify about the write operation
	}
	return
}

func (c *conn) Close() error {
	err := c.conn.Close()
	wsEventChan <- WebSocketClose // Notify about the close operation
	return err
}

func monitorWebSocketActivity(sablierRequest *http.Request, sm *SablierMiddleware) {
	duration, err := time.ParseDuration(sm.config.SessionDuration)
	if err != nil {
		fmt.Fprintln(os.Stdout, []any{`Error parsing sessionDuration: %v`, err}...)
		return
	}
	alertTime := duration - (duration * 5 / 100) // Calculate alert time at 95% of the total duration
	alertTicker := time.NewTicker(alertTime)
	defer alertTicker.Stop()

	// Active flag to determine if the ticker should be reset
	activeDuringAlert := false

	for {
		select {
		case event := <-wsEventChan:
			switch event {
			case WebSocketRead, WebSocketWrite:
				activeDuringAlert = true // Mark that there was activity during the alert period

			case WebSocketClose:
				fmt.Println("WebSocket closed")
				activeDuringAlert = false // Do not reset ticker on close
			}

		case <-alertTicker.C:
			if activeDuringAlert {
				fmt.Println("Continuing activity detected, resetting ticker")
				_, err := sm.client.Do(sablierRequest)
				if err != nil {
					fmt.Println("Error in sending request to update websocket alive to sablier", err)
				}

				alertTicker.Reset(alertTime) // Reset the ticker for another period
				activeDuringAlert = false    // Reset the activity flag for the next period

			} else {
				fmt.Println("No activity detected within the alert time,will scaling down")
				return
			}
		}
	}
}
