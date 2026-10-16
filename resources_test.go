package voicast

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"testing"
)

// check runs op against a fake API and compares the request it sent.
func check(t *testing.T, name string, reply reply, op func(c *Client) error, method, path, query, body string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		f := newFakeAPI(t, reply)
		c, _ := testClient(t, f)
		if err := op(c); err != nil {
			t.Fatal(err)
		}
		got := f.Last()
		if got.Method != method || got.Path != path || got.Query != query || string(got.Raw) != body {
			t.Fatalf("got %s %s ?%s %s, want %s %s ?%s %s", got.Method, got.Path, got.Query, got.Raw, method, path, query, body)
		}
	})
}

var cfg = AgentConfig{Greeting: "g", Persona: "p", Fields: []AgentField{{Key: "name", Label: "お名前", Required: true}}, HandoffWhen: []string{}, Closing: "c"}

const cfgJSON = `{"greeting":"g","persona":"p","fields":[{"key":"name","label":"お名前","required":true}],"handoff_when":[],"closing":"c"}`

func TestAgents(t *testing.T) {
	ctx := context.Background()
	ok := jsonReply(200, map[string]any{"id": "agt_1", "config": map[string]any{"greeting": "g"}, "tool_keys": []any{map[string]any{"name": "lookup", "hint": "9012", "updated_at": 1}}})
	check(t, "create", ok, func(c *Client) error {
		a, err := c.Agents.Create(ctx, AgentCreateParams{Name: "Reception", Config: cfg})
		if err == nil && (a.ID != "agt_1" || a.Config.Greeting != "g" || a.ToolKeys[0].Hint != "9012") {
			t.Fatal(a)
		}
		return err
	}, "POST", "/v1/agents", "", `{"name":"Reception","config":`+cfgJSON+`}`)
	check(t, "list", jsonReply(200, map[string]any{"data": []any{map[string]any{"id": "agt_1", "version": 2}}}), func(c *Client) error {
		l, err := c.Agents.List(ctx)
		if err == nil && (len(l.Data) != 1 || l.Data[0].Version != 2 || l.Data[0].Config != nil) {
			t.Fatal(l)
		}
		return err
	}, "GET", "/v1/agents", "", "")
	check(t, "get", ok, func(c *Client) error { _, err := c.Agents.Get(ctx, "agt_1", nil); return err }, "GET", "/v1/agents/agt_1", "", "")
	check(t, "get a version", ok, func(c *Client) error {
		_, err := c.Agents.Get(ctx, "agt_1", &AgentGetParams{Version: 3})
		return err
	}, "GET", "/v1/agents/agt_1", "version=3", "")
	check(t, "update", ok, func(c *Client) error {
		_, err := c.Agents.Update(ctx, "agt_1", AgentUpdateParams{Config: cfg})
		return err
	}, "PUT", "/v1/agents/agt_1", "", `{"config":`+cfgJSON+`}`)
	check(t, "update with name", ok, func(c *Client) error {
		_, err := c.Agents.Update(ctx, "agt_1", AgentUpdateParams{Name: "New", Config: cfg})
		return err
	}, "PUT", "/v1/agents/agt_1", "", `{"name":"New","config":`+cfgJSON+`}`)
	check(t, "versions", jsonReply(200, map[string]any{"data": []any{map[string]any{"version": 2, "created_at": 5}}}), func(c *Client) error {
		l, err := c.Agents.Versions(ctx, "agt_1")
		if err == nil && (l.Data[0].Version != 2 || l.Data[0].CreatedAt != 5) {
			t.Fatal(l)
		}
		return err
	}, "GET", "/v1/agents/agt_1/versions", "", "")
	check(t, "delete", rawReply(204, ""), func(c *Client) error { return c.Agents.Delete(ctx, "agt_1") }, "DELETE", "/v1/agents/agt_1", "", "")
	check(t, "ids are escaped", ok, func(c *Client) error { _, err := c.Agents.Get(ctx, "a/b?c d", nil); return err }, "GET", "/v1/agents/a%2Fb%3Fc%20d", "", "")
}

