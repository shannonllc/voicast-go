# Changelog

## 0.1.0 (unreleased)

- First release. Same API coverage as the Node.js SDK 0.1.0 (without the browser helper `voicast/web`). Standard library only, Go 1.21+, every call takes a `context.Context`.
- Agent tools for third-party APIs: `AgentTool` has `Method` (`GET` or `POST`), `Sign` (`voicast.Bool(false)` sends the arguments as they are), `Auth` (`*AgentToolAuth`; the key is stored encrypted and never returned) and `Confirm`. `Agent.ToolKeys` lists the stored keys (tool name and last 4 characters) on `Create`, `Get` and `Update`.
- `Tools.Try(ctx, ToolTryParams)` calls a saved (`ToolName`) or unsaved (`Tool`) tool once and returns a `ToolTryResult` (`OK`, `Result`, `Error`, `Status`, `Body`, `DurationMS`). Not retried on a 5xx or a connection error. `ToolCallRequest.Test` is true for these requests, `Call.Channel` is `ChannelTest` and `Call.Tenant` (now `*EventTenant`) can be nil.
- `AgentConnectorEntry.Actions` lists action names; `"*"` is no longer accepted.
- `NewClient(apiKey, WithBaseURL, WithTimeout, WithMaxRetries, WithHTTPClient)`: API key from the argument or `VOICAST_API_KEY`, default base URL `https://api.voicast.jp`. Retries 429 (honouring `retry-after` and `x-ratelimit-reset`, up to one minute) and, for requests safe to repeat (GET, PATCH, DELETE), 5xx, timeouts and connection errors, with exponential backoff and jitter. Every method takes `WithRequestTimeout` and `WithRequestMaxRetries`.
- Agents: `Agents.Create`, `List`, `Get(ctx, id, *AgentGetParams)`, `Update`, `Versions`, `Delete`.
- Tenants: `Tenants.Create` (returns the connection secret once), `List`, `Get`, `Update` (`AgentID: Null[string]()` removes the agent), `RotateSecret`, `Delete`, `ListConnections`, `CreateConnectLink`.
- Calls: `Calls.List` (one page) and `Calls.ListAll` (an `Iter` over every call), `Get`, `GetByProvider`, `Transcript`.
- Sessions for Web and app voice: `Sessions.Create`.
- Usage: `Usage.Get`.
- Connectors and connections: `Connectors.List`, `Connections.List` (`TenantID: Null[string]()` for the environment's connections), `Create`, `Get`, `Update`, `Options`, `Delete`.
- Webhook portal: `Webhooks.CreatePortalLink`.
- Billing: `Billing.Get` (balance, auto-recharge, price per minute) and `Billing.CreateTopup` (a Stripe Checkout URL). Tools signing secret: `Tools.GetSecret` and `Tools.RotateSecret`. `Call.BilledYen` is the fee for the call.
- Errors: `*Error` with `Kind`, `Status`, `Code`, `Message`, `Details`, `RequestID` and `RetryAfter`, and sentinels for `errors.Is` (`ErrAuthentication`, `ErrPaymentRequired`, `ErrPermission`, `ErrNotFound`, `ErrConflict`, `ErrValidation`, `ErrRateLimit`, `ErrAPI`, `ErrConnection`, `ErrTimeout`, `ErrWebhookVerification`, `ErrInvalidArgument`).
- `VerifyWebhook(payload, headers, secret)` returns an `Event`; `Event.ParseData()` gives the typed data (`*CallEndedData`, `*CallSummaryReadyData`, `*BillingBalanceLowData` and the others). `VerifyToolCall(payload, headers, secret)` verifies in-call tool requests and returns a `ToolCallRequest` (`BindArguments` decodes the arguments). `WithSecrets` accepts more secrets while you switch to a new one, `WithTolerance` changes the allowed clock difference. `NewWebhook` for lower-level use and `Sign` for tests.
