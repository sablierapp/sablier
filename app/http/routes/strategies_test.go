package routes

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
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
}

func (s *SessionsManagerMock) RequestSession(names []string, duration time.Duration) *sessions.SessionState {
	return &s.SessionState
}

func (s *SessionsManagerMock) RequestReadySession(names []string, duration time.Duration, timeout time.Duration) (*sessions.SessionState, error) {
	return &s.SessionState, nil
}

func (s *SessionsManagerMock) LoadSessions(io.ReadCloser) error {
	return nil
}
func (s *SessionsManagerMock) SaveSessions(io.WriteCloser) error {
	return nil
}

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
			name: "return HTML Theme",
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
			name: "temporary redirect when session is ready",
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
