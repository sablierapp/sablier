package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/acouvreur/sablier/version"
	"github.com/gin-gonic/gin"
	"gotest.tools/v3/assert"
)

func TestGetVersion(t *testing.T) {

	version.Branch = "testing"
	version.Revision = "8ffebca"

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	expected, _ := json.Marshal(version.Map())

	GetVersion(c)
	res := recorder.Result()
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)

	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Equal(t, string(data), string(expected))

}
