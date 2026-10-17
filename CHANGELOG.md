# Changelog

## 0.5.0 (2026-10-17)

- Voices are chosen by ID: `AgentConfig.Voice` takes a voice ID, such as `female-breezy` (default), `female-firm` or `male-calm`. Agents read from the API return these IDs. The doc comments and examples list only the IDs. The API still accepts the earlier voice names and saves the matching ID, so existing code keeps working; switch to the ID.
- `AgentTool.ConfirmMessage` and `AgentTool.DoneMessage`: for `Confirm` tools, voicast now asks the caller with a fixed confirmation message and tells the result in a fixed message. These templates set the messages; `{argument name}` inserts an argument value (dates and times are read as spoken dates).

## 0.4.0 (2026-10-16)

- Transfer on a handoff for SIP: `SipCredential.TransferTo` (always present, empty for none) is where the PBX transfers the call with SIP REFER when the AI hands off to a person (outcome `handoff`): an extension or number (digits and `*` `#` `+`, up to 32 characters) or a SIP URI (`sip:` / `sips:`, up to 200 characters). `SipCredentialCreateParams.TransferTo`.
- `Tenants.UpdateSipCredential(ctx, tenantID, credentialID, *SipCredentialUpdateParams)` (`PATCH /v1/tenants/:id/sip-credentials/:credential`, retried like other PATCH requests) changes only the fields that are set and returns the credentials without the password. `AgentID: Null[string]()` uses the tenant's agent; `TransferTo: Null[string]()` or `Value("")` turns the transfer off. Type `SipCredentialUpdateParams`.

## 0.3.0 (2026-10-16)

- SIP credentials for a PBX that sends calls to `sip.voicast.jp`: `Tenants.CreateSipCredential(ctx, tenantID, *SipCredentialCreateParams)` (`POST /v1/tenants/:id/sip-credentials`, returns the password once, not retried on a 5xx or a connection error), `Tenants.ListSipCredentials(ctx, tenantID)` and `Tenants.DeleteSipCredential(ctx, tenantID, credentialID)`. Types `SipCredential`, `SipCredentialWithPassword` and `SipCredentialCreateParams`.

## 0.2.0 (2026-10-12)

- Retention period for call records and recordings: `Retention.Get` (`GET /v1/retention`) and `Retention.Update(ctx, RetentionUpdateParams{RetentionDays: 7})` (`PATCH /v1/retention`, retried like other PATCH requests). `RetentionDays` is 1, 7, 14 or 30 (30 by default and the maximum).
- `UsageParams` takes `Channel`, `TenantID` and `ExternalID` filters, and `Usage.Filter` returns the filters applied.
- `ToolTryParams.Tool`: a definition must carry its own `Auth.Value`; the agent's stored API key is used only with `ToolName` (422 `missing_api_key`).

## 0.1.0 (2026-10-04)

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
