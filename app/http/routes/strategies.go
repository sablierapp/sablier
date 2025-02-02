package routes

import (
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/app/theme"
	"github.com/sablierapp/sablier/config"
)

type ServeStrategy struct {
	Theme *theme.Themes

	SessionsManager sessions.Manager
	StrategyConfig  config.Strategy
	SessionsConfig  config.Sessions
}
