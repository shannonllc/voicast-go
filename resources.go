package voicast

import (
	"context"
	"net/url"
	"strconv"
)

func esc(id string) string { return url.PathEscape(id) }

// values builds a query from key, value pairs, leaving out empty values.
func values(kv ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			q.Set(kv[i], kv[i+1])
		}
	}
	return q
}

func itoa(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

func derefOr[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// get, send and remove are the three call shapes; out may be nil.
func get[T any](ctx context.Context, c *Client, path string, q url.Values, opts []RequestOption) (*T, error) {
	var out T
	if err := c.do(ctx, request{method: "GET", path: path, query: q, idempotent: true}, &out, c.config(opts)); err != nil {
		return nil, err
	}
	return &out, nil
}

func send[T any](ctx context.Context, c *Client, method, path string, body any, idempotent bool, opts []RequestOption) (*T, error) {
	var out T
	if err := c.do(ctx, request{method: method, path: path, body: body, idempotent: idempotent}, &out, c.config(opts)); err != nil {
		return nil, err
	}
	return &out, nil
}

func remove(ctx context.Context, c *Client, path string, opts []RequestOption) error {
	return c.do(ctx, request{method: "DELETE", path: path, idempotent: true}, nil, c.config(opts))
}

// Iter walks every item across pages:
//
//	it := client.Calls.ListAll(&voicast.CallListParams{TenantID: "ten_…"})
//	for it.Next(ctx) {
//		call := it.Current()
//	}
//	if err := it.Err(); err != nil { … }
type Iter[T any] struct {
	fetch    func(ctx context.Context, cursor int64) (*Page[T], error)
	cursorOf func(item T) int64
	cursor   int64
	items    []T
	idx      int
	cur      T
	err      error
	last     bool
}

// Next advances to the next item, fetching the next page when needed. It returns false at the end or on an error.
func (it *Iter[T]) Next(ctx context.Context) bool {
	for {
		if it.err != nil {
			return false
		}
		if it.idx+1 < len(it.items) {
			it.idx++
			it.cur = it.items[it.idx]
			return true
		}
		if it.last {
			return false
		}
		p, err := it.fetch(ctx, it.cursor)
		if err != nil {
			it.err = err
			return false
		}
		it.items, it.idx = p.Data, -1
		// a page that says has_more but is empty stops the walk (instead of reading the same page again)
		it.last = !p.HasMore || len(p.Data) == 0
		if !it.last {
			it.cursor = it.cursorOf(p.Data[len(p.Data)-1])
		}
	}
}

// Current returns the item Next moved to.
func (it *Iter[T]) Current() T { return it.cur }

// Err returns the error that stopped the iteration, if any.
func (it *Iter[T]) Err() error { return it.err }

// AgentsService manages agents: what the AI says and asks (/v1/agents). Every update adds a new version.
type AgentsService struct{ c *Client }

func agentPath(id, rest string) string { return "/v1/agents/" + esc(id) + rest }

// Create creates an agent. Not retried on a 5xx or a connection error (it could create two).
func (s *AgentsService) Create(ctx context.Context, params AgentCreateParams, opts ...RequestOption) (*Agent, error) {
	return send[Agent](ctx, s.c, "POST", "/v1/agents", params, false, opts)
}

// List lists the agents of the environment, without Config.
func (s *AgentsService) List(ctx context.Context, opts ...RequestOption) (*List[Agent], error) {
	return get[List[Agent]](ctx, s.c, "/v1/agents", nil, opts)
}

// Get returns an agent with its Config. params may be nil (the current version).
func (s *AgentsService) Get(ctx context.Context, agentID string, params *AgentGetParams, opts ...RequestOption) (*Agent, error) {
	p := derefOr(params)
	return get[Agent](ctx, s.c, agentPath(agentID, ""), values("version", itoa(int64(p.Version))), opts)
}

// Update saves a new version. Calls in progress keep the version they started with.
// Not retried on a 5xx or a connection error (each request adds a version).
func (s *AgentsService) Update(ctx context.Context, agentID string, params AgentUpdateParams, opts ...RequestOption) (*Agent, error) {
	return send[Agent](ctx, s.c, "PUT", agentPath(agentID, ""), params, false, opts)
}

// Versions lists the versions of an agent, newest first.
func (s *AgentsService) Versions(ctx context.Context, agentID string, opts ...RequestOption) (*List[AgentVersion], error) {
	return get[List[AgentVersion]](ctx, s.c, agentPath(agentID, "/versions"), nil, opts)
}

// Delete deletes an agent. Returns an error matching ErrConflict (code "in_use") while a tenant uses it.
func (s *AgentsService) Delete(ctx context.Context, agentID string, opts ...RequestOption) error {
	return remove(ctx, s.c, agentPath(agentID, ""), opts)
}

// TenantsService manages tenants: the businesses that actually take the calls (/v1/tenants).
type TenantsService struct{ c *Client }

func tenantPath(id, rest string) string { return "/v1/tenants/" + esc(id) + rest }

// Create creates a tenant. The connection secret (vts_…) is returned only here.
// Not retried on a 5xx or a connection error (a retry would get 409 and lose the secret).
func (s *TenantsService) Create(ctx context.Context, params TenantCreateParams, opts ...RequestOption) (*TenantWithSecret, error) {
	return send[TenantWithSecret](ctx, s.c, "POST", "/v1/tenants", params, false, opts)
}

// List lists the tenants of the environment.
func (s *TenantsService) List(ctx context.Context, opts ...RequestOption) (*List[Tenant], error) {
	return get[List[Tenant]](ctx, s.c, "/v1/tenants", nil, opts)
}

// Get returns a tenant.
func (s *TenantsService) Get(ctx context.Context, tenantID string, opts ...RequestOption) (*Tenant, error) {
	return get[Tenant](ctx, s.c, tenantPath(tenantID, ""), nil, opts)
}

// Update changes only the fields that are set. AgentID: Null[string]() removes the tenant's agent.
func (s *TenantsService) Update(ctx context.Context, tenantID string, params TenantUpdateParams, opts ...RequestOption) (*Tenant, error) {
	return send[Tenant](ctx, s.c, "PATCH", tenantPath(tenantID, ""), params, true, opts)
}

// RotateSecret issues a new connection secret. The previous one keeps working for 24 hours. Not retried on a 5xx.
func (s *TenantsService) RotateSecret(ctx context.Context, tenantID string, opts ...RequestOption) (*TenantSecret, error) {
	return send[TenantSecret](ctx, s.c, "POST", tenantPath(tenantID, "/secret"), nil, false, opts)
}

// Delete deletes a tenant and revokes its connections. Its calls are kept.
func (s *TenantsService) Delete(ctx context.Context, tenantID string, opts ...RequestOption) error {
	return remove(ctx, s.c, tenantPath(tenantID, ""), opts)
}

// ListConnections lists the tenant's connections.
func (s *TenantsService) ListConnections(ctx context.Context, tenantID string, opts ...RequestOption) (*List[Connection], error) {
	return get[List[Connection]](ctx, s.c, tenantPath(tenantID, "/connections"), nil, opts)
}

// CreateConnectLink creates a link (valid for 15 minutes) to the page where the business connects its calendar, CRM or
// chat. Open it in a new tab, or in an iframe on FrameOrigin. params may be nil.
func (s *TenantsService) CreateConnectLink(ctx context.Context, tenantID string, params *ConnectLinkParams, opts ...RequestOption) (*Link, error) {
	return send[Link](ctx, s.c, "POST", tenantPath(tenantID, "/connect-links"), derefOr(params), true, opts)
}

// CreateSipCredential creates Digest credentials for a PBX that sends calls over SIP to sip.voicast.jp. params may be
// nil. The password is returned only here. Not retried on a 5xx or a connection error (a retry would make a second
// credential).
func (s *TenantsService) CreateSipCredential(ctx context.Context, tenantID string, params *SipCredentialCreateParams, opts ...RequestOption) (*SipCredentialWithPassword, error) {
	return send[SipCredentialWithPassword](ctx, s.c, "POST", tenantPath(tenantID, "/sip-credentials"), derefOr(params), false, opts)
}

// ListSipCredentials lists the tenant's SIP credentials (without passwords).
func (s *TenantsService) ListSipCredentials(ctx context.Context, tenantID string, opts ...RequestOption) (*List[SipCredential], error) {
	return get[List[SipCredential]](ctx, s.c, tenantPath(tenantID, "/sip-credentials"), nil, opts)
}

// DeleteSipCredential deletes SIP credentials. Calls on them are refused at once.
func (s *TenantsService) DeleteSipCredential(ctx context.Context, tenantID, credentialID string, opts ...RequestOption) error {
	return remove(ctx, s.c, tenantPath(tenantID, "/sip-credentials/"+esc(credentialID)), opts)
}

// CallsService reads calls (/v1/calls), newest first.
type CallsService struct{ c *Client }

func (s *CallsService) page(ctx context.Context, p CallListParams, before int64, rc requestConfig) (*Page[Call], error) {
	q := values("tenant_id", p.TenantID, "before", itoa(before), "limit", itoa(int64(p.Limit)))
	var out Page[Call]
	if err := s.c.do(ctx, request{method: "GET", path: "/v1/calls", query: q, idempotent: true}, &out, rc); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns one page of calls, newest first. params may be nil.
func (s *CallsService) List(ctx context.Context, params *CallListParams, opts ...RequestOption) (*Page[Call], error) {
	p := derefOr(params)
	return s.page(ctx, p, p.Before, s.c.config(opts))
}

// ListAll walks every call, newest first, starting before params.Before. The next page starts before the last call's
// StartedAt. params may be nil.
func (s *CallsService) ListAll(params *CallListParams, opts ...RequestOption) *Iter[Call] {
	p := derefOr(params)
	rc := s.c.config(opts)
	return &Iter[Call]{
		fetch:    func(ctx context.Context, before int64) (*Page[Call], error) { return s.page(ctx, p, before, rc) },
		cursorOf: func(c Call) int64 { return c.StartedAt },
		cursor:   p.Before,
		idx:      -1,
	}
}

// Get returns a call. Summary and Category are filled in when call.summary_ready is sent.
func (s *CallsService) Get(ctx context.Context, callID string, opts ...RequestOption) (*Call, error) {
	return get[Call](ctx, s.c, "/v1/calls/"+esc(callID), nil, opts)
}

// GetByProvider returns a call by the provider's call ID: Twilio's CallSid, or the session ID for Web calls.
func (s *CallsService) GetByProvider(ctx context.Context, providerCallID string, opts ...RequestOption) (*Call, error) {
	return get[Call](ctx, s.c, "/v1/calls/by-provider/"+esc(providerCallID), nil, opts)
}

// Transcript returns the transcript. Returns an error matching ErrNotFound when the call has none (HasTranscript false).
func (s *CallsService) Transcript(ctx context.Context, callID string, opts ...RequestOption) (*Transcript, error) {
	return get[Transcript](ctx, s.c, "/v1/calls/"+esc(callID)+"/transcript", nil, opts)
}

// SessionsService creates sessions for talking from a browser or an app over WebRTC (/v1/sessions).
type SessionsService struct{ c *Client }

// Create creates a session on your server. Pass only ClientToken and WebRTCURL to the browser or app, never the API
// key. The token expires in 2 minutes and can be used once. Returns an error matching ErrPaymentRequired when calls
// are refused for billing. Not retried on a 5xx or a connection error.
func (s *SessionsService) Create(ctx context.Context, params SessionCreateParams, opts ...RequestOption) (*Session, error) {
	return send[Session](ctx, s.c, "POST", "/v1/sessions", params, false, opts)
}

// UsageService reads the minutes used, for billing your own customers (/v1/usage).
type UsageService struct{ c *Client }

// Get counts calls that ended in the period. Default: this month (Japan time) until now. params may be nil.
func (s *UsageService) Get(ctx context.Context, params *UsageParams, opts ...RequestOption) (*Usage, error) {
	p := derefOr(params)
	return get[Usage](ctx, s.c, "/v1/usage", values("from", p.From, "to", p.To, "group_by", p.GroupBy,
		"channel", string(p.Channel), "tenant_id", p.TenantID, "external_id", p.ExternalID), opts)
}

// ConnectorsService lists the connectors that can be connected (/v1/connectors).
type ConnectorsService struct{ c *Client }

// List lists the connectors.
func (s *ConnectorsService) List(ctx context.Context, opts ...RequestOption) (*List[Connector], error) {
	return get[List[Connector]](ctx, s.c, "/v1/connectors", nil, opts)
}

// ConnectionsService manages connections to calendars, CRMs and chat (/v1/connections).
type ConnectionsService struct{ c *Client }

func connectionPath(id, rest string) string { return "/v1/connections/" + esc(id) + rest }

// List lists connections. params may be nil (every connection).
func (s *ConnectionsService) List(ctx context.Context, params *ConnectionListParams, opts ...RequestOption) (*List[Connection], error) {
	p := derefOr(params)
	q := url.Values{}
	if p.TenantID.IsSet() {
		q.Set("tenant_id", derefOr(p.TenantID.value))
	}
	return get[List[Connection]](ctx, s.c, "/v1/connections", q, opts)
}

// Create connects a connector set up with an API key or a webhook URL (teams, http).
// OAuth connectors are connected by the business on the page from Tenants.CreateConnectLink. Not retried on a 5xx.
func (s *ConnectionsService) Create(ctx context.Context, params ConnectionCreateParams, opts ...RequestOption) (*Connection, error) {
	return send[Connection](ctx, s.c, "POST", "/v1/connections", params, false, opts)
}

// Get returns a connection.
func (s *ConnectionsService) Get(ctx context.Context, connectionID string, opts ...RequestOption) (*Connection, error) {
	return get[Connection](ctx, s.c, connectionPath(connectionID, ""), nil, opts)
}

// Update changes only the Config fields given. A connection waiting for setup becomes active.
func (s *ConnectionsService) Update(ctx context.Context, connectionID string, params ConnectionUpdateParams, opts ...RequestOption) (*Connection, error) {
	return send[Connection](ctx, s.c, "PATCH", connectionPath(connectionID, ""), params, true, opts)
}

// Options reads the choices for the connection's settings from the connected service (calendars, sheets, apps, rooms;
// empty for http). query is passed as is, such as {"spreadsheet": …, "sheet": …} or {"app": …}; empty values are left out.
func (s *ConnectionsService) Options(ctx context.Context, connectionID string, query map[string]string, opts ...RequestOption) (map[string]any, error) {
	q := url.Values{}
	for k, v := range query {
		if v != "" {
			q.Set(k, v)
		}
	}
	out, err := get[map[string]any](ctx, s.c, connectionPath(connectionID, "/options"), q, opts)
	if err != nil {
		return nil, err
	}
	return *out, nil
}

// Delete disconnects: revokes the tokens at the connected service and deletes the stored secret.
func (s *ConnectionsService) Delete(ctx context.Context, connectionID string, opts ...RequestOption) error {
	return remove(ctx, s.c, connectionPath(connectionID, ""), opts)
}

// WebhooksService creates links to the webhook portal, where you set up where webhooks (call.ended and others) are sent.
type WebhooksService struct{ c *Client }

// CreatePortalLink creates a link (valid for 15 minutes) to the webhook portal: endpoints, event types and signing
// secrets. params may be nil.
func (s *WebhooksService) CreatePortalLink(ctx context.Context, params *WebhookPortalParams, opts ...RequestOption) (*Link, error) {
	return send[Link](ctx, s.c, "POST", "/v1/webhooks/portal", derefOr(params), true, opts)
}

// BillingService reads the organization's billing (/v1/billing): balance, auto-recharge and the price per minute.
type BillingService struct{ c *Client }

// Get returns the organization's billing. The organization has one balance, shared by its live and test environments.
func (s *BillingService) Get(ctx context.Context, opts ...RequestOption) (*Billing, error) {
	return get[Billing](ctx, s.c, "/v1/billing", nil, opts)
}

// CreateTopup creates a Stripe Checkout URL to top up the balance (500 to 300,000 yen, before tax). Open it in a
// browser to pay.
// Not retried on a 5xx or a connection error.
func (s *BillingService) CreateTopup(ctx context.Context, params TopupParams, opts ...RequestOption) (*Topup, error) {
	return send[Topup](ctx, s.c, "POST", "/v1/billing/topups", params, false, opts)
}

// RetentionService reads and changes how long call records and recordings are kept in the API key's environment
// (/v1/retention): 1, 7, 14 or 30 days (30 by default). Call records count from the end of the call, recordings
// from when they were saved.
type RetentionService struct{ c *Client }

// Get returns the retention period.
func (s *RetentionService) Get(ctx context.Context, opts ...RequestOption) (*Retention, error) {
	return get[Retention](ctx, s.c, "/v1/retention", nil, opts)
}

// Update changes the retention period (1, 7, 14 or 30 days). When shortened, call records and recordings past the
// new period are deleted at the next daily cleanup and cannot be restored. Usage and billing do not change.
func (s *RetentionService) Update(ctx context.Context, params RetentionUpdateParams, opts ...RequestOption) (*Retention, error) {
	return send[Retention](ctx, s.c, "PATCH", "/v1/retention", params, true, opts)
}

// ToolsService manages the signing secret for in-call tool requests (/v1/tools/secret, one per environment) and tests
// agent tools.
type ToolsService struct{ c *Client }

// GetSecret returns the signing secret (whsec_…) to pass to VerifyToolCall.
func (s *ToolsService) GetSecret(ctx context.Context, opts ...RequestOption) (*ToolsSecret, error) {
	return get[ToolsSecret](ctx, s.c, "/v1/tools/secret", nil, opts)
}

// RotateSecret issues a new signing secret. For 24 hours requests carry signatures with both secrets, so switch your
// server to the new one within that time. Not retried on a 5xx or a connection error.
func (s *ToolsService) RotateSecret(ctx context.Context, opts ...RequestOption) (*ToolsSecret, error) {
	return send[ToolsSecret](ctx, s.c, "POST", "/v1/tools/secret/rotate", nil, false, opts)
}

// Try calls an agent tool once, as during a call, and returns what it answered. Not billed and not recorded as a
// call. Signed requests carry "test": true, call.channel "test" and a try_… call ID. Up to 30 per minute per
// environment. Not retried on a 5xx or a connection error (it calls your API).
func (s *ToolsService) Try(ctx context.Context, params ToolTryParams, opts ...RequestOption) (*ToolTryResult, error) {
	return send[ToolTryResult](ctx, s.c, "POST", "/v1/tools/try", params, false, opts)
}
