package api

import (
	config2 "github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
)

type ServeStrategy struct {
	Theme *theme.Themes

	Sablier        sablier.Sablier
	StrategyConfig config2.Strategy
	SessionsConfig config2.Sessions
}
