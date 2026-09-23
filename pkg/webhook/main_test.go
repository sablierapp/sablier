package webhook_test

import (
	"testing"

	"github.com/sablierapp/sablier/internal/leakcheck"
)

func TestMain(m *testing.M) {
	leakcheck.Main(m)
}
