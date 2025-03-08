package routes

import (
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
)

type ServeStrategy struct {
	Theme *theme.Themes

	SessionsManager sablier.Sablier
	StrategyConfig  config.Strategy
	SessionsConfig  config.Sessions
}
