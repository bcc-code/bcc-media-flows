# BCC Media - Flows

This repository contains definitions and code for execution of flows.

We use [Temporal IO](https://temporal.io) for managing and running workflows for Media pipelines.

## [Worker](/cmd/worker)

This package is responsible for executing the different workflows.

## [Http Trigger](/cmd/httpin)

This package allows HTTP Calls to trigger a subset of workflows.

## [Trigger UI](/cmd/trigger_ui)

This package contains a UI for starting a subset of workflows with specified parameters.
