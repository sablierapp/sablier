package version

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"text/template"
)

// Build information. Populated at build-time.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion = runtime.Version()
)

// versionInfoTmpl contains the template used by Info.
var versionInfoTmpl = `
{{.program}}, version {{.version}} (branch: {{.branch}}, revision: {{.revision}})
  build user:       {{.buildUser}}
  build date:       {{.buildDate}}
  go version:       {{.goVersion}}
  platform:         {{.platform}}
`

// Print returns version information.
func Print(program string) string {
	m := map[string]string{
		"program":   program,
		"version":   Version,
		"revision":  Revision,
		"branch":    Branch,
		"buildUser": BuildUser,
		"buildDate": BuildDate,
		"goVersion": GoVersion,
		"platform":  runtime.GOOS + "/" + runtime.GOARCH,
	}
	t := template.Must(template.New("version").Parse(versionInfoTmpl))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "version", m); err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}

// Info returns version, branch and revision information.
func Info() string {
	return fmt.Sprintf("(version=%s, branch=%s, revision=%s)", Version, Branch, Revision)
}

// BuildContext returns goVersion, buildUser and buildDate information.
func BuildContext() string {
	return fmt.Sprintf("(go=%s, user=%s, date=%s)", GoVersion, BuildUser, BuildDate)
}
