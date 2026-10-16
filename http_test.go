package voicast

import (
	"context"
	"errors"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var emptyList = map[string]any{"data": []any{}}

func TestClientOptions(t *testing.T) {
	t.Run("headers", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(200, emptyList))
		c, _ := testClient(t, f)
		if _, err := c.Agents.List(context.Background()); err != nil {
			t.Fatal(err)
		}
		h := f.Last().Header
		for k, want := range map[string]string{"Authorization": "Bearer vk_test_abc", "Accept": "application/json", "User-Agent": "voicast-go/" + Version} {
			if got := h.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		if h.Get("Content-Type") != "" || len(f.Last().Raw) != 0 {
			t.Errorf("GET sent a body")
		}
	})

	t.Run("body and content type", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(201, map[string]any{"id": "ten_1", "secret": "vts_1"}))
		c, _ := testClient(t, f)
		r, err := c.Tenants.Create(context.Background(), TenantCreateParams{ExternalID: "clinic-1", Name: "○○歯科"})
		if err != nil || r.ID != "ten_1" || r.Secret != "vts_1" {
			t.Fatal(r, err)
		}
		if f.Last().Header.Get("Content-Type") != "application/json" || string(f.Last().Raw) != `{"external_id":"clinic-1","name":"○○歯科"}` {
			t.Fatal(string(f.Last().Raw))
		}
	})

	t.Run("user agent format", func(t *testing.T) {
		if !regexp.MustCompile(`^voicast-go/\d+\.\d+\.\d+$`).MatchString(UserAgent) {
			t.Fatal(UserAgent)
		}
	})

	t.Run("api key from env", func(t *testing.T) {
		t.Setenv(APIKeyEnv, " vk_live_env ")
		f := newFakeAPI(t, jsonReply(200, emptyList))
		c, err := NewClient("", WithBaseURL(f.srv.URL))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = c.Agents.List(context.Background())
		if got := f.Last().Header.Get("Authorization"); got != "Bearer vk_live_env" {
			t.Fatal(got)
		}
	})

	t.Run("missing api key", func(t *testing.T) {
		t.Setenv(APIKeyEnv, "  ")
		_, err := NewClient("")
		if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), APIKeyEnv) {
			t.Fatal(err)
		}
	})

	t.Run("base url and options", func(t *testing.T) {
		c, _ := NewClient("vk_test_x")
		if c.BaseURL() != "https://api.voicast.jp" {
			t.Fatal(c.BaseURL())
		}
		c, _ = NewClient("vk_test_x", WithBaseURL("http://localhost:8787//"))
		if c.BaseURL() != "http://localhost:8787" {
			t.Fatal(c.BaseURL())
		}
		if c.timeout != DefaultTimeout || c.maxRetries != DefaultMaxRetries || c.httpClient == nil {
			t.Fatal("defaults")
		}
		h := &http.Client{}
		c, _ = NewClient("vk_test_x", WithHTTPClient(h), WithTimeout(time.Second), WithMaxRetries(5))
		if c.httpClient != h || c.timeout != time.Second || c.maxRetries != 5 {
			t.Fatal("options")
		}
	})

	t.Run("version matches the changelog", func(t *testing.T) {
		b, err := os.ReadFile("CHANGELOG.md")
		if err != nil || !strings.Contains(string(b), "## "+Version) {
			t.Fatal("CHANGELOG.md has no entry for", Version)
		}
	})
}

