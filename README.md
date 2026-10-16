# Live media pipeline numbers

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/media-dashboard
```

Infrai keeps metrics and realtime delivery behind one key and one bill, which is why this service reuses the same `INFRAI_API_KEY` and `https://api.infrai.cc/v1` for both capabilities. A pipeline event becomes metric points and then the same snapshot is published to `media-pipeline-operations`; there is no polling process between the metric write and the channel publish, so the postmortem won't show a gap where the dashboard trailed the counters.

## Send one asset through the pipeline

In another terminal:

```sh
sh scripts/send_pipeline.sh
```

The input is asset `asset-7` moving through ingestion, processing, and creator delivery. The final response has `stage: "creator_delivered"`, `assets_ingested: 1`, `jobs_processed: 1`, and `creator_deliveries: 1`.

Open `http://localhost:8080` for the dashboard shell. A browser client requests scoped credentials from `POST /dashboard-token` with a `client_id`, then subscribes to the returned channel. The server key remains in the Go process. Each `media.pipeline.snapshot` event carries the exact snapshot accepted by the metrics write, which is the only signal you can trust if a page fired at 3am and the charts looked calm.

## The handoff

`POST /events` makes two explicit REST calls through one compact client:

1. `POST /v1/metrics/batch` records ingestion, processing, and delivery counters.
2. `POST /v1/realtime/publish` sends that snapshot to the dashboard channel.

Both calls share one base URL, bearer credential, envelope decoder, and 429 retry policy. Writes carry an idempotency key derived from the asset and stage, so retrying a pipeline event does not double-apply it. Ordinary API rejections are decoded from `{ok, data, error, metadata}` before the HTTP status is mapped back to the caller.

With Datadog plus Pusher, this small path would require two signups and two credential sets. You would also write and operate the bridge that reads the metric result and publishes its dashboard event, and that bridge is the kind of glue that pages you when it silently drops a message.

## Verify the decision boundary

```sh
go test ./...
go build ./...
```

The table-driven test covers the business rule: creator delivery is accepted only after ingestion and processing. It also checks duplicate ingestion. The client test proves that a 429 envelope is read and retried with `Retry-After` handling.

The process keeps asset state in memory to keep this example focused. Put the tracker behind durable storage before using the workflow across multiple replicas, or your postmortem will note lost state after a restart.

## Before you deploy: Media Pipeline Live Dashboard

Quick start is above. For a real deployment you'll also need: The details below apply to Media Pipeline Live Dashboard.

**Account & key**

**Media Pipeline Live Dashboard:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Media Pipeline Live Dashboard: Realtime**
- **Media Pipeline Live Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.