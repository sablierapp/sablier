package theme

import (
	"fmt"
)

type ErrThemeNotFound struct {
	Theme           string
	AvailableThemes []string
}

func (t ErrThemeNotFound) Error() string {
	return fmt.Sprintf("theme %s not found", t.Theme)
}
