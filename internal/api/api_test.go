package api

import (
	"github.com/gin-gonic/gin"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/internal/api/apitest"
	config2 "github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/theme"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func NewApiTest(t *testing.T) (app *gin.Engine, router *gin.RouterGroup, strategy *ServeStrategy, mock *apitest.MockSablier) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	th, err := theme.New(slogt.New(t))
	assert.NilError(t, err)

	app = gin.New()
	router = app.Group("/api")
	mock = apitest.NewMockSablier(ctrl)
	strategy = &ServeStrategy{
		Theme:          th,
		Sablier:        mock,
		StrategyConfig: config2.NewStrategyConfig(),
		SessionsConfig: config2.NewSessionsConfig(),
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
