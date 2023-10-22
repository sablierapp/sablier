package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/acouvreur/sablier/app/http/routes/models"
	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/config"
	"github.com/gin-gonic/gin"
	"gotest.tools/v3/assert"
)

type SessionsManagerMock struct {
	SessionState sessions.SessionState
	sessions.Manager
}

func (s *SessionsManagerMock) RequestSession(names []string, duration time.Duration) *sessions.SessionState {
	return &s.SessionState
}

func (s *SessionsManagerMock) RequestReadySession(ctx context.Context, names []string, duration time.Duration, timeout time.Duration) (*sessions.SessionState, error) {
	return &s.SessionState, nil
}

func (s *SessionsManagerMock) LoadSessions(io.ReadCloser) error {
	return nil
}
func (s *SessionsManagerMock) SaveSessions(io.WriteCloser) error {
	return nil
}

func (s *SessionsManagerMock) Stop() {}

func TestServeStrategy_ServeDynamic(t *testing.T) {
	type arg struct {
		body    models.DynamicRequest
		session sessions.SessionState
	}
	tests := []struct {
		name                string
		arg                 arg
		expectedHeaderKey   string
		expectedHeaderValue string
	}{
		{
			name: "header has not ready value when not ready",
			arg: arg{
				body: models.DynamicRequest{
					Names:           []string{"nginx"},
					DisplayName:     "Test",
					Theme:           "hacker-terminal",
					SessionDuration: 1 * time.Minute,
				},
				session: sessions.SessionState{
					Instances: createMap([]*instance.State{
						{Name: "nginx", Status: instance.NotReady},
					}),
				},
			},
			expectedHeaderKey:   "X-Sablier-Session-Status",
			expectedHeaderValue: "not-ready",
		},
		{
			name: "header has ready value when session is ready",
			arg: arg{
				body: models.DynamicRequest{
					Names:           []string{"nginx"},
					DisplayName:     "Test",
					Theme:           "hacker-terminal",
					SessionDuration: 1 * time.Minute,
				},
				session: sessions.SessionState{
					Instances: createMap([]*instance.State{
						{Name: "nginx", Status: instance.Ready},
					}),
				},
			},
			expectedHeaderKey:   "X-Sablier-Session-Status",
			expectedHeaderValue: "ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s := &ServeStrategy{
				SessionsManager: &SessionsManagerMock{
					SessionState: tt.arg.session,
				},
				StrategyConfig: config.NewStrategyConfig(),
			}
			recorder := httptest.NewRecorder()
			c := GetTestGinContext(recorder)
			MockJsonPost(c, tt.arg.body)

			s.ServeDynamic(c)

			res := recorder.Result()
			defer res.Body.Close()

			assert.Equal(t, c.Writer.Header().Get(tt.expectedHeaderKey), tt.expectedHeaderValue)
		})
	}
}