func TestErrors(t *testing.T) {
	cases := []struct {
		status   int
		code     string
		kind     Kind
		sentinel error
	}{
		{400, "bad_json", KindValidation, ErrValidation},
		{401, "unauthorized", KindAuthentication, ErrAuthentication},
		{402, "balance_depleted", KindPaymentRequired, ErrPaymentRequired},
		{403, "forbidden", KindPermission, ErrPermission},
		{404, "not_found", KindNotFound, ErrNotFound},
		{409, "in_use", KindConflict, ErrConflict},
		{413, "too_large", KindAPI, ErrAPI},
		{418, "teapot", KindAPI, ErrAPI},
		{422, "invalid_request", KindValidation, ErrValidation},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			f := newFakeAPI(t, jsonReply(tc.status, map[string]any{"error": tc.code, "message": "m"}, "cf-ray", "ray-1"))
			c, _ := testClient(t, f)
			_, err := c.Sessions.Create(context.Background(), SessionCreateParams{TenantID: "ten_1"})
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("%T", err)
			}
			if e.Kind != tc.kind || e.Status != tc.status || e.Code != tc.code || e.Message != "m" || e.RequestID != "ray-1" || e.Details != nil {
				t.Fatalf("%+v", e)
			}
			if !errors.Is(err, tc.sentinel) || errors.Is(err, ErrConnection) || err.Error() != "m" {
				t.Fatal("sentinel")
			}
			if len(f.Calls()) != 1 {
				t.Fatal("retried")
			}
		})
	}

	t.Run("details and request id", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(422, map[string]any{"error": "invalid_request", "message": "bad", "details": []any{map[string]any{"path": "name"}}}, "x-request-id", "req_1", "cf-ray", "r"))
		c, _ := testClient(t, f)
		_, err := c.Agents.Create(context.Background(), AgentCreateParams{Name: "x"})
		var e *Error
		errors.As(err, &e)
		if string(e.Details) != `[{"path":"name"}]` || e.RequestID != "req_1" {
			t.Fatalf("%+v", e)
		}
	})

	t.Run("non-json error body and fields of the wrong type", func(t *testing.T) {
		f := newFakeAPI(t, rawReply(502, "<html>bad gateway</html>"), rawReply(400, "null"), rawReply(500, `"oops"`), rawReply(404, `{"error":1,"message":null,"details":null}`))
		c, _ := testClient(t, f, WithMaxRetries(0))
		for _, want := range []struct {
			status int
			kind   Kind
		}{{502, KindAPI}, {400, KindValidation}, {500, KindAPI}, {404, KindNotFound}} {
			_, err := c.Calls.Get(context.Background(), "c")
			var e *Error
			errors.As(err, &e)
			if e.Status != want.status || e.Kind != want.kind || e.Code != "" || e.Message != "HTTP "+strconv.Itoa(want.status) || e.RequestID != "" || e.Details != nil {
				t.Fatalf("%+v", e)
			}
		}
	})

	t.Run("invalid json in 2xx is not retried", func(t *testing.T) {
		f := newFakeAPI(t, rawReply(200, "{oops", "cf-ray", "r2"))
		c, _ := testClient(t, f)
		_, err := c.Calls.Get(context.Background(), "c")
		var e *Error
		errors.As(err, &e)
		if e.Kind != KindAPI || e.Status != 200 || e.RequestID != "r2" || !strings.Contains(e.Message, "Invalid JSON") || len(f.Calls()) != 1 {
			t.Fatalf("%+v", e)
		}
	})

	t.Run("json of the wrong shape in 2xx", func(t *testing.T) {
		f := newFakeAPI(t, rawReply(200, `{"id": 1}`))
		c, _ := testClient(t, f)
		_, err := c.Calls.Get(context.Background(), "c")
		var e *Error
		errors.As(err, &e)
		if e.Kind != KindAPI || !strings.Contains(e.Message, "Unexpected response") || e.Unwrap() == nil {
			t.Fatalf("%+v", e)
		}
	})

	t.Run("empty 2xx body", func(t *testing.T) {
		f := newFakeAPI(t, rawReply(204, ""))
		c, _ := testClient(t, f)
		if err := c.Tenants.Delete(context.Background(), "ten_1"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("connection error keeps the cause", func(t *testing.T) {
		f := newFakeAPI(t, dropReply)
		c, _ := testClient(t, f, WithMaxRetries(0))
		_, err := c.Calls.Get(context.Background(), "c")
		var e *Error
		errors.As(err, &e)
		if e.Kind != KindConnection || e.Status != 0 || e.Err == nil || !strings.HasPrefix(e.Message, "Connection error: ") {
			t.Fatalf("%+v", e)
		}
		if !errors.Is(err, ErrConnection) || errors.Is(err, ErrTimeout) {
			t.Fatal("sentinel")
		}
	})

	t.Run("unencodable body", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(200, map[string]any{}))
		c, _ := testClient(t, f)
		_, err := c.Connections.Update(context.Background(), "con_1", ConnectionUpdateParams{Config: map[string]any{"x": make(chan int)}})
		if !errors.Is(err, ErrInvalidArgument) || len(f.Calls()) != 0 {
			t.Fatal(err)
		}
	})

	t.Run("invalid base url", func(t *testing.T) {
		c, _ := NewClient("vk_test_x", WithBaseURL("http://bad host"))
		_, err := c.Calls.Get(context.Background(), "c")
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatal(err)
		}
	})
}