func TestTenants(t *testing.T) {
	ctx := context.Background()
	ok := jsonReply(200, map[string]any{"id": "ten_1", "agent_id": nil, "status": "active"})
	check(t, "create", ok, func(c *Client) error {
		_, err := c.Tenants.Create(ctx, TenantCreateParams{ExternalID: "clinic-1"})
		return err
	}, "POST", "/v1/tenants", "", `{"external_id":"clinic-1"}`)
	check(t, "create with agent", ok, func(c *Client) error {
		_, err := c.Tenants.Create(ctx, TenantCreateParams{ExternalID: "clinic-1", Name: "n", AgentID: "agt_1"})
		return err
	}, "POST", "/v1/tenants", "", `{"external_id":"clinic-1","name":"n","agent_id":"agt_1"}`)
	check(t, "list", jsonReply(200, emptyList), func(c *Client) error { _, err := c.Tenants.List(ctx); return err }, "GET", "/v1/tenants", "", "")
	check(t, "get", ok, func(c *Client) error {
		tn, err := c.Tenants.Get(ctx, "ten_1")
		if err == nil && (tn.AgentID != nil || tn.Status != TenantActive) {
			t.Fatal(tn)
		}
		return err
	}, "GET", "/v1/tenants/ten_1", "", "")
	disabled := TenantDisabled
	check(t, "update status", ok, func(c *Client) error {
		_, err := c.Tenants.Update(ctx, "ten_1", TenantUpdateParams{Status: &disabled})
		return err
	}, "PATCH", "/v1/tenants/ten_1", "", `{"status":"disabled"}`)
	check(t, "update removes the agent", ok, func(c *Client) error {
		_, err := c.Tenants.Update(ctx, "ten_1", TenantUpdateParams{Name: String("x"), AgentID: Null[string]()})
		return err
	}, "PATCH", "/v1/tenants/ten_1", "", `{"agent_id":null,"name":"x"}`)
	check(t, "update sets the agent", ok, func(c *Client) error {
		_, err := c.Tenants.Update(ctx, "ten_1", TenantUpdateParams{AgentID: Value("agt_2")})
		return err
	}, "PATCH", "/v1/tenants/ten_1", "", `{"agent_id":"agt_2"}`)
	check(t, "update nothing", ok, func(c *Client) error {
		_, err := c.Tenants.Update(ctx, "ten_1", TenantUpdateParams{})
		return err
	}, "PATCH", "/v1/tenants/ten_1", "", `{}`)
	check(t, "rotate secret", jsonReply(200, map[string]any{"secret": "vts_2", "previous_secret_expires_at": 9}), func(c *Client) error {
		s, err := c.Tenants.RotateSecret(ctx, "ten_1")
		if err == nil && (s.Secret != "vts_2" || s.PreviousSecretExpiresAt != 9) {
			t.Fatal(s)
		}
		return err
	}, "POST", "/v1/tenants/ten_1/secret", "", "")
	check(t, "delete", rawReply(204, ""), func(c *Client) error { return c.Tenants.Delete(ctx, "ten_1") }, "DELETE", "/v1/tenants/ten_1", "", "")
	check(t, "list connections", jsonReply(200, emptyList), func(c *Client) error {
		_, err := c.Tenants.ListConnections(ctx, "ten_1")
		return err
	}, "GET", "/v1/tenants/ten_1/connections", "", "")
	link := jsonReply(201, map[string]any{"url": "https://app.voicast.jp/connect/vcl_1", "expires_at": 1})
	check(t, "connect link", link, func(c *Client) error {
		l, err := c.Tenants.CreateConnectLink(ctx, "ten_1", nil)
		if err == nil && (l.URL != "https://app.voicast.jp/connect/vcl_1" || l.ExpiresAt != 1) {
			t.Fatal(l)
		}
		return err
	}, "POST", "/v1/tenants/ten_1/connect-links", "", `{}`)
	check(t, "connect link with options", link, func(c *Client) error {
		_, err := c.Tenants.CreateConnectLink(ctx, "ten_1", &ConnectLinkParams{Connectors: []string{"google_calendar"}, FrameOrigin: "https://saas.example"})
		return err
	}, "POST", "/v1/tenants/ten_1/connect-links", "", `{"connectors":["google_calendar"],"frame_origin":"https://saas.example"}`)
	sip := jsonReply(201, map[string]any{
		"id": "sipc_1", "username": "sipc_1", "password": "pw", "realm": "voicast", "server": "sip.voicast.jp",
		"tenant_id": "ten_1", "agent_id": nil, "name": "", "created_at": 1,
	})
	check(t, "create sip credential", sip, func(c *Client) error {
		cr, err := c.Tenants.CreateSipCredential(ctx, "ten_1", nil)
		if err == nil && (cr.Username != "sipc_1" || cr.Password != "pw" || cr.Realm != "voicast" || cr.Server != "sip.voicast.jp" || cr.AgentID != nil) {
			t.Fatal(cr)
		}
		return err
	}, "POST", "/v1/tenants/ten_1/sip-credentials", "", `{}`)
	check(t, "create sip credential with options", sip, func(c *Client) error {
		_, err := c.Tenants.CreateSipCredential(ctx, "ten_1", &SipCredentialCreateParams{Name: "本院の PBX", AgentID: "agt_1"})
		return err
	}, "POST", "/v1/tenants/ten_1/sip-credentials", "", `{"name":"本院の PBX","agent_id":"agt_1"}`)
	check(t, "list sip credentials", jsonReply(200, emptyList), func(c *Client) error {
		_, err := c.Tenants.ListSipCredentials(ctx, "ten_1")
		return err
	}, "GET", "/v1/tenants/ten_1/sip-credentials", "", "")
	check(t, "delete sip credential (escaped)", rawReply(204, ""), func(c *Client) error {
		return c.Tenants.DeleteSipCredential(ctx, "ten_1", "sipc/1")
	}, "DELETE", "/v1/tenants/ten_1/sip-credentials/sipc%2F1", "", "")

	n := Nullable[string]{}
	if n.IsSet() || n.IsNull() || !Null[string]().IsNull() || Value("x").IsNull() || !Value("x").IsSet() {
		t.Fatal("nullable")
	}
}

