package pages

import (
	"bytes"

	"fmt"
	"html/template"
	"math"
	"time"
)

var loadingPage = `<!doctype html>
<html lang="en-US">

<head>
  <title>Ondemand - Loading</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />

  <meta http-equiv="refresh" content="5" />


  <link rel="shortcut icon"
    href="https://docs.traefik.io/assets/images/logo-traefik-proxy-logo.svg" />
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link rel="stylesheet"
    href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500&display=swap" />


  <style>
    * {
      box-sizing: border-box;
    }

    html {
      -ms-text-size-adjust: 100%;
      -webkit-text-size-adjust: 100%;
      font-family: 'Inter', 'system-ui', sans-serif;
      font-size: 62.5%;
      height: 100%;
      line-height: 1.15;
      margin: 0;
      min-height: 100vh;
      width: 100%;
    }

    body {
      align-items: center;
      background-color: #c7d0d9;
      background-position: center center;
      background-repeat: no-repeat;
      display: flex;
      flex-flow: column nowrap;
      margin: 0;
      min-height: 100vh;
      padding: 10rem 0 0 0;
      width: 100%;
    }

    img {
      border: 0;
    }

    a {
      background-color: transparent;
      color: inherit;
    }

    a:active,
    a:hover {
      outline: 0;
    }

    .text {
      color: #c7d0d9;
      font-size: 1.6rem;
      line-height: 2.4rem;
      text-align: center;
    }

    .header {
      position: relative;
    }


    .lds-ellipsis {
      display: inline-block;
      position: relative;
      width: 80px;
      height: 80px;
    }

    .lds-ellipsis div {
      position: absolute;
      top: 33px;
      width: 13px;
      height: 13px;
      border-radius: 50%;
      background: #fff;
      animation-timing-function: cubic-bezier(0, 1, 1, 0);
    }

    .lds-ellipsis div:nth-child(1) {
      left: 8px;
      animation: lds-ellipsis1 0.6s infinite;
    }

    .lds-ellipsis div:nth-child(2) {
      left: 8px;
      animation: lds-ellipsis2 0.6s infinite;
    }

    .lds-ellipsis div:nth-child(3) {
      left: 32px;
      animation: lds-ellipsis2 0.6s infinite;
    }

    .lds-ellipsis div:nth-child(4) {
      left: 56px;
      animation: lds-ellipsis3 0.6s infinite;
    }

    @keyframes lds-ellipsis1 {
      0% {
        transform: scale(0);
      }

      100% {
        transform: scale(1);
      }
    }

    @keyframes lds-ellipsis3 {
      0% {
        transform: scale(1);
      }

      100% {
        transform: scale(0);
      }
    }

    @keyframes lds-ellipsis2 {
      0% {
        transform: translate(0, 0);
      }

      100% {
        transform: translate(24px, 0);
      }
    }


    .logo {
      height: 4rem;
      left: 50%;
      position: absolute;
      top: 50%;
      transform: translateX(-50%) translateY(-50%);
      width: 4rem;
    }

    .panel {
      align-items: center;
      background-color: #212124;
      border-radius: 2px;
      display: flex;
      flex-flow: column nowrap;
      margin: 6rem 0 0 0;
      max-width: 100vw;
      padding: 3.5rem 6.2rem;
      width: 56rem;
    }

    .panel>*:not(:last-child) {
      margin: 0 0 4rem;
    }

    .headline {
      color: #ffffff;
      font-size: 3.2rem;
      font-weight: 500;
      line-height: 5rem;
      margin: 0 0 1.3rem;
      text-align: center;
    }

    .footer {
      bottom: 1rem;
      left: 0;
      position: fixed;
      width: 100%;
      font-size: small;
    }

    @media (max-width: 56rem) {
      body {
        background-image: none;
        padding: 0;
      }

      .panel {
        margin: 0;
        padding: 2rem;
      }

      .panel>*:not(:last-child) {
        margin-bottom: 2rem;
      }

      .footer {
        margin: 2rem 0;
        padding: 2rem;
        position: initial;
      }
    }
  </style>
</head>

<body>
  <header class="header">
    <img
      src="https://docs.traefik.io/assets/images/logo-traefik-proxy-logo.svg">
  </header>

  <section class="panel">
    <h2 class="headline" id="headline">{{ .Name }} is loading...</h2>
    <div class="lds-ellipsis"><div></div><div></div><div></div><div></div></div>

    <p class="message text" id="message">Your instance is loading, and will be
      ready shortly.</p>


    <div class="support text">
      Your instance will shutdown automatically after {{ .Timeout }} of
      inactivity.
    </div>
  </section>

  <footer class="footer text">
    <a href="https://github.com/acouvreur/traefik-ondemand-plugin"
      target="_blank">acouvreur/traefik-ondemand-plugin</a>
  </footer>
</body>

</html>`

type LoadingData struct {
	Name    string
	Timeout string
}

func GetLoadingPage(name string, timeout time.Duration) string {
	tpl, err := template.New("loading").Parse(loadingPage)
	if err != nil {
		return err.Error()
	}
	b := bytes.Buffer{}
	tpl.Execute(&b, LoadingData{
		Name:    name,
		Timeout: humanizeDuration(timeout),
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
		if remainingSeconds > 0 {
			return fmt.Sprintf("%d minutes %d seconds", int64(duration.Minutes()), int64(remainingSeconds))
		}
		return fmt.Sprintf("%d minutes", int64(duration.Minutes()))
	}
	if duration.Hours() < 24.0 {
		remainingMinutes := math.Mod(duration.Minutes(), 60)
		remainingSeconds := math.Mod(duration.Seconds(), 60)

		if remainingMinutes > 0 {
			if remainingSeconds > 0 {
				return fmt.Sprintf("%d hours %d minutes %d seconds", int64(duration.Hours()), int64(remainingMinutes), int64(remainingSeconds))
			}
			return fmt.Sprintf("%d hours %d minutes", int64(duration.Hours()), int64(remainingMinutes))
		}
		return fmt.Sprintf("%d hours", int64(duration.Hours()))
	}
	remainingHours := math.Mod(duration.Hours(), 24)
	remainingMinutes := math.Mod(duration.Minutes(), 60)
	remainingSeconds := math.Mod(duration.Seconds(), 60)
	return fmt.Sprintf("%d days %d hours %d minutes %d seconds",
		int64(duration.Hours()/24), int64(remainingHours),
		int64(remainingMinutes), int64(remainingSeconds))
}
