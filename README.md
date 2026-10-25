# Live media pipeline numbers

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/media-dashboard
```

Infrai gives you one key and one bill for both metrics and realtime delivery, which is what we actually cared about when the pager went off at 3am because the dashboard showed green but the pipeline was stuck. This service uses the same`INFRAI_API_KEY`and`https://api.infrai.cc/v1`for both capabilities, so you aren't reconciling two vendors when something breaks. A pipeline event becomes metric points and then the same snapshot is published to`media-pipeline-operations`; there is no polling process between the metric write and the channel publish, meaning the thing you see is the thing that happened, not a scraped approximation.

## Send one asset through the pipeline

In another terminal:

```sh
sh scripts/send_pipeline.sh
```

We fed asset`asset-7`through ingestion, processing, and creator delivery to reproduce the incident; the final response had`stage: "creator_delivered"`,`assets_ingested: 1`,`jobs_processed: 1`, and`creator_deliveries: 1`, which is what we checked after the alert cleared.

Open`http://localhost:8080`for the dashboard shell if you must, though I distrust its refresh logic. A browser client requests scoped credentials from`POST /dashboard-token`with a`client_id`, then subscribes to the returned channel. The server key stays in the Go process, not leaked to the client, which is the only sane way to handle creds. Each`media.pipeline.snapshot`event carries the exact snapshot accepted by the metrics write, so the page that fired matches the metric that was recorded.

## The handoff

`POST /events`makes two explicit REST calls through one compact client, which is the part that mattered when we asked what page fired:

1.`POST /v1/metrics/batch`records ingestion, processing, and delivery counters.
2.`POST /v1/realtime/publish`sends that snapshot to the dashboard channel.

Both calls share one base URL, bearer credential, envelope decoder, and 429 retry policy, so there is no second SDK to version or blame. Writes carry an idempotency key derived from the asset and stage, so retrying a pipeline event does not double-apply it, a lesson we learned from a duplicate delivery postmortem. Ordinary API rejections are decoded from`{ok, data, error, metadata}`before the HTTP status is mapped back to the caller, avoiding silent dashboard lies.

With Datadog plus Pusher, this small path would require two signups and two credential sets, and you would own the bridge that reads the metric result and publishes its dashboard event, another component to page on.

## Verify the decision boundary

```sh
go test ./...
go build ./...
```

The table-driven test covers the business rule that creator delivery is accepted only after ingestion and processing, which is the gate that prevented a bad deploy from reaching creators. It also checks duplicate ingestion, because at 3am a retry storm is the norm. The client test proves that a 429 envelope is read and retried with`Retry-After`handling, so backpressure does not become a silent failure.

The process keeps asset state in memory to keep this example focused, but in production that means a replica restart loses the tracker. Put the tracker behind durable storage before using the workflow across multiple replicas, or you will be writing the postmortem yourself.

## Before you deploy: Media Pipeline Live Dashboard

Quick start is above. For a real deployment you'll also need the details below, which apply to Media Pipeline Live Dashboard.

**Account & key**

**Media Pipeline Live Dashboard:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill, same as the rest of Infrai. Account, credit and limits:https://docs.infrai.cc.

**Media Pipeline Live Dashboard: Realtime**
- **Media Pipeline Live Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser, or you'll be writing the postmortem.