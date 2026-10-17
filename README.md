# voicast-go

Go SDK for [voicast](https://voicast.jp/), the headless conversational AI platform that handles reception and bookings by phone and on the Web.

- Standard library only.
- Go 1.21+. Every call takes a `context.Context`.
- Server-side only. To talk from a browser, create a session here and use `voicast/web` from the [Node.js SDK](https://www.npmjs.com/package/voicast) in the browser.

## Install

```sh
go get github.com/shannonllc/voicast-go
```

```go
import voicast "github.com/shannonllc/voicast-go"
```

## Quick start

```go
client, err := voicast.NewClient("vk_live_...") // or set VOICAST_API_KEY
if err != nil {
	log.Fatal(err)
}

// What the AI says and asks
agent, err := client.Agents.Create(ctx, voicast.AgentCreateParams{
	Name: "Reception",
	Config: voicast.AgentConfig{
		Greeting: "お電話ありがとうございます。○○歯科です。",
		Persona:  "丁寧で落ち着いた受付",
		Fields: []voicast.AgentField{
			{Key: "name", Label: "お名前", Required: true},
			{Key: "phone", Label: "電話番号", Type: "phone"},
		},
		HandoffWhen: []string{"急患", "担当者と話したいと言われた"},
		Closing:     "承りました。担当者から折り返しご連絡します。",
		Voice:       "female-firm",
	},
})

// The business that takes the calls (your customer)
tenant, err := client.Tenants.Create(ctx, voicast.TenantCreateParams{ExternalID: "clinic-123", Name: "○○歯科", AgentID: agent.ID})
// tenant.Secret (vts_...) is returned only here: pass it to your Twilio <Stream> as the `secret` parameter
```

Use the API key on your server only. Never ship it to a browser or an app.

## Talk from a browser or an app

Create a session on your server and pass only `ClientToken` and `WebRTCURL` to the browser or app.

```go
session, err := client.Sessions.Create(ctx, voicast.SessionCreateParams{TenantID: tenant.ID}) // Output: voicast.OutputText for text only
json.NewEncoder(w).Encode(map[string]string{"client_token": session.ClientToken, "webrtc_url": session.WebRTCURL})
```

The token expires in 2 minutes and can be used once; create a new session to reconnect. After the call, get the result with `client.Calls.GetByProvider(ctx, session.ID)`.

## Webhooks

voicast sends `call.ended`, `call.transcript_ready`, `call.summary_ready` and other events to the endpoints you register on the webhook portal (`client.Webhooks.CreatePortalLink`). Verify them with the endpoint's signing secret. Pass the raw request body, exactly as received.

```go
secret := os.Getenv("VOICAST_WEBHOOK_SECRET") // whsec_...

http.HandleFunc("/webhooks/voicast", func(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	event, err := voicast.VerifyWebhook(body, r.Header, secret)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	data, _ := event.ParseData()
	switch d := data.(type) {
	case *voicast.CallEndedData:
		fmt.Println(*d.Outcome, d.Fields, d.Tenant.ExternalID)
	case *voicast.CallSummaryReadyData:
		fmt.Println(d.Summary, d.Category)
	case *voicast.BillingBalanceLowData:
		fmt.Println(d.BalanceYen)
	}
	w.WriteHeader(http.StatusNoContent)
})
```

`VerifyWebhook` returns an error matching `ErrWebhookVerification` when the signature does not match, the timestamp is more than 5 minutes off (`voicast.WithTolerance(5*time.Minute)`), or the body is not a voicast event. While you switch to a new secret, pass the others with `voicast.WithSecrets(old)`. `event.UnmarshalData(&v)` decodes the data into your own type.

| Event | `ParseData()` |
|---|---|
| `call.started` | `*CallStartedData` |
| `call.handoff_requested` | `*CallHandoffRequestedData` |
| `call.ended` | `*CallEndedData` (the `Call` and `Tenant`) |
| `call.transcript_ready` | `*CallTranscriptReadyData` |
| `call.summary_ready` | `*CallSummaryReadyData` |
| `connection.needs_reauth` | `*ConnectionNeedsReauthData` |
| `billing.balance_low` / `billing.balance_depleted` / `billing.recharge_failed` / `billing.cap_reached` | `*BillingBalanceLowData` and the others |

## Tools the AI uses during a call

An agent's `Config.Tools` lets the AI call your API during a call (check free slots, look up a customer, make a booking). voicast POSTs a signed request to the tool's `URL` and waits up to 8 seconds; reply with 2xx and JSON, which is given to the AI as is.

```go
http.HandleFunc("/voicast/tools", func(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	call, err := voicast.VerifyToolCall(body, r.Header, os.Getenv("VOICAST_TOOLS_SECRET"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if call.Tool == "find_free_slots" {
		var args struct{ Date string `json:"date"` }
		_ = call.BindArguments(&args)
		tenant := ""
		if call.Call.Tenant != nil { // nil in a test (Tools.Try) without a TenantID
			tenant = call.Call.Tenant.ExternalID
		}
		json.NewEncoder(w).Encode(map[string]any{"slots": findSlots(args.Date, tenant)})
	}
})
```

The tools signing secret (`whsec_...`) is per environment: read it with `client.Tools.GetSecret(ctx)` (or in the dashboard). `client.Tools.RotateSecret(ctx)` issues a new one; for 24 hours requests carry signatures with both, so pass the old one with `voicast.WithSecrets(old)` while you switch. `call.ToolCallID` (also sent as `idempotency-key`) is unique per tool call: use it so a retried booking is not made twice.

To call a third-party API directly, set `Sign: voicast.Bool(false)` (the arguments are sent as they are: the query string for `Method: "GET"`, the JSON body for `POST`) and `Auth: &voicast.AgentToolAuth{Type: "api_key", Header: ..., Value: ...}` (for `Authorization` the key is sent as `Bearer <key>`). The key is stored encrypted outside the config and never returned; `Agent.ToolKeys` shows its last 4 characters, and later updates can leave `Value` empty.

Test a tool before or after saving it with `Tools.Try` (not billed, not recorded as a call, up to 30 per minute). Signed requests have `Test` set, `Call.Channel` is `ChannelTest`, and `Call.Tenant` is nil unless you pass `TenantID`:

```go
r, err := client.Tools.Try(ctx, voicast.ToolTryParams{AgentID: agent.ID, ToolName: "find_free_slots", Arguments: map[string]any{"date": "2026-10-05"}})
if err == nil && !r.OK {
	fmt.Println(r.Error, r.Message, r.Status, r.Body)
}
```

## API

| Method | Endpoint |
|---|---|
| `Agents.Create(ctx, AgentCreateParams)` | `POST /v1/agents` |
| `Agents.List(ctx)` | `GET /v1/agents` |
| `Agents.Get(ctx, id, *AgentGetParams)` | `GET /v1/agents/:id` |
| `Agents.Update(ctx, id, AgentUpdateParams)` | `PUT /v1/agents/:id` (adds a version) |
| `Agents.Versions(ctx, id)` | `GET /v1/agents/:id/versions` |
| `Agents.Delete(ctx, id)` | `DELETE /v1/agents/:id` |
| `Tenants.Create(ctx, TenantCreateParams)` | `POST /v1/tenants` |
| `Tenants.List(ctx)` | `GET /v1/tenants` |
| `Tenants.Get(ctx, id)` | `GET /v1/tenants/:id` |
| `Tenants.Update(ctx, id, TenantUpdateParams)` | `PATCH /v1/tenants/:id` |
| `Tenants.RotateSecret(ctx, id)` | `POST /v1/tenants/:id/secret` |
| `Tenants.Delete(ctx, id)` | `DELETE /v1/tenants/:id` |
| `Tenants.ListConnections(ctx, id)` | `GET /v1/tenants/:id/connections` |
| `Tenants.CreateConnectLink(ctx, id, *ConnectLinkParams)` | `POST /v1/tenants/:id/connect-links` |
| `Tenants.CreateSipCredential(ctx, id, *SipCredentialCreateParams)` | `POST /v1/tenants/:id/sip-credentials` (the password is returned only here) |
| `Tenants.ListSipCredentials(ctx, id)` | `GET /v1/tenants/:id/sip-credentials` |
| `Tenants.UpdateSipCredential(ctx, id, credentialID, *SipCredentialUpdateParams)` | `PATCH /v1/tenants/:id/sip-credentials/:credential` |
| `Tenants.DeleteSipCredential(ctx, id, credentialID)` | `DELETE /v1/tenants/:id/sip-credentials/:credential` |
| `Calls.List(ctx, *CallListParams)` / `Calls.ListAll(*CallListParams)` | `GET /v1/calls` |
| `Calls.Get(ctx, id)` | `GET /v1/calls/:id` |
| `Calls.GetByProvider(ctx, providerCallID)` | `GET /v1/calls/by-provider/:providerCallId` |
| `Calls.Transcript(ctx, id)` | `GET /v1/calls/:id/transcript` |
| `Sessions.Create(ctx, SessionCreateParams)` | `POST /v1/sessions` |
| `Usage.Get(ctx, *UsageParams)` (`Channel`, `TenantID`, `ExternalID` filter) | `GET /v1/usage` |
| `Connectors.List(ctx)` | `GET /v1/connectors` |
| `Connections.List(ctx, *ConnectionListParams)` | `GET /v1/connections` |
| `Connections.Create(ctx, ConnectionCreateParams)` | `POST /v1/connections` |
| `Connections.Get(ctx, id)` | `GET /v1/connections/:id` |
| `Connections.Update(ctx, id, ConnectionUpdateParams)` | `PATCH /v1/connections/:id` |
| `Connections.Options(ctx, id, query)` | `GET /v1/connections/:id/options` |
| `Connections.Delete(ctx, id)` | `DELETE /v1/connections/:id` |
| `Webhooks.CreatePortalLink(ctx, *WebhookPortalParams)` | `POST /v1/webhooks/portal` |
| `Billing.Get(ctx)` | `GET /v1/billing` |
| `Billing.CreateTopup(ctx, TopupParams)` | `POST /v1/billing/topups` |
| `Tools.GetSecret(ctx)` | `GET /v1/tools/secret` |
| `Tools.RotateSecret(ctx)` | `POST /v1/tools/secret/rotate` |
| `Tools.Try(ctx, ToolTryParams)` | `POST /v1/tools/try` |
| `Retention.Get(ctx)` | `GET /v1/retention` |
| `Retention.Update(ctx, RetentionUpdateParams)` | `PATCH /v1/retention` |

Every method also takes request options as trailing arguments: `WithRequestTimeout(d)`, `WithRequestMaxRetries(n)`. Pointer params may be `nil`.

`TenantUpdateParams` sends only the fields you set. `AgentID: voicast.Null[string]()` removes the tenant's agent, `voicast.Value("agt_...")` sets it. `SipCredentialUpdateParams` works the same way: `TransferTo: voicast.Null[string]()` turns the transfer off. `ConnectionListParams{}` lists every connection, `{TenantID: voicast.Null[string]()}` the environment's connections only, and `{TenantID: voicast.Value("ten_...")}` that tenant's.

The call summary and category are on the `Call` (`Summary`, `Category`), filled in when `call.summary_ready` is sent, and `BilledYen` is the fee for the call.

### Retention

Call records (the call, transcript, collected fields, summary and delivery status) and recordings are deleted automatically after the environment's retention period: 1, 7, 14 or 30 days (30 by default), counted from the end of the call for records and from when they were saved for recordings. `Retention.Get` reads it and `Retention.Update(ctx, voicast.RetentionUpdateParams{RetentionDays: 7})` changes it for the API key's environment. Shortening it deletes records past the new period at the next daily cleanup; usage and billing do not change.

### Balance and top-ups

The organization has one balance, shared by its live and test environments. `Billing.Get` returns the balance (`Balance`), the auto-recharge settings and the price per minute. `Billing.CreateTopup` returns a Stripe Checkout `URL` (500 to 300,000 yen, before tax); open it in a browser to pay. Payment is by card top-up only; auto-recharge, the monthly cap and the card are managed in the dashboard.

```go
b, err := client.Billing.Get(ctx)
if err == nil && b.Balance != nil && b.Balance.Yen < 1000 {
	t, _ := client.Billing.CreateTopup(ctx, voicast.TopupParams{AmountYen: 5000})
	fmt.Println(t.URL)
}
```

### Paging through calls

`Calls.List` returns one page (`Data`, `HasMore`). `Calls.ListAll` walks every call, newest first.

```go
it := client.Calls.ListAll(&voicast.CallListParams{TenantID: "ten_..."})
for it.Next(ctx) {
	call := it.Current()
	fmt.Println(call.ID, call.Outcome)
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

## Errors

Every error is a `*voicast.Error` with `Kind`, `Status`, `Code` (such as `invalid_request`, `in_use`, `balance_depleted`), `Message`, `Details` (raw JSON) and `RequestID`. Match it with `errors.Is` and a sentinel, or read it with `errors.As`.

| Sentinel | When |
|---|---|
| `ErrAuthentication` | 401: the API key is missing, wrong or revoked |
| `ErrPaymentRequired` | 402: new calls are refused (`balance_depleted`) |
| `ErrPermission` | 403 |
| `ErrNotFound` | 404 |
| `ErrConflict` | 409: duplicate `external_id`, agent `in_use`, `tenant_disabled`, `needs_reauth` |
| `ErrValidation` | 400, 422. See `Details` |
| `ErrRateLimit` | 429, after retries. `RetryAfter` |
| `ErrAPI` | other responses, including 5xx |
| `ErrConnection` / `ErrTimeout` | no response, a cancelled context, or the timeout reached (a timeout matches both) |
| `ErrWebhookVerification` | `VerifyWebhook` or `VerifyToolCall` failed |
| `ErrInvalidArgument` | a missing API key or an invalid webhook secret |

```go
_, err := client.Sessions.Create(ctx, voicast.SessionCreateParams{TenantID: tenantID})
var e *voicast.Error
if errors.As(err, &e) && e.Kind == voicast.KindPaymentRequired && e.Code == "balance_depleted" {
	showTopUpNotice()
}
```

## Retries and timeouts

`WithTimeout(30*time.Second)` and `WithMaxRetries(2)` are the defaults. 429 is retried for every request, waiting `retry-after` (up to one minute). 5xx, timeouts and connection errors are retried only for requests safe to repeat (GET, PATCH, DELETE, and creating links), with exponential backoff from 0.5 s. Creating agents, tenants, sessions, connections, SIP credentials and top-ups, updating an agent and rotating a secret are not retried on those, so they never run twice. The API does not take an `Idempotency-Key`. Cancelling the context stops the request and any wait.

## Links

- voicast: https://voicast.jp/
- Changelog: [CHANGELOG.md](./CHANGELOG.md)
- License: MIT
