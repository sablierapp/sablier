---
title: Configure the server
description: Set the Sablier server's options via a config file, environment variables or CLI flags.
weight: 105
---

There are three ways to define configuration options for the Sablier server:

1. [In a configuration file](/tutorials/configuration/#configuration-file)
2. [As environment variables](/tutorials/configuration/#environment-variables)
3. [As command-line arguments](/tutorials/configuration/#arguments)

These methods are evaluated in the order listed above, with later methods overriding earlier ones. If no value is provided for a given option, a default value is used.

{{< callout type="info" >}}
This page covers **server** options. To configure the workloads Sablier manages, see [Labels & annotations](/concepts/configuring-instances/) and the [Label reference](/reference/labels/).
{{< /callout >}}

## Configuration file

At startup, Sablier searches for a configuration file named `sablier.yml` (or `sablier.yaml`) in the following locations:

- `/etc/sablier/`
- `$XDG_CONFIG_HOME/`
- `$HOME/.config/`
- `.` *(the working directory)*

You can override this with the `configFile` argument:

```bash
sablier --configFile=path/to/myconfigfile.yml
```

A minimal file looks like this:

```yaml
provider:
  name: docker
server:
  port: 10000
sessions:
  default-duration: 5m
```

Every option is listed in the [**CLI reference**](/reference/cli/), with its type, default, and the environment variable or flag it maps to. The reference is generated from the code and never drifts. A full annotated sample is available at [sablier.sample.yaml](https://raw.githubusercontent.com/sablierapp/sablier/main/sablier.sample.yaml).

## Environment variables

All configuration options can be set as environment variables. The variable name follows the structure of the configuration file: upper-case the dotted key, replace `.` and `-` with `_`, and add the `SABLIER_` prefix.

For example, this configuration:

```yaml
strategy:
  dynamic:
    custom-themes-path: /my/path
```

Becomes:

```bash
SABLIER_STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH=/my/path
```

## Arguments

To get the list of all available arguments:

<!-- x-release-please-start-version -->
```bash
sablier --help

# or

docker run sablierapp/sablier:1.14.0 --help
```
<!-- x-release-please-end -->

All configuration options can be used as command-line arguments. The argument name follows the structure of the configuration file.

For example, the configuration above becomes:

```bash
sablier start --strategy.dynamic.custom-themes-path /my/path
```

## Reference

The complete, auto-generated list of options lives in the [**CLI reference**](/reference/cli/). Each option lists its environment variable, type, default, "since" version, and description.