func TestRetries(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		replies   []reply
		opts      []Option
		op        func(c *Client) error
		calls     int
		sleeps    []time.Duration
		wantError error
	}{
		{
			name:    "5xx with backoff, then success",
			replies: []reply{jsonReply(500, map[string]any{"error": "internal", "message": "x"}), jsonReply(503, map[string]any{}), jsonReply(200, map[string]any{"id": "call_1"})},
			op:      func(c *Client) error { _, err := c.Calls.Get(ctx, "call_1"); return err },
			calls:   3, sleeps: []time.Duration{500 * time.Millisecond, time.Second},
		},
		{
			name:    "gives up after max retries",
			replies: []reply{jsonReply(500, map[string]any{"error": "internal", "message": "down"})},
			opts:    []Option{WithMaxRetries(3)},
			op:      func(c *Client) error { _, err := c.Calls.Get(ctx, "c"); return err },
			calls:   4, sleeps: []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}, wantError: ErrAPI,
		},
		{
			name:    "max retries per request",
			replies: []reply{jsonReply(500, map[string]any{})},
			opts:    []Option{WithMaxRetries(5)},
			op:      func(c *Client) error { _, err := c.Calls.Get(ctx, "c", WithRequestMaxRetries(0)); return err },
			calls:   1, wantError: ErrAPI,
		},
		{
			name:    "connection errors retried for safe requests",
			replies: []reply{dropReply, jsonReply(200, emptyList)},
			op:      func(c *Client) error { _, err := c.Tenants.List(ctx); return err },
			calls:   2, sleeps: []time.Duration{500 * time.Millisecond},
		},
		{
			name:    "PATCH is retried",
			replies: []reply{jsonReply(502, map[string]any{}), jsonReply(200, map[string]any{"id": "ten_1"})},
			op: func(c *Client) error {
				_, err := c.Tenants.Update(ctx, "ten_1", TenantUpdateParams{Name: String("x")})
				return err
			},
			calls: 2, sleeps: []time.Duration{500 * time.Millisecond},
		},
		{
			name:    "creating links is retried",
			replies: []reply{jsonReply(500, map[string]any{}), jsonReply(201, map[string]any{"url": "u"})},
			op:      func(c *Client) error { _, err := c.Webhooks.CreatePortalLink(ctx, nil); return err },
			calls:   2, sleeps: []time.Duration{500 * time.Millisecond},
		},
		{
			name:    "4xx not retried",
			replies: []reply{jsonReply(409, map[string]any{"error": "in_use", "message": "x"})},
			op:      func(c *Client) error { return c.Agents.Delete(ctx, "a") },
			calls:   1, wantError: ErrConflict,
		},
		{
			name:    "429 waits retry-after, for any request",
			replies: []reply{jsonReply(429, map[string]any{"error": "rate_limited", "message": "slow"}, "retry-after", "7"), jsonReply(201, map[string]any{"id": "ses_1"})},
			op: func(c *Client) error {
				_, err := c.Sessions.Create(ctx, SessionCreateParams{TenantID: "t"})
				return err
			},
			calls: 2, sleeps: []time.Duration{7 * time.Second},
		},
		{
			name:    "429 without headers uses backoff",
			replies: []reply{jsonReply(429, map[string]any{"error": "rate_limited", "message": "x"}), jsonReply(200, map[string]any{"id": "c"})},
			op:      func(c *Client) error { _, err := c.Calls.Get(ctx, "c"); return err },
			calls:   2, sleeps: []time.Duration{500 * time.Millisecond},
		},
		{
			name:    "429 at exactly one minute is retried",
			replies: []reply{jsonReply(429, map[string]any{}, "retry-after", "60"), jsonReply(200, map[string]any{"id": "c"})},
			op:      func(c *Client) error { _, err := c.Calls.Get(ctx, "c"); return err },
			calls:   2, sleeps: []time.Duration{time.Minute},
		},
		{
			name:    "429 longer than a minute returned at once",
			replies: []reply{jsonReply(429, map[string]any{"error": "rate_limited", "message": "x"}, "retry-after", "61")},
			op:      func(c *Client) error { _, err := c.Calls.Get(ctx, "c"); return err },
			calls:   1, wantError: ErrRateLimit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t, tc.replies...)
			c, s := testClient(t, f, tc.opts...)
			err := tc.op(c)
			if tc.wantError == nil && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != nil && !errors.Is(err, tc.wantError) {
				t.Fatalf("got %v, want %v", err, tc.wantError)
			}
			if n := len(f.Calls()); n != tc.calls {
				t.Fatalf("calls = %d, want %d", n, tc.calls)
			}
			if got := s.All(); !reflect.DeepEqual(got, tc.sleeps) && !(len(got) == 0 && len(tc.sleeps) == 0) {
				t.Fatalf("sleeps = %v, want %v", got, tc.sleeps)
			}
		})
	}

	t.Run("429 waits until x-ratelimit-reset", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(429, map[string]any{"error": "rate_limited", "message": "x"}, "x-ratelimit-reset", "1012"), jsonReply(200, map[string]any{"id": "c"}))
		c, s := testClient(t, f)
		c.now = func() time.Time { return time.Unix(1000, 0) }
		if _, err := c.Calls.Get(ctx, "c"); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.All(), []time.Duration{12 * time.Second}) {
			t.Fatal(s.All())
		}
	})

	t.Run("429 returned with RetryAfter", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(429, map[string]any{"error": "rate_limited", "message": "x"}, "retry-after", "1"))
		c, _ := testClient(t, f, WithMaxRetries(1))
		_, err := c.Calls.Get(ctx, "c")
		var e *Error
		errors.As(err, &e)
		if e.Kind != KindRateLimit || e.Code != "rate_limited" || e.RetryAfter == nil || *e.RetryAfter != time.Second || len(f.Calls()) != 2 {
			t.Fatalf("%+v", e)
		}
	})

	t.Run("unsafe requests are not retried on 5xx, connection errors or timeouts", func(t *testing.T) {
		for _, rep := range []reply{jsonReply(500, map[string]any{}), dropReply, hangReply} {
			f := newFakeAPI(t, rep)
			c, _ := testClient(t, f, WithTimeout(50*time.Millisecond))
			ops := []func() error{
				func() error { _, err := c.Agents.Create(ctx, AgentCreateParams{Name: "x"}); return err },
				func() error { _, err := c.Agents.Update(ctx, "a", AgentUpdateParams{}); return err },
				func() error { _, err := c.Tenants.Create(ctx, TenantCreateParams{ExternalID: "x"}); return err },
				func() error { _, err := c.Tenants.RotateSecret(ctx, "t"); return err },
				func() error { _, err := c.Tenants.CreateSipCredential(ctx, "t", nil); return err },
				func() error { _, err := c.Sessions.Create(ctx, SessionCreateParams{TenantID: "t"}); return err },
				func() error {
					_, err := c.Connections.Create(ctx, ConnectionCreateParams{Connector: "teams", Secret: "s"})
					return err
				},
				func() error { _, err := c.Billing.CreateTopup(ctx, TopupParams{AmountYen: 500}); return err },
				func() error { _, err := c.Tools.RotateSecret(ctx); return err },
				func() error { _, err := c.Tools.Try(ctx, ToolTryParams{AgentID: "a", ToolName: "x"}); return err },
			}
			for _, op := range ops {
				var e *Error
				if !errors.As(op(), &e) {
					t.Fatal("no error")
				}
			}
			if len(f.Calls()) != len(ops) {
				t.Fatalf("calls = %d", len(f.Calls()))
			}
		}
	})
}

