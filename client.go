// Package voicast is the Go SDK for voicast (https://voicast.jp/), the headless conversational AI platform:
// manage agents, tenants and calls, and verify webhooks and in-call tool requests.
//
// Use it on your server only: never ship the API key to a browser or an app.
package voicast

import (
	"context"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the public API.
	DefaultBaseURL = "https://api.voicast.jp"
	// DefaultTimeout is the time allowed for each attempt.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRetries is the number of retries after the first attempt.
	DefaultMaxRetries = 2
	// APIKeyEnv is read when the API key argument is empty.
	APIKeyEnv = "VOICAST_API_KEY"
)

// Client for the voicast API (/v1). It is safe for concurrent use.
type Client struct {
	Agents      *AgentsService
	Tenants     *TenantsService
	Calls       *CallsService
	Sessions    *SessionsService
	Usage       *UsageService
	Connectors  *ConnectorsService
	Connections *ConnectionsService
	Webhooks    *WebhooksService
	Billing     *BillingService
	Tools       *ToolsService

	apiKey     string
	baseURL    string
	timeout    time.Duration
	maxRetries int
	httpClient *http.Client

	// replaced in tests
	sleep  func(ctx context.Context, d time.Duration) error
	random func() float64
	now    func() time.Time
}

// Option configures a [Client].
type Option func(*Client)

// WithBaseURL sets the API origin. Default https://api.voicast.jp.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithTimeout sets the time allowed for each attempt. Default 30 seconds.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithMaxRetries sets the retries after the first attempt. Default 2. 0 disables retries.
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// WithHTTPClient sets the HTTP client. Default: a new http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// NewClient creates a client. apiKey is vk_live_… or vk_test_…; when empty, VOICAST_API_KEY is used.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	key := apiKey
	if key == "" {
		key = strings.TrimSpace(os.Getenv(APIKeyEnv))
	}
	if key == "" {
		return nil, newError(KindInvalidArgument, "Missing API key. Pass it to voicast.NewClient(\"vk_…\") or set "+APIKeyEnv+".", nil)
	}
	c := &Client{
		apiKey:     key,
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		sleep:      sleepContext,
		random:     rand.Float64,
		now:        time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	c.Agents = &AgentsService{c}
	c.Tenants = &TenantsService{c}
	c.Calls = &CallsService{c}
	c.Sessions = &SessionsService{c}
	c.Usage = &UsageService{c}
	c.Connectors = &ConnectorsService{c}
	c.Connections = &ConnectionsService{c}
	c.Webhooks = &WebhooksService{c}
	c.Billing = &BillingService{c}
	c.Tools = &ToolsService{c}
	return c, nil
}

// BaseURL returns the API origin without a trailing slash.
func (c *Client) BaseURL() string { return c.baseURL }

// RequestOption overrides client settings for one call.
type RequestOption func(*requestConfig)

type requestConfig struct {
	timeout    time.Duration
	maxRetries int
}

// WithRequestTimeout sets the time allowed for each attempt of this call.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(r *requestConfig) { r.timeout = d }
}

// WithRequestMaxRetries sets the retries after the first attempt for this call.
func WithRequestMaxRetries(n int) RequestOption {
	return func(r *requestConfig) { r.maxRetries = n }
}

func (c *Client) config(opts []RequestOption) requestConfig {
	r := requestConfig{timeout: c.timeout, maxRetries: c.maxRetries}
	for _, o := range opts {
		o(&r)
	}
	return r
}

// String returns a pointer to s, for optional fields.
func String(s string) *string { return &s }

// Int returns a pointer to n, for optional fields.
func Int(n int) *int { return &n }

// Bool returns a pointer to b, for optional fields (such as AgentTool.Sign).
func Bool(b bool) *bool { return &b }
