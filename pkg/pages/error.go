package pages

import (
	"bytes"
	"html/template"
)

var errorPage = `<!doctype html>
<html lang="en-US">

<head>
  <title>Ondemand - Error</title>
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
    <h2 class="headline" id="headline">Error loading {{.name}}.</h2>

    <p class="message text" id="message">There was an error loading your instance.</p>


    <div class="support text">
      {{.error}}
    </div>
  </section>

  <footer class="footer text">
    <a href="https://github.com/acouvreur/traefik-ondemand-plugin"
      target="_blank">acouvreur/traefik-ondemand-plugin</a>
  </footer>
</body>

</html>`

type ErrorData struct {
	name string
	err  string
}

func GetErrorPage(name string, e string) string {
	tpl, err := template.New("error").Parse(errorPage)
	if err != nil {
		return err.Error()
	}
	b := bytes.Buffer{}
	tpl.Execute(&b, ErrorData{
		name: name,
		err:  e,
	})
	return b.String()
}