func TestTimeoutAndCancel(t *testing.T) {
	t.Run("each attempt times out; safe requests are retried", func(t *testing.T) {
		f := newFakeAPI(t, hangReply)
		c, _ := testClient(t, f, WithTimeout(50*time.Millisecond), WithMaxRetries(1))
		_, err := c.Calls.Get(context.Background(), "c")
		var e *Error
		errors.As(err, &e)
		if e.Kind != KindTimeout || !errors.Is(err, ErrTimeout) || !errors.Is(err, ErrConnection) || e.Message != "Request timed out after 50ms" {
			t.Fatalf("%+v", e)
		}
		if len(f.Calls()) != 2 {
			t.Fatal(len(f.Calls()))
		}
	})

	t.Run("timeout per request", func(t *testing.T) {
		f := newFakeAPI(t, hangReply)
		c, _ := testClient(t, f, WithTimeout(time.Minute))
		_, err := c.Tenants.RotateSecret(context.Background(), "t", WithRequestTimeout(20*time.Millisecond))
		if !errors.Is(err, ErrTimeout) || len(f.Calls()) != 1 {
			t.Fatal(err)
		}
	})

	t.Run("http.Client timeout counts as a timeout", func(t *testing.T) {
		f := newFakeAPI(t, hangReply)
		c, _ := testClient(t, f, WithHTTPClient(&http.Client{Timeout: 20 * time.Millisecond}), WithMaxRetries(0))
		_, err := c.Calls.Get(context.Background(), "c")
		if !errors.Is(err, ErrTimeout) {
			t.Fatal(err)
		}
	})

	t.Run("a cancelled context stops the request and is not retried", func(t *testing.T) {
		f := newFakeAPI(t, hangReply)
		c, _ := testClient(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(30*time.Millisecond, cancel)
		_, err := c.Calls.Get(ctx, "c")
		var e *Error
		errors.As(err, &e)
		if e.Kind != KindConnection || e.Message != "Request was aborted" || len(f.Calls()) != 1 {
			t.Fatalf("%+v", e)
		}
	})

	t.Run("a context cancelled before the call sends nothing", func(t *testing.T) {
		f := newFakeAPI(t, hangReply)
		c, _ := testClient(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.Calls.Get(ctx, "c")
		if !errors.Is(err, ErrConnection) || !errors.Is(err, context.Canceled) || len(f.Calls()) != 0 {
			t.Fatal(err)
		}
	})

	t.Run("cancelling during the wait between retries", func(t *testing.T) {
		f := newFakeAPI(t, jsonReply(503, map[string]any{}))
		c, _ := testClient(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		c.sleep = func(context.Context, time.Duration) error { cancel(); return context.Canceled }
		_, err := c.Calls.Get(ctx, "c")
		if !errors.Is(err, ErrConnection) || !errors.Is(err, context.Canceled) || len(f.Calls()) != 1 {
			t.Fatal(err)
		}
	})

	t.Run("real sleep honours the context", func(t *testing.T) {
		if err := sleepContext(context.Background(), time.Millisecond); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := sleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
}

func TestHelpers(t *testing.T) {
	half := func() float64 { return 0.5 }
	var got []time.Duration
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10} {
		got = append(got, backoff(n, half))
	}
	ms := time.Millisecond
	if !reflect.DeepEqual(got, []time.Duration{500 * ms, 1000 * ms, 2000 * ms, 4000 * ms, 8000 * ms, 8000 * ms, 8000 * ms}) {
		t.Fatal(got)
	}
	if backoff(0, func() float64 { return 0 }) != 375*ms || backoff(0, func() float64 { return 1 }) != 625*ms {
		t.Fatal("jitter")
	}

	h := func(kv ...string) http.Header {
		out := http.Header{}
		for i := 0; i < len(kv); i += 2 {
			out.Set(kv[i], kv[i+1])
		}
		return out
	}
	at := func(sec float64) time.Time { return time.Unix(0, int64(sec*1e9)) }
	cases := []struct {
		h    http.Header
		now  float64
		want time.Duration
		ok   bool
	}{
		{h("retry-after", "2"), 0, 2 * time.Second, true},
		{h("retry-after", "0.4"), 0, 400 * ms, true},
		{h("retry-after", "Thu, 01 Jan 1970 00:00:05 GMT"), 1, 4 * time.Second, true},
		{h("retry-after", "Thu, 01 Jan 1970 00:00:05 GMT"), 9, 0, true},
		{h("retry-after", "soon", "x-ratelimit-reset", "10"), 4, 6 * time.Second, true},
		{h("retry-after", "-3"), 0, 0, true},
		{h("retry-after", "NaN"), 0, 0, false},
		{h("x-ratelimit-reset", "1"), 5, 0, true},
		{h("x-ratelimit-reset", "x"), 0, 0, false},
		{h("x-ratelimit-reset", "Inf"), 0, 0, false},
		{h("retry-after", " "), 0, 0, false},
		{h(), 0, 0, false},
	}
	for i, tc := range cases {
		d, ok := rateLimitWait(tc.h, at(tc.now))
		if d != tc.want || ok != tc.ok {
			t.Errorf("case %d: got %v %v, want %v %v", i, d, ok, tc.want, tc.ok)
		}
	}
	if MaxRateLimitWait != time.Minute {
		t.Fatal(MaxRateLimitWait)
	}
	if *String("a") != "a" || *Int(3) != 3 {
		t.Fatal("pointer helpers")
	}
	e := &Error{Kind: KindRateLimit, Message: "slow"}
	if e.Error() != "slow" || e.Unwrap() != nil || e.RetryAfter != nil {
		t.Fatal(e)
	}
}
