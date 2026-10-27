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
- Link the first mention of each platform concept in every section (label,
  Deployment, service account…) to the platform's official documentation, so
  readers know exactly which concept is meant. Use inline links: reference-style
  links do not resolve inside tabs, callouts or details blocks.
- Show configuration with the example shortcode, from a runnable file under
  examples/<provider>/, never with a hand-copied snippet. Use a
  "# region <name>" / "# endregion" pair to show only part of a file.

Writing style:

1. Example first, then explain: one intro line, the excerpt, then short notes.
2. One idea per sentence. Rules a reader must remember go in a list, with the
   rule in bold first and the reason after it.
3. At most two code spans per sentence; full keys belong in the example.
4. The rule before the reason.
5. Edge cases (legacy keys, precedence) go in a collapsed details block.
6. Cut sentences that do not change what the reader does.

docs/content/providers/kubernetes.md is the reference page.
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