func TestServeStrategy_ServeBlocking(t *testing.T) {
	type arg struct {
		body    models.BlockingRequest
		session sessions.SessionState
	}
	tests := []struct {
		name                string
		arg                 arg
		expectedBody        string
		expectedHeaderKey   string
		expectedHeaderValue string
	}{
		{
			name: "not ready returns session status not ready",
			arg: arg{
				body: models.BlockingRequest{
					Names:           []string{"nginx"},
					Timeout:         10 * time.Second,
					SessionDuration: 1 * time.Minute,
				},
				session: sessions.SessionState{
					Instances: createMap([]*instance.State{
						{Name: "nginx", Status: instance.NotReady, CurrentReplicas: 0, DesiredReplicas: 1},
					}),
				},
			},
			expectedBody:        `{"session":{"instances":[{"instance":{"name":"nginx","currentReplicas":0,"desiredReplicas":1,"status":"not-ready"},"error":null}],"status":"not-ready"}}`,
			expectedHeaderKey:   "X-Sablier-Session-Status",
			expectedHeaderValue: "not-ready",
		},
		{
			name: "ready returns session status ready",
			arg: arg{
				body: models.BlockingRequest{
					Names:           []string{"nginx"},
					SessionDuration: 1 * time.Minute,
				},
				session: sessions.SessionState{
					Instances: createMap([]*instance.State{
						{Name: "nginx", Status: instance.Ready, CurrentReplicas: 1, DesiredReplicas: 1},
					}),
				},
			},
			expectedBody:        `{"session":{"instances":[{"instance":{"name":"nginx","currentReplicas":1,"desiredReplicas":1,"status":"ready"},"error":null}],"status":"ready"}}`,
			expectedHeaderKey:   "X-Sablier-Session-Status",
			expectedHeaderValue: "ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s := &ServeStrategy{
				SessionsManager: &SessionsManagerMock{
					SessionState: tt.arg.session,
				},
				StrategyConfig: config.NewStrategyConfig(),
			}
			recorder := httptest.NewRecorder()
			c := GetTestGinContext(recorder)
			MockJsonPost(c, tt.arg.body)

			s.ServeBlocking(c)

			res := recorder.Result()
			defer res.Body.Close()

			bytes, err := io.ReadAll(res.Body)

			if err != nil {
				panic(err)
			}

			assert.Equal(t, c.Writer.Header().Get(tt.expectedHeaderKey), tt.expectedHeaderValue)
			assert.Equal(t, string(bytes), tt.expectedBody)
		})
	}
}

// mock gin context
func GetTestGinContext(w *httptest.ResponseRecorder) *gin.Context {
	gin.SetMode(gin.TestMode)

	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = &http.Request{
		Header: make(http.Header),
		URL:    &url.URL{},
	}

	return ctx
}

// mock getrequest
func MockJsonGet(c *gin.Context, params gin.Params, u url.Values) {
	c.Request.Method = "GET"
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	c.Request.URL.RawQuery = u.Encode()
}

func MockJsonPost(c *gin.Context, content interface{}) {
	c.Request.Method = "POST"
	c.Request.Header.Set("Content-Type", "application/json")

	jsonbytes, err := json.Marshal(content)
	if err != nil {
		panic(err)
	}

	// the request body must be an io.ReadCloser
	// the bytes buffer though doesn't implement io.Closer,
	// so you wrap it in a no-op closer
	c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonbytes))
}

func MockJsonPut(c *gin.Context, content interface{}, params gin.Params) {
	c.Request.Method = "PUT"
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params

	jsonbytes, err := json.Marshal(content)
	if err != nil {
		panic(err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonbytes))
}

func MockJsonDelete(c *gin.Context, params gin.Params) {
	c.Request.Method = "DELETE"
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
}

func createMap(instances []*instance.State) (store *sync.Map) {
	store = &sync.Map{}

	for _, v := range instances {
		store.Store(v.Name, sessions.InstanceState{
			Instance: v,
			Error:    nil,
		})
	}

	return
}

