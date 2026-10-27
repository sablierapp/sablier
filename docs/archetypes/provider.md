---
title: "{{ replace .File.ContentBaseName "_" " " | title }}"
weight: 10
since: next
---

<!--
Provider page outline. Every provider page uses these sections in this order,
so readers find the same thing in the same place for every provider. Drop a
section only when it has nothing to say for this provider.

Create a page with: hugo new content providers/<name>.md --kind provider

- Set `since` in the front matter to the release that added the provider, and
  mark every setting heading with it: #### Group {since="v1.4.0"}
  (since="next" for an unreleased feature).
- Show configuration with the example shortcode, from a runnable file under
  examples/<provider>/, never with a hand-copied snippet. Use a
  "# region <name>" / "# endregion" pair to show only part of a file.
-->

## Introduction

<!-- What Sablier starts and stops on this platform, and how, in one paragraph. -->

## Install Sablier

<!-- Where Sablier runs on this platform, how to install it, and the access it needs. -->

## Usage example

<!-- One complete workload from examples/<provider>/, and how to request it. -->

## Provider reference

### Configuration

<!-- The --provider.<name>.* options, linking to the CLI reference. -->

### Workloads

<!-- Each kind of workload Sablier manages, one H4 each: how Sablier starts and stops it. -->

### Instance names

<!-- The name format a reverse proxy uses to target one workload. -->

### Labels

<!-- Where Sablier settings live on this platform, and their syntax. -->

### Workload settings

<!--
One H4 per supported setting, in this order: Enable, Group, Ready after, Ready on
start, Running hours, Anti-affinity, Scale instead of stop, Block I/O. Each has a
since attribute, an example excerpt, the provider specifics, and a link to the feature
page. Leave out unsupported settings and list them under Limitations.
-->

### Readiness

<!-- When Sablier considers a workload ready on this platform. -->

### Limitations

## Examples

<!-- Links to the runnable examples and guides for this provider. -->
