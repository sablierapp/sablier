package routes

import (
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/config"
	"github.com/sablierapp/sablier/pkg/theme"
)

type ServeStrategy struct {
	Theme *theme.Themes

	SessionsManager sessions.Manager
	StrategyConfig  config.Strategy
	SessionsConfig  config.Sessions
}
