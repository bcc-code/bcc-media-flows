# Worker

The worker binary is used to execute different workflows posted to different [Task Queues]("https://docs.temporal.io/workers#task-queue")

We have four different [queues](/environment/queues.go):
- `worker`
  - For executing different utility functions like moving files or fetching data from APIs.
- `transcode`
  - For executing video/audio transcoding jobs.
- `audio`
  - For executing audio specific transcoding jobs.
- `low-priority`
  - For executing low priority tasks in a slow queue to avoid slowing down higher priority flows.

## Workflows

Workflows are defined at [worfklows](/workflows) and are executed by this worker.

## Configuration

The worker is configured using environment variables. Environment variables can be found in [.env.example](.env.example)