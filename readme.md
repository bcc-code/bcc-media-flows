# BCC Media - Flows

This repository contains definitions and code for execution of flows.

We use [Temporal IO](https://temporal.io) for managing and running workflows for Media pipelines.
To start a local server, you can use:

```
temporal server start-dev
```

## [Worker](/cmd/worker)

This package is responsible for executing the different workflows.

## [Http Trigger](/cmd/httpin)

This package allows HTTP Calls to trigger a subset of workflows.

## [Trigger UI](/cmd/trigger_ui)

This package contains a UI for starting a subset of workflows with specified parameters.

## Search attributes

Workflows are stamped with two custom search attributes so runs can be found in the
Temporal UI or CLI:

- `VXID` (Keyword) — the Vidispine item the flow operates on. For ingest flows the
  asset is created mid-workflow, so the attribute is upserted once the placeholder exists.
- `TriggeredBy` (Keyword) — who or what started the flow. httpin accepts a `triggeredBy`
  request parameter; the Trigger UI reads the header named by the `TRIGGERED_BY_HEADER`
  env var (e.g. `X-Forwarded-User` from an identity-aware proxy) and falls back to
  `trigger-ui`; watchers stamp `watcher`.

Example queries:

```
temporal workflow list --query "VXID='VX-1234'"
temporal workflow list --query "TriggeredBy='someone@bcc.media'"
```

The legacy `CustomStringField` attribute is still written alongside `VXID` so existing
queries keep working.

The worker registers both attributes on the namespace at startup (idempotent, via the
operator API), so starting the worker once against a fresh server is enough. To register
them manually instead:

```
temporal operator search-attribute create --name VXID --type Keyword
temporal operator search-attribute create --name TriggeredBy --type Keyword
```

or pass them to the dev server directly:

```
temporal server start-dev --search-attribute "VXID=Keyword" --search-attribute "TriggeredBy=Keyword"
```

Note the deploy order on a fresh namespace: the worker must have started once (or the
attributes been created manually) before httpin/trigger_ui start stamping workflows,
otherwise workflow starts are rejected server-side.