func TestCalls(t *testing.T) {
	ctx := context.Background()
	call := map[string]any{"id": "call_1", "from": "+81312345678", "outcome": "completed", "fields": map[string]any{"name": "山田"}, "summary": nil, "channel": "phone", "output": "audio"}
	check(t, "get", jsonReply(200, call), func(c *Client) error {
		cl, err := c.Calls.Get(ctx, "call_1")
		if err == nil && (*cl.From != "+81312345678" || *cl.Outcome != OutcomeCompleted || cl.Fields["name"] != "山田" || cl.Summary != nil || cl.Channel != ChannelPhone || cl.Output != OutputAudio) {
			t.Fatal(cl)
		}
		return err
	}, "GET", "/v1/calls/call_1", "", "")
	check(t, "by provider", jsonReply(200, call), func(c *Client) error {
		_, err := c.Calls.GetByProvider(ctx, "CA123")
		return err
	}, "GET", "/v1/calls/by-provider/CA123", "", "")
	check(t, "transcript", jsonReply(200, map[string]any{"call_id": "call_1", "turns": []any{map[string]any{"role": "user", "text": "こんにちは"}}}), func(c *Client) error {
		tr, err := c.Calls.Transcript(ctx, "call_1")
		if err == nil && (tr.CallID != "call_1" || tr.Turns[0].Text != "こんにちは") {
			t.Fatal(tr)
		}
		return err
	}, "GET", "/v1/calls/call_1/transcript", "", "")
}

func page(ids []string, hasMore bool, base int64) reply {
	data := []any{}
	for i, id := range ids {
		data = append(data, map[string]any{"id": id, "started_at": base - int64(i)})
	}
	return jsonReply(200, map[string]any{"data": data, "has_more": hasMore})
}

