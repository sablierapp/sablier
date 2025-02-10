package api

import (
	"github.com/gin-gonic/gin"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/app/http/routes"
	"github.com/sablierapp/sablier/app/sessions/sessionstest"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/theme"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func NewApiTest(t *testing.T) (app *gin.Engine, router *gin.RouterGroup, strategy *routes.ServeStrategy, mock *sessionstest.MockManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	th, err := theme.New(slogt.New(t))
	assert.NilError(t, err)

	app = gin.New()
	router = app.Group("/api")
	mock = sessionstest.NewMockManager(ctrl)
	strategy = &routes.ServeStrategy{
		Theme:           th,
		SessionsManager: mock,
		StrategyConfig:  config.NewStrategyConfig(),
		SessionsConfig:  config.NewSessionsConfig(),
	}

	return app, router, strategy, mock
}

// PerformRequest runs an API request with an empty request body.
func PerformRequest(r http.Handler, method, path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w
}
