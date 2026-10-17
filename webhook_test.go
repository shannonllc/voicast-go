package voicast

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The vectors are signed like voicast and by the Node SDK, with results decided by the Node SDK
// (scripts/sdk-vectors.mjs). The same file is used by the Python and PHP SDKs.
type vector struct {
	Name      string            `json:"name"`
	Verify    string            `json:"verify"`
	Secret    json.RawMessage   `json:"secret"`
	Payload   string            `json:"payload"`
	Headers   map[string]string `json:"headers"`
	Now       int64             `json:"now"`
	Tolerance *int64            `json:"tolerance"`
	Expect    json.RawMessage   `json:"expect"`
	Error     string            `json:"error"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	b, err := os.ReadFile("testdata/webhook_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Vectors []vector }
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f.Vectors
}

func (v vector) secrets() []string {
	var list []string
	if json.Unmarshal(v.Secret, &list) == nil {
		return list
	}
	var one string
	_ = json.Unmarshal(v.Secret, &one)
	return []string{one}
}

func (v vector) header() http.Header {
	h := http.Header{}
	for k, val := range v.Headers {
		h.Set(k, val)
	}
	return h
}

func (v vector) webhook(t *testing.T) *Webhook {
	t.Helper()
	s := v.secrets()
	opts := []WebhookOption{WithSecrets(s[1:]...)}
	if v.Tolerance != nil {
		opts = append(opts, WithTolerance(time.Duration(*v.Tolerance)*time.Second))
	}
	w, err := NewWebhook(s[0], opts...)
	if err != nil {
		t.Fatal(err)
	}
	w.now = func() time.Time { return time.Unix(v.Now, 0) }
	return w
}

func sameJSON(t *testing.T, got []byte, want json.RawMessage) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(want, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		t.Run(v.Name, func(t *testing.T) {
			w := v.webhook(t)
			var got []byte
			var err error
			switch v.Verify {
			case "webhook":
				var ev *Event
				if ev, err = w.VerifyEvent([]byte(v.Payload), v.header()); err == nil {
					got = ev.Raw
				}
			case "tool_call":
				var tc *ToolCallRequest
				if tc, err = w.VerifyToolCall([]byte(v.Payload), v.header()); err == nil {
					got = tc.Raw
				}
			default:
				got, err = w.Verify([]byte(v.Payload), v.header())
			}
			if string(v.Expect) == "null" {
				var e *Error
				if !errors.As(err, &e) || e.Kind != KindWebhookVerification || e.Message != v.Error || !errors.Is(err, ErrWebhookVerification) {
					t.Fatalf("got %v, want %q", err, v.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sameJSON(t, got, v.Expect)
		})
	}
}

func find(t *testing.T, name string) vector {
	t.Helper()
	for _, v := range loadVectors(t) {
		if v.Name == name {
			return v
		}
	}
	t.Fatal("no vector", name)
	return vector{}
}

func TestSignMatchesToolsPyAndNode(t *testing.T) {
	for _, name := range []string{"signed like voicast", "signed by the node sdk", "standard webhooks spec vector"} {
		v := find(t, name)
		w, _ := NewWebhook(v.secrets()[0], WithSecrets("whsec_dGhpcyBpcyBhbm90aGVyIHNlY3JldCBrZXk="))
		ts, _ := strconv.ParseInt(v.Headers["webhook-timestamp"], 10, 64)
		if got := w.Sign(v.Headers["webhook-id"], time.Unix(ts, 0), []byte(v.Payload)); got != v.Headers["webhook-signature"] {
			t.Fatalf("%s: %s", name, got)
		}
	}
}

func TestEventAndToolCall(t *testing.T) {
	v := find(t, "webhook event")
	ev, err := v.webhook(t).VerifyEvent([]byte(v.Payload), v.header())
	if err != nil || ev.Type != EventCallEnded || ev.Timestamp != "2026-10-03T00:00:00.000Z" {
		t.Fatal(ev, err)
	}
	d, err := ev.ParseData()
	ended, ok := d.(*CallEndedData)
	if err != nil || !ok || ended.Tenant.ExternalID != "clinic-1" || *ended.Outcome != OutcomeCompleted || ended.Fields["name"] != "山田 太郎" {
		t.Fatal(d, err)
	}
	var m map[string]any
	if err := ev.UnmarshalData(&m); err != nil || m["id"] != "call_1" {
		t.Fatal(m, err)
	}

	v = find(t, "tool call")
	tc, err := v.webhook(t).VerifyToolCall([]byte(v.Payload), v.header())
	if err != nil || tc.Type != "tool.call" || tc.Tool != "find_free_slots" || tc.ToolCallID != "tc_1" || tc.Call.Channel != ChannelPhone || *tc.Call.From != "+81312345678" || tc.Call.Tenant.ID != "ten_1" {
		t.Fatal(tc, err)
	}
	var args struct{ Date string }
	if err := tc.BindArguments(&args); err != nil || args.Date != "2026-10-04" {
		t.Fatal(args, err)
	}
}

func TestParseData(t *testing.T) {
	cases := map[EventType]any{
		EventCallStarted:            &CallStartedData{},
		EventCallHandoffRequested:   &CallHandoffRequestedData{},
		EventCallEnded:              &CallEndedData{},
		EventCallTranscriptReady:    &CallTranscriptReadyData{},
		EventCallSummaryReady:       &CallSummaryReadyData{},
		EventConnectionNeedsReauth:  &ConnectionNeedsReauthData{},
		EventBillingBalanceLow:      &BillingBalanceLowData{},
		EventBillingBalanceDepleted: &BillingBalanceDepletedData{},
		EventBillingRechargeFailed:  &BillingRechargeFailedData{},
		EventBillingCapReached:      &BillingCapReachedData{},
	}
	for typ, want := range cases {
		got, err := (&Event{Type: typ, Data: json.RawMessage(`{}`)}).ParseData()
		if err != nil || reflect.TypeOf(got) != reflect.TypeOf(want) {
			t.Fatal(typ, got, err)
		}
		if _, err := (&Event{Type: typ, Data: json.RawMessage(`[]`)}).ParseData(); err == nil {
			t.Fatal(typ, "decoded an array")
		}
	}
	got, err := (&Event{Type: "call.future", Data: json.RawMessage(`{"x":1}`)}).ParseData()
	if err != nil || !reflect.DeepEqual(got, map[string]any{"x": float64(1)}) {
		t.Fatal(got, err)
	}
	if _, err := (&Event{Type: "call.future", Data: json.RawMessage(`[]`)}).ParseData(); err == nil {
		t.Fatal("decoded an array")
	}
	d, _ := (&Event{Type: EventCallStarted, Data: json.RawMessage(`{"call_id":"c","from":"+81","tenant":{"id":"t"}}`)}).ParseData()
	if s := d.(*CallStartedData); *s.From != "+81" || s.Tenant.ID != "t" {
		t.Fatal(s)
	}
	d, _ = (&Event{Type: EventBillingCapReached, Data: json.RawMessage(`{"kind":"auto_recharge","recharged_yen":5000}`)}).ParseData()
	if s := d.(*BillingCapReachedData); s.Kind != "auto_recharge" || s.RechargedYen != 5000 {
		t.Fatal(s)
	}
}

func TestVerifyFunctions(t *testing.T) {
	const secret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	const other = "whsec_dGhpcyBpcyBhbm90aGVyIHNlY3JldCBrZXk="
	w, _ := NewWebhook(secret)
	now := time.Now()
	sign := func(body string) http.Header {
		h := http.Header{}
		h.Set("webhook-id", "msg_now")
		h.Set("webhook-timestamp", strconv.FormatInt(now.Unix(), 10))
		h.Set("webhook-signature", w.Sign("msg_now", now, []byte(body)))
		return h
	}
	ev := `{"type":"billing.balance_low","timestamp":"t","data":{"balance_yen":120}}`
	e, err := VerifyWebhook([]byte(ev), sign(ev), other, WithSecrets(secret), WithTolerance(10*time.Second))
	if err != nil || e.Type != EventBillingBalanceLow {
		t.Fatal(e, err)
	}
	tool := `{"type":"tool.call","tool":"x","tool_call_id":"tc","arguments":{},"call":{"id":"c","tenant":{"id":"t","external_id":"e"}}}`
	tc, err := VerifyToolCall([]byte(tool), sign(tool), secret)
	if err != nil || tc.Call.Tenant.ExternalID != "e" || tc.Test {
		t.Fatal(tc, err)
	}
	test := `{"type":"tool.call","tool":"x","tool_call_id":"try_1","arguments":{},"call":{"id":"try_1","channel":"test","provider_call_id":null,"from":null,"tenant":null},"test":true}`
	tc, err = VerifyToolCall([]byte(test), sign(test), secret)
	if err != nil || !tc.Test || tc.Call.Channel != ChannelTest || tc.Call.Tenant != nil {
		t.Fatal(tc, err)
	}
	// fields of an unexpected type stay empty, like the Node SDK (which does not check them)
	odd := `{"type":"tool.call","tool":"x","arguments":5,"call":{"id":7}}`
	tc, err = VerifyToolCall([]byte(odd), sign(odd), secret)
	if err != nil || tc.Call.ID != "" || string(tc.Raw) != odd {
		t.Fatal(tc, err)
	}
	if _, err := VerifyWebhook([]byte(ev), sign(ev), "whsec_***"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := VerifyToolCall([]byte(tool), sign(tool), ""); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := VerifyWebhook([]byte(ev), http.Header{}, secret); !errors.Is(err, ErrWebhookVerification) {
		t.Fatal(err)
	}
	if _, err := VerifyToolCall([]byte(tool), http.Header{}, secret); !errors.Is(err, ErrWebhookVerification) {
		t.Fatal(err)
	}
}

func TestHeaderForms(t *testing.T) {
	v := find(t, "signed like voicast")
	w := v.webhook(t)
	// a map literal with lowercase keys (not canonicalised by Set)
	lower := http.Header{}
	for k, val := range v.Headers {
		lower[k] = []string{val, "x"}
	}
	got, err := w.Verify([]byte(v.Payload), lower)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, got, v.Expect)
	empty := http.Header{"webhook-id": {}}
	if _, err := w.Verify([]byte(v.Payload), empty); err == nil || !strings.Contains(err.Error(), "Missing required headers") {
		t.Fatal(err)
	}
}

func TestSecretChecks(t *testing.T) {
	if _, err := NewWebhook(""); !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "required") {
		t.Fatal(err)
	}
	if _, err := NewWebhook("whsec_***"); !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "not valid base64") {
		t.Fatal(err)
	}
	if _, err := NewWebhook("whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw", WithSecrets("")); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	// unpadded base64 is accepted, like atob in the Node SDK
	w, err := NewWebhook("whsec_dGhpcyBpcyBhbm90aGVyIHNlY3JldCBrZXk")
	if err != nil || w.tolerance != DefaultTolerance || len(w.keys) != 1 {
		t.Fatal(err)
	}
}