func TestPagination(t *testing.T) {
	ctx := context.Background()
	t.Run("List gives one page; ListAll walks every page", func(t *testing.T) {
		f := newFakeAPI(t, page([]string{"a", "b"}, true, 1000), page([]string{"a", "b"}, true, 1000), page([]string{"c", "d"}, true, 900), page([]string{"e"}, false, 800))
		c, _ := testClient(t, f)
		p, err := c.Calls.List(ctx, &CallListParams{TenantID: "ten_1", Limit: 2})
		if err != nil || len(p.Data) != 2 || !p.HasMore {
			t.Fatal(p, err)
		}
		var ids []string
		it := c.Calls.ListAll(&CallListParams{TenantID: "ten_1", Limit: 2})
		for it.Next(ctx) {
			ids = append(ids, it.Current().ID)
		}
		if it.Err() != nil || !reflect.DeepEqual(ids, []string{"a", "b", "c", "d", "e"}) {
			t.Fatal(ids, it.Err())
		}
		var qs []string
		for _, cl := range f.Calls() {
			qs = append(qs, cl.Query)
		}
		want := []string{"limit=2&tenant_id=ten_1", "limit=2&tenant_id=ten_1", "before=999&limit=2&tenant_id=ten_1", "before=899&limit=2&tenant_id=ten_1"}
		if !reflect.DeepEqual(qs, want) {
			t.Fatal(qs)
		}
		if it.Next(ctx) {
			t.Fatal("Next after the end")
		}
	})

	t.Run("starts before the given time; an empty page that says has_more stops", func(t *testing.T) {
		f := newFakeAPI(t, page([]string{"x"}, true, 500), jsonReply(200, map[string]any{"data": []any{}, "has_more": true}))
		c, _ := testClient(t, f)
		it := c.Calls.ListAll(&CallListParams{Before: 12345})
		var got []string
		for it.Next(ctx) {
			got = append(got, it.Current().ID)
		}
		if it.Err() != nil || !reflect.DeepEqual(got, []string{"x"}) || len(f.Calls()) != 2 || f.Calls()[0].Query != "before=12345" || f.Calls()[1].Query != "before=500" {
			t.Fatal(got, f.Calls())
		}
	})

	t.Run("an error stops the iterator", func(t *testing.T) {
		f := newFakeAPI(t, page([]string{"x"}, true, 500), jsonReply(404, map[string]any{"error": "not_found", "message": "gone"}))
		c, _ := testClient(t, f, WithMaxRetries(0))
		it := c.Calls.ListAll(nil)
		var got []string
		for it.Next(ctx) {
			got = append(got, it.Current().ID)
		}
		if !errors.Is(it.Err(), ErrNotFound) || !reflect.DeepEqual(got, []string{"x"}) || f.Calls()[0].Query != "" {
			t.Fatal(got, it.Err())
		}
		if it.Next(ctx) {
			t.Fatal("Next after an error")
		}
	})

	t.Run("nil params and list errors", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(401, map[string]any{"error": "unauthorized", "message": "no"}))
		c, _ := testClient(t, f)
		if _, err := c.Calls.List(ctx, nil); !errors.Is(err, ErrAuthentication) {
			t.Fatal(err)
		}
		if f.Last().Query != "" {
			t.Fatal(f.Last().Query)
		}
	})
}

