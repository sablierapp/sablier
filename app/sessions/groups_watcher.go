package sessions

import (
	"context"
	"time"

	"github.com/acouvreur/sablier/app/providers"
	log "github.com/sirupsen/logrus"
)

// watchGroups watches indefinitely for new groups
func watchGroups(ctx context.Context, provider providers.Provider, frequency time.Duration, send chan<- map[string][]string) {
	ticker := time.NewTicker(frequency)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			groups, err := provider.GetGroups(ctx)
			if err != nil {
				log.Warn("could not get groups", err)
			} else {
				send <- groups
			}
		}
	}
}
