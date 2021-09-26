package pages

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"time"
)

// go:embed loading.html
var loadingPage string

type LoadingData struct {
	name    string
	timeout string
}

func GetLoadingPage(name string, timeout time.Duration) string {
	tpl, err := template.New("loading").Parse(loadingPage)
	if err != nil {
		return err.Error()
	}
	b := bytes.Buffer{}
	tpl.Execute(&b, LoadingData{
		name:    name,
		timeout: humanizeDuration(timeout),
	})
	return b.String()
}

// humanizeDuration humanizes time.Duration output to a meaningful value,
// golang's default ``time.Duration`` output is badly formatted and unreadable.
func humanizeDuration(duration time.Duration) string {
	if duration.Seconds() < 60.0 {
		return fmt.Sprintf("%d seconds", int64(duration.Seconds()))
	}
	if duration.Minutes() < 60.0 {
		remainingSeconds := math.Mod(duration.Seconds(), 60)
		return fmt.Sprintf("%d minutes %d seconds", int64(duration.Minutes()), int64(remainingSeconds))
	}
	if duration.Hours() < 24.0 {
		remainingMinutes := math.Mod(duration.Minutes(), 60)
		remainingSeconds := math.Mod(duration.Seconds(), 60)
		return fmt.Sprintf("%d hours %d minutes %d seconds",
			int64(duration.Hours()), int64(remainingMinutes), int64(remainingSeconds))
	}
	remainingHours := math.Mod(duration.Hours(), 24)
	remainingMinutes := math.Mod(duration.Minutes(), 60)
	remainingSeconds := math.Mod(duration.Seconds(), 60)
	return fmt.Sprintf("%d days %d hours %d minutes %d seconds",
		int64(duration.Hours()/24), int64(remainingHours),
		int64(remainingMinutes), int64(remainingSeconds))
}