func TestOthers(t *testing.T) {
	ctx := context.Background()
	check(t, "session", jsonReply(201, map[string]any{"id": "ses_1", "client_token": "vct_1", "webrtc_url": "https://voice.voicast.jp/v1/webrtc/offer", "output": "text"}), func(c *Client) error {
		s, err := c.Sessions.Create(ctx, SessionCreateParams{TenantID: "ten_1"})
		if err == nil && (s.ClientToken != "vct_1" || s.WebRTCURL == "" || s.Output != OutputText) {
			t.Fatal(s)
		}
		return err
	}, "POST", "/v1/sessions", "", `{"tenant_id":"ten_1"}`)
	check(t, "session with options", jsonReply(201, map[string]any{}), func(c *Client) error {
		_, err := c.Sessions.Create(ctx, SessionCreateParams{TenantID: "ten_1", AgentID: "agt_1", Output: OutputText})
		return err
	}, "POST", "/v1/sessions", "", `{"tenant_id":"ten_1","agent_id":"agt_1","output":"text"}`)
	usage := map[string]any{"environment": "live", "from": 1, "to": 2, "group_by": "tenant", "total": map[string]any{"calls": 3, "minutes": 4, "amount_yen": 24}, "data": []any{map[string]any{"tenant": map[string]any{"id": "ten_1"}, "calls": 3}}}
	check(t, "usage", jsonReply(200, usage), func(c *Client) error {
		u, err := c.Usage.Get(ctx, nil)
		if err == nil && (u.Total.Calls != 3 || u.Total.AmountYen != 24 || u.Data[0].Tenant.ID != "ten_1" || *u.GroupBy != "tenant") {
			t.Fatal(u)
		}
		return err
	}, "GET", "/v1/usage", "", "")
	check(t, "usage with params", jsonReply(200, usage), func(c *Client) error {
		_, err := c.Usage.Get(ctx, &UsageParams{From: "2026-10-01", To: "1790000000000", GroupBy: "day"})
		return err
	}, "GET", "/v1/usage", "from=2026-10-01&group_by=day&to=1790000000000", "")
	check(t, "usage with filters", jsonReply(200, usage), func(c *Client) error {
		_, err := c.Usage.Get(ctx, &UsageParams{GroupBy: "tenant", Channel: "phone", TenantID: "ten_1", ExternalID: "clinic-1"})
		return err
	}, "GET", "/v1/usage", "channel=phone&external_id=clinic-1&group_by=tenant&tenant_id=ten_1", "")
	check(t, "connectors", jsonReply(200, map[string]any{"data": []any{map[string]any{"id": "google_calendar", "actions": []any{map[string]any{"tool": "gcal_find_free_slots"}}}}}), func(c *Client) error {
		l, err := c.Connectors.List(ctx)
		if err == nil && l.Data[0].Actions[0].Tool != "gcal_find_free_slots" {
			t.Fatal(l)
		}
		return err
	}, "GET", "/v1/connectors", "", "")

	conn := jsonReply(200, map[string]any{"id": "con_1", "status": "needs_setup", "config": map[string]any{"base_url": "https://api.example"}})
	check(t, "connections: every one", jsonReply(200, emptyList), func(c *Client) error { _, err := c.Connections.List(ctx, nil); return err }, "GET", "/v1/connections", "", "")
	check(t, "connections: the environment's", jsonReply(200, emptyList), func(c *Client) error {
		_, err := c.Connections.List(ctx, &ConnectionListParams{TenantID: Null[string]()})
		return err
	}, "GET", "/v1/connections", "tenant_id=", "")
	check(t, "connections: a tenant's", jsonReply(200, emptyList), func(c *Client) error {
		_, err := c.Connections.List(ctx, &ConnectionListParams{TenantID: Value("ten_1")})
		return err
	}, "GET", "/v1/connections", "tenant_id=ten_1", "")
	check(t, "connection create", conn, func(c *Client) error {
		cn, err := c.Connections.Create(ctx, ConnectionCreateParams{Connector: "http", Secret: "k", Input: map[string]string{"base_url": "https://api.example"}, TenantID: "t"})
		if err == nil && (cn.Status != ConnectionNeedsSetup || cn.Config["base_url"] != "https://api.example") {
			t.Fatal(cn)
		}
		return err
	}, "POST", "/v1/connections", "", `{"connector":"http","secret":"k","input":{"base_url":"https://api.example"},"tenant_id":"t"}`)
	check(t, "connection get", conn, func(c *Client) error { _, err := c.Connections.Get(ctx, "con_1"); return err }, "GET", "/v1/connections/con_1", "", "")
	check(t, "connection update", conn, func(c *Client) error {
		_, err := c.Connections.Update(ctx, "con_1", ConnectionUpdateParams{Config: map[string]any{"calendar_id": "primary"}})
		return err
	}, "PATCH", "/v1/connections/con_1", "", `{"config":{"calendar_id":"primary"}}`)
	check(t, "connection options", jsonReply(200, map[string]any{"calendars": []any{}}), func(c *Client) error {
		o, err := c.Connections.Options(ctx, "con_1", map[string]string{"spreadsheet": "s1", "sheet": ""})
		if err == nil && o["calendars"] == nil {
			t.Fatal(o)
		}
		return err
	}, "GET", "/v1/connections/con_1/options", "spreadsheet=s1", "")
	check(t, "connection delete", rawReply(204, ""), func(c *Client) error { return c.Connections.Delete(ctx, "con_1") }, "DELETE", "/v1/connections/con_1", "", "")
	check(t, "portal link", jsonReply(201, map[string]any{"url": "u"}), func(c *Client) error {
		_, err := c.Webhooks.CreatePortalLink(ctx, &WebhookPortalParams{FrameOrigin: "https://saas.example"})
		return err
	}, "POST", "/v1/webhooks/portal", "", `{"frame_origin":"https://saas.example"}`)

	check(t, "billing", jsonReply(200, map[string]any{"balance": map[string]any{"yen": 1200, "depleted": false}, "contract": map[string]any{"per_minute_yen": 30}, "auto_recharge": map[string]any{"enabled": true}}), func(c *Client) error {
		b, err := c.Billing.Get(ctx)
		if err == nil && (b.Balance.Yen != 1200 || b.Contract.PerMinuteYen != 30 || !b.AutoRecharge.Enabled) {
			t.Fatal(b)
		}
		return err
	}, "GET", "/v1/billing", "", "")
	check(t, "top-up", jsonReply(201, map[string]any{"url": "https://checkout.stripe.com/x", "amount_yen": 5000, "tax_yen": 500}), func(c *Client) error {
		tp, err := c.Billing.CreateTopup(ctx, TopupParams{AmountYen: 5000})
		if err == nil && (tp.URL == "" || tp.TaxYen != 500) {
			t.Fatal(tp)
		}
		return err
	}, "POST", "/v1/billing/topups", "", `{"amount_yen":5000}`)
	check(t, "retention", jsonReply(200, map[string]any{"retention_days": 7, "choices": []int{1, 7, 14, 30}}), func(c *Client) error {
		r, err := c.Retention.Get(ctx)
		if err == nil && (r.RetentionDays != 7 || len(r.Choices) != 4) {
			t.Fatal(r)
		}
		return err
	}, "GET", "/v1/retention", "", "")
	check(t, "retention update", jsonReply(200, map[string]any{"retention_days": 14, "choices": []int{1, 7, 14, 30}}), func(c *Client) error {
		r, err := c.Retention.Update(ctx, RetentionUpdateParams{RetentionDays: 14})
		if err == nil && r.RetentionDays != 14 {
			t.Fatal(r)
		}
		return err
	}, "PATCH", "/v1/retention", "", `{"retention_days":14}`)
	check(t, "tools secret", jsonReply(200, map[string]any{"secret": "whsec_x", "previous_secret_expires_at": nil}), func(c *Client) error {
		ts, err := c.Tools.GetSecret(ctx)
		if err == nil && (ts.Secret != "whsec_x" || ts.PreviousSecretExpiresAt != nil) {
			t.Fatal(ts)
		}
		return err
	}, "GET", "/v1/tools/secret", "", "")
	check(t, "tools secret rotate", jsonReply(200, map[string]any{"secret": "whsec_y", "previous_secret_expires_at": 9}), func(c *Client) error {
		_, err := c.Tools.RotateSecret(ctx)
		return err
	}, "POST", "/v1/tools/secret/rotate", "", "")
	tried := jsonReply(200, map[string]any{"tool": "find_slots", "ok": false, "error": "unauthorized", "status": 401, "body": "no", "truncated": false, "duration_ms": 12})
	check(t, "tools try (saved tool)", tried, func(c *Client) error {
		r, err := c.Tools.Try(ctx, ToolTryParams{AgentID: "agt_1", ToolName: "find_slots", Arguments: map[string]any{"date": "2026-10-05"}, TenantID: "ten_1"})
		if err == nil && (r.OK || r.Error != "unauthorized" || *r.Status != 401 || *r.Body != "no" || r.DurationMS != 12) {
			t.Fatal(r)
		}
		return err
	}, "POST", "/v1/tools/try", "", `{"agent_id":"agt_1","tool":"find_slots","arguments":{"date":"2026-10-05"},"tenant_id":"ten_1"}`)
	check(t, "tools try (definition)", tried, func(c *Client) error {
		_, err := c.Tools.Try(ctx, ToolTryParams{Tool: &AgentTool{
			Name: "lookup", Description: "d", URL: "https://api.example.com/x", Method: "GET", Sign: Bool(false),
			Auth: &AgentToolAuth{Type: "api_key", Header: "Authorization", Value: "sk_123456789012"},
		}})
		return err
	}, "POST", "/v1/tools/try", "", `{"tool":{"name":"lookup","description":"d","url":"https://api.example.com/x","method":"GET","sign":false,"auth":{"type":"api_key","header":"Authorization","value":"sk_123456789012"}}}`)

	t.Run("options error", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(409, map[string]any{"error": "needs_reauth", "message": "x"}))
		c, _ := testClient(t, f)
		if _, err := c.Connections.Options(ctx, "con_1", nil); !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	})

	t.Run("values leaves out empty values", func(t *testing.T) {
		if got := values("a", "1", "b", "", "c"); !reflect.DeepEqual(got, url.Values{"a": {"1"}}) {
			t.Fatal(got)
		}
	})
}
