package sabliercmd

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/sablierapp/sablier/pkg/config"
)

func TestSetupProviderInvalidConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if _, err := setupProvider(context.Background(), logger, config.Provider{Name: ""}); err == nil {
		t.Fatal("expected an error for an invalid provider configuration")
	}
}