func TestNewServeStrategy(t *testing.T) {
	type args struct {
		sessionsManager sessions.Manager
		strategyConf    config.Strategy
		sessionsConf    config.Sessions
	}
	tests := []struct {
		name    string
		args    args
		osDirFS fs.FS
		want    map[string]bool
	}{
		{
			name: "load custom themes",
			args: args{
				sessionsManager: &SessionsManagerMock{},
				strategyConf: config.Strategy{
					Dynamic: config.DynamicStrategy{
						CustomThemesPath: "my/path/to/themes",
					},
				},
			},
			osDirFS: fstest.MapFS{
				"my/path/to/themes/marvel.html":    {Data: []byte("thor")},
				"my/path/to/themes/dc-comics.html": {Data: []byte("batman")},
			},
			want: map[string]bool{
				"marvel":    true,
				"dc-comics": true,
			},
		},
		{
			name: "load custom themes recursively",
			args: args{
				sessionsManager: &SessionsManagerMock{},
				strategyConf: config.Strategy{
					Dynamic: config.DynamicStrategy{
						CustomThemesPath: "my/path/to/themes",
					},
				},
			},
			osDirFS: fstest.MapFS{
				"my/path/to/themes/marvel.html":          {Data: []byte("thor")},
				"my/path/to/themes/dc-comics.html":       {Data: []byte("batman")},
				"my/path/to/themes/inner/dc-comics.html": {Data: []byte("batman")},
			},
			want: map[string]bool{
				"marvel":          true,
				"dc-comics":       true,
				"inner/dc-comics": true,
			},
		},
		{
			name: "do not load custom themes outside of path",
			args: args{
				sessionsManager: &SessionsManagerMock{},
				strategyConf: config.Strategy{
					Dynamic: config.DynamicStrategy{
						CustomThemesPath: "my/path/to/themes",
					},
				},
			},
			osDirFS: fstest.MapFS{
				"my/path/to/superman.html":               {Data: []byte("superman")},
				"my/path/to/themes/marvel.html":          {Data: []byte("thor")},
				"my/path/to/themes/dc-comics.html":       {Data: []byte("batman")},
				"my/path/to/themes/inner/dc-comics.html": {Data: []byte("batman")},
			},
			want: map[string]bool{
				"marvel":          true,
				"dc-comics":       true,
				"inner/dc-comics": true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			oldosDirFS := osDirFS
			defer func() { osDirFS = oldosDirFS }()

			myOsDirFS := func(dir string) fs.FS {
				fs, err := fs.Sub(tt.osDirFS, dir)

				if err != nil {
					panic(err)
				}

				return fs
			}

			osDirFS = myOsDirFS

			if got := NewServeStrategy(tt.args.sessionsManager, tt.args.strategyConf, tt.args.sessionsConf); !reflect.DeepEqual(got.customThemes, tt.want) {
				t.Errorf("NewServeStrategy() = %v, want %v", got.customThemes, tt.want)
			}
		})
	}
}

func TestServeStrategy_ServeDynamicThemes(t *testing.T) {
	type fields struct {
		StrategyConfig config.Strategy
		SessionsConfig config.Sessions
	}
	tests := []struct {
		name     string
		fields   fields
		osDirFS  fs.FS
		expected map[string]any
	}{
		{
			name: "load custom themes",
			fields: fields{StrategyConfig: config.Strategy{
				Dynamic: config.DynamicStrategy{
					CustomThemesPath: "my/path/to/themes",
				},
			}},
			osDirFS: fstest.MapFS{
				"my/path/to/superman.html":               {Data: []byte("superman")},
				"my/path/to/themes/marvel.html":          {Data: []byte("thor")},
				"my/path/to/themes/dc-comics.html":       {Data: []byte("batman")},
				"my/path/to/themes/inner/dc-comics.html": {Data: []byte("batman")},
			},
			expected: map[string]any{
				"custom": []any{
					"dc-comics",
					"inner/dc-comics",
					"marvel",
				},
				"embedded": []any{
					"ghost",
					"hacker-terminal",
					"matrix",
					"shuffle",
				},
			},
		},
		{
			name: "load without custom themes",
			expected: map[string]any{
				"custom": []any{},
				"embedded": []any{
					"ghost",
					"hacker-terminal",
					"matrix",
					"shuffle",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			oldosDirFS := osDirFS
			defer func() { osDirFS = oldosDirFS }()

			myOsDirFS := func(dir string) fs.FS {
				fs, err := fs.Sub(tt.osDirFS, dir)

				if err != nil {
					panic(err)
				}

				return fs
			}

			osDirFS = myOsDirFS

			s := NewServeStrategy(nil, tt.fields.StrategyConfig, tt.fields.SessionsConfig)

			recorder := httptest.NewRecorder()
			c := GetTestGinContext(recorder)

			s.ServeDynamicThemes(c)

			res := recorder.Result()
			defer res.Body.Close()

			jsonRes := make(map[string]interface{})
			err := json.NewDecoder(res.Body).Decode(&jsonRes)

			if err != nil {
				panic(err)
			}

			assert.DeepEqual(t, jsonRes, tt.expected)

		})
	}
}
