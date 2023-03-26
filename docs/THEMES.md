# Creating your own loading theme

- [Creating your own loading theme](#creating-your-own-loading-theme)
  - [Custom themes locations](#custom-themes-locations)
  - [Create a custom theme](#create-a-custom-theme)
    - [Available Go Template Values](#available-go-template-values)
    - [The `<meta http-equiv="refresh" />` tag](#the-meta-http-equivrefresh--tag)
  - [The `showDetails` option](#the-showdetails-option)
  - [The embedded themes](#the-embedded-themes)
  - [How to load my custom theme](#how-to-load-my-custom-theme)
  - [See the available themes from the API](#see-the-available-themes-from-the-api)

## Custom themes locations

You can use the argument `--strategy.dynamic.coustom-themes` to define the location to search upon starting.

By default, the docker image looks for theme inside the `/etc/sablier/themes` folder.

```yaml
version: 3.9

services:
  sablier:
    image: acouvreur/sablier:1.3.0
    volumes:
      - '/var/run/docker.sock:/var/run/docker.sock'
      - '/path/to/my/themes:/etc/sablier/themes'
```

It will look recursively for themes with the `.html` extension.

- You **cannot** load new themes added in the folder without restarting
  - Why? Because we build a theme whitelist in order to prevent malicious payload crafting by using `theme=../../very_secret.txt`
- You **can** modify the existing themes files

## Create a custom theme

Themes are served using [Go Templates](https://pkg.go.dev/text/template).

### Available Go Template Values

| Template Key                                  | Template Value                                                                                                      | Go Template Usage                                                                        |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `.DisplayName`                                | The display name configured for the session                                                                         | `{{ .DisplayName }}`                                                                     |
| `.InstanceStates`                             | An array of `RenderOptionsInstanceState` that represents the state of each required instances                       | `{{- range $i, $instance := .InstanceStates }}{{ end -}}`                                |
| `.SessionDuration`                            | The humanized session duration from a [time.Duration](https://pkg.go.dev/time#Duration)                             | `{{ .SessionDuration }}`                                                                 |
| `.RefreshFrequency`                           | The refresh frequency for the page. See [The `<meta http-equiv="refresh" />` tag](#the-meta-http-equivrefresh--tag) | `<meta http-equiv="refresh" content="{{ .RefreshFrequency }}" />`                        |
| `.Version`                                    | Sablier version as a string                                                                                         | `{{ .Version }}`                                                                         |
| `$RenderOptionsInstanceState.Name`            | The name of the instance loading                                                                                    | `{{- range $i, $instance := .InstanceStates }}{{ $instance.Name }}{{ end -}}`            |
| `$RenderOptionsInstanceState.CurrentReplicas` | The number of current replicas of the instance loading                                                              | `{{- range $i, $instance := .InstanceStates }}{{ $instance.CurrentReplicas }}{{ end -}}` |
| `$RenderOptionsInstanceState.DesiredReplicas` | The number of desired replicas of the instance loading                                                              | `{{- range $i, $instance := .InstanceStates }}{{ $instance.DesiredReplicas }}{{ end -}}` |
| `$RenderOptionsInstanceState.Status`          | The status of the instance loading, `ready` or `not-ready`                                                          | `{{- range $i, $instance := .InstanceStates }}{{ $instance.Status }}{{ end -}}`          |
| `$RenderOptionsInstanceState.Error`           | The error trigger by this instance which won't be able to load                                                      | `{{- range $i, $instance := .InstanceStates }}{{ $instance.Error }}{{ end -}}`           |

### The `<meta http-equiv="refresh" />` tag

The auto refresh is done using the [HTML <meta> http-equiv Attribute](https://www.w3schools.com/tags/att_meta_http_equiv.asp).

> Defines a time interval for the document to refresh itself.

So the first step to create your own theme is to include the `HTML <meta> http-equiv Attribute` as follows:

```html
<head>
  ...
  <meta http-equiv="refresh" content="{{ .RefreshFrequency }}" />
  ...
</head>
```

## The `showDetails` option

If `showDetails` is set to `false` then the `.InstanceStates` will be an empty array.

## The embedded themes

See the [embedded themes](../app/http/pages/themes/).

## How to load my custom theme

You can load themes by specifying their name and their relative path from the `--strategy.dynamic.custom-themes-path` value.

```bash
/my/custom/themes/
├── custom1.html      # custom1
├── custom2.html      # custom2
└── special
    └── secret.html   # special/secret
```

Such as 

```shell
curl 'http://localhost:10000/api/strategies/dynamic?session_duration=1m&names=nginx&theme=custom1'
```

## See the available themes from the API

```
> curl 'http://localhost:10000/api/strategies/dynamic/themes'
```
```json
{
  "custom": [
    "custom"
  ],
  "embedded": [
    "ghost",
    "hacker-terminal",
    "matrix",
    "shuffle"
  ]
}
```