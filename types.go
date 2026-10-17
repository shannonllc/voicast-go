package voicast

import "encoding/json"

// Request and response shapes of /v1 (see the API reference). Times are Unix milliseconds.

// List is a list that is not paged.
type List[T any] struct {
	Data []T `json:"data"`
}

// Page is one page of a paged list (Calls.List).
type Page[T any] struct {
	Data []T `json:"data"`
	// HasMore is true when there are more items after this page.
	HasMore bool `json:"has_more"`
}

// Nullable is an optional field that can also be sent as JSON null.
// The zero value is not sent.
type Nullable[T any] struct {
	value *T
	set   bool
}

// Value returns a Nullable that sends v.
func Value[T any](v T) Nullable[T] { return Nullable[T]{value: &v, set: true} }

// Null returns a Nullable that sends JSON null.
func Null[T any]() Nullable[T] { return Nullable[T]{set: true} }

// IsSet reports whether the field is sent.
func (n Nullable[T]) IsSet() bool { return n.set }

// IsNull reports whether the field is sent as null.
func (n Nullable[T]) IsNull() bool { return n.set && n.value == nil }

func (n Nullable[T]) jsonValue() any {
	if n.value == nil {
		return nil
	}
	return *n.value
}

// Agents

// AgentField is something the AI asks the caller.
type AgentField struct {
	// Key is lowercase letters, digits and _.
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required,omitempty"`
	// Type is "text" (default) or "phone".
	Type string `json:"type,omitempty"`
}

// AgentToolParameter is one argument of an AgentTool.
type AgentToolParameter struct {
	// Type is "string", "number", "integer" or "boolean".
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// AgentToolParameters are the arguments of an AgentTool.
type AgentToolParameters struct {
	Properties map[string]AgentToolParameter `json:"properties"`
	Required   []string                      `json:"required,omitempty"`
}

// AgentToolAuth sends an API key with a tool's requests.
type AgentToolAuth struct {
	// Type is "api_key".
	Type string `json:"type"`
	// Header is the header to send the key in. For "Authorization" the key is sent as "Bearer <key>".
	Header string `json:"header"`
	// Value is the key. It is only sent: voicast stores it encrypted per agent, outside the config, and never
	// returns it. Leave it empty on later updates to keep the stored key (see Agent.ToolKeys).
	Value string `json:"value,omitempty"`
}

// AgentTool is a tool the AI can use during a call. By default voicast POSTs a signed tool.call to URL (see
// VerifyToolCall); with Sign set to false it sends the arguments as they are (for third-party APIs).
// Up to 10 per agent.
type AgentTool struct {
	// Name is lowercase letters, digits and _. "finish" is reserved.
	Name string `json:"name"`
	// Description says when to use it. The AI reads this to decide.
	Description string `json:"description"`
	// URL is a public https URL.
	URL string `json:"url"`
	// Method is "GET" or "POST" (default). Signed tools must use "POST".
	Method string `json:"method,omitempty"`
	// Sign defaults to true: a signed tool.call request. voicast.Bool(false) sends the arguments as they are,
	// in the query string (GET) or as the JSON body (POST).
	Sign *bool `json:"sign,omitempty"`
	// Auth sends an API key with every request.
	Auth       *AgentToolAuth       `json:"auth,omitempty"`
	Parameters *AgentToolParameters `json:"parameters,omitempty"`
	// Confirm: voicast asks the caller with a fixed confirmation message made from the arguments, runs the tool only
	// after the caller agrees, and tells the result in a fixed message.
	Confirm bool `json:"confirm,omitempty"`
	// ConfirmMessage is the confirmation message template for a Confirm tool (up to 200 characters).
	// "{argument name}" inserts the value; date and time values are read as spoken dates.
	ConfirmMessage string `json:"confirm_message,omitempty"`
	// DoneMessage is said when a Confirm tool succeeds (up to 200 characters, "{argument name}" can be inserted).
	DoneMessage string `json:"done_message,omitempty"`
}

// AgentConnectorEntry is a connector used by the agent. Connector picks a preset (the tenant's connection, else the
// environment's); Connection picks one connection (con_…, required for custom connections).
type AgentConnectorEntry struct {
	Connector  string `json:"connector,omitempty"`
	Connection string `json:"connection,omitempty"`
	// Actions are action names (there is no wildcard: actions added later are not included).
	Actions []string `json:"actions,omitempty"`
	// Confirm lists read actions that are read back to the caller for consent first.
	Confirm []string `json:"confirm,omitempty"`
}

// FAQEntry is a question the AI answers.
type FAQEntry struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// AgentConfig is what the AI says and asks.
type AgentConfig struct {
	// Greeting is the first line, spoken as is.
	Greeting string `json:"greeting"`
	// Persona is how the AI talks.
	Persona string `json:"persona"`
	// Fields are what to ask the caller.
	Fields []AgentField `json:"fields"`
	// HandoffWhen lists when to hand the call over to a person.
	HandoffWhen []string `json:"handoff_when"`
	// Closing is said when every field is collected.
	Closing string `json:"closing"`
	// ClosingAnswered is said when an FAQ answered the caller's question.
	ClosingAnswered string     `json:"closing_answered,omitempty"`
	FAQ             []FAQEntry `json:"faq,omitempty"`
	// Domain is the business domain (speech recognition hint).
	Domain string `json:"domain,omitempty"`
	// Vocabulary are terms (speech recognition hint).
	Vocabulary []string `json:"vocabulary,omitempty"`
	// Voice is a voice ID, such as "female-breezy" (default), "female-firm" or "male-calm".
	Voice string `json:"voice,omitempty"`
	// SpeakingRate is 0.85, 0.9, 0.95, 1, 1.05, 1.1, 1.15 (default), 1.2, 1.25, 1.3, 1.4 or 1.5. 0 is not sent.
	SpeakingRate float64               `json:"speaking_rate,omitempty"`
	Tools        []AgentTool           `json:"tools,omitempty"`
	Connectors   []AgentConnectorEntry `json:"connectors,omitempty"`
}

// Agent is what the AI says and asks. Every update adds a version.
type Agent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	// Config is set on Create, Get and Update; nil in List.
	Config *AgentConfig `json:"config,omitempty"`
	// ToolKeys are the tools' stored API keys. Set on Create, Get and Update; nil in List.
	ToolKeys  []AgentToolKey `json:"tool_keys,omitempty"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

// AgentToolKey is an API key stored for one of the agent's tools (the key itself is never returned).
type AgentToolKey struct {
	// Name is the tool's Name.
	Name string `json:"name"`
	// Hint is the last 4 characters of the key; empty for keys shorter than 12 characters.
	Hint      string `json:"hint"`
	UpdatedAt int64  `json:"updated_at"`
}

// AgentCreateParams creates an agent.
type AgentCreateParams struct {
	Name   string      `json:"name"`
	Config AgentConfig `json:"config"`
}

// AgentUpdateParams saves a new version. An empty Name keeps the current name.
type AgentUpdateParams struct {
	Name   string      `json:"name,omitempty"`
	Config AgentConfig `json:"config"`
}

// AgentGetParams selects a version. 0 is the current version.
type AgentGetParams struct {
	Version int
}

// AgentVersion is one saved version.
type AgentVersion struct {
	Version   int   `json:"version"`
	CreatedAt int64 `json:"created_at"`
}

// Tenants

// TenantStatus is "active" or "disabled".
type TenantStatus string

// Tenant statuses.
const (
	TenantActive   TenantStatus = "active"
	TenantDisabled TenantStatus = "disabled"
)

// Tenant is a business that actually takes the calls.
type Tenant struct {
	ID string `json:"id"`
	// ExternalID is your ID for the business. Letters, digits and . _ : -, 1 to 100 characters.
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	// AgentID is nil when the tenant has no agent.
	AgentID   *string      `json:"agent_id"`
	Status    TenantStatus `json:"status"`
	CreatedAt int64        `json:"created_at"`
}

// TenantWithSecret is returned by Tenants.Create.
type TenantWithSecret struct {
	Tenant
	// Secret (vts_…) for the phone stream. Returned only here.
	Secret string `json:"secret"`
}

// TenantCreateParams creates a tenant.
type TenantCreateParams struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
}

// TenantUpdateParams sends only the fields that are set.
type TenantUpdateParams struct {
	Name *string
	// AgentID: Null[string]() removes the tenant's agent.
	AgentID Nullable[string]
	Status  *TenantStatus
}

// MarshalJSON sends only the fields that are set.
func (p TenantUpdateParams) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if p.Name != nil {
		m["name"] = *p.Name
	}
	if p.AgentID.set {
		m["agent_id"] = p.AgentID.jsonValue()
	}
	if p.Status != nil {
		m["status"] = *p.Status
	}
	return json.Marshal(m)
}

// TenantSecret is returned by Tenants.RotateSecret.
type TenantSecret struct {
	Secret string `json:"secret"`
	// PreviousSecretExpiresAt: the previous secret keeps working until this time.
	PreviousSecretExpiresAt int64 `json:"previous_secret_expires_at"`
}

// SipCredential is the Digest credentials for a PBX that sends calls over SIP to sip.voicast.jp.
type SipCredential struct {
	// ID is sipc_….
	ID string `json:"id"`
	// Username is the Digest user name (the same as ID).
	Username string `json:"username"`
	// Realm is the Digest realm ("voicast").
	Realm string `json:"realm"`
	// Server is where the PBX sends calls ("sip.voicast.jp").
	Server   string `json:"server"`
	TenantID string `json:"tenant_id"`
	// AgentID is the agent that takes calls on these credentials. nil uses the tenant's agent.
	AgentID *string `json:"agent_id"`
	Name    string  `json:"name"`
	// TransferTo is where the PBX transfers the call (with SIP REFER) when the AI hands off to a person (outcome
	// handoff): an extension or number, or a SIP URI (sip: / sips:). Empty means no transfer.
	TransferTo string `json:"transfer_to"`
	CreatedAt  int64  `json:"created_at"`
}

// SipCredentialWithPassword is returned by Tenants.CreateSipCredential.
type SipCredentialWithPassword struct {
	SipCredential
	// Password is the Digest password. Returned only here.
	Password string `json:"password"`
}

// SipCredentialCreateParams creates SIP credentials.
type SipCredentialCreateParams struct {
	// Name tells the PBX apart (up to 100 characters).
	Name string `json:"name,omitempty"`
	// AgentID is the agent that takes calls on these credentials. Empty uses the tenant's agent.
	AgentID string `json:"agent_id,omitempty"`
	// TransferTo is where the PBX transfers the call on a handoff: an extension or number (digits and * # +, up to
	// 32 characters) or a SIP URI (sip: / sips:, up to 200 characters). Empty means no transfer.
	TransferTo string `json:"transfer_to,omitempty"`
}

// SipCredentialUpdateParams sends only the fields that are set.
type SipCredentialUpdateParams struct {
	Name *string
	// AgentID: Null[string]() uses the tenant's agent.
	AgentID Nullable[string]
	// TransferTo: Null[string]() or Value("") turns the transfer off.
	TransferTo Nullable[string]
}

// MarshalJSON sends only the fields that are set.
func (p SipCredentialUpdateParams) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	if p.Name != nil {
		m["name"] = *p.Name
	}
	if p.AgentID.set {
		m["agent_id"] = p.AgentID.jsonValue()
	}
	if p.TransferTo.set {
		m["transfer_to"] = p.TransferTo.jsonValue()
	}
	return json.Marshal(m)
}

// ConnectLinkParams creates a link to the page where the business connects its services.
type ConnectLinkParams struct {
	// Connectors to show. Default: every connector that is not env_only.
	Connectors []string `json:"connectors,omitempty"`
	// FrameOrigin is the https origin of the page that embeds the link in an iframe.
	FrameOrigin string `json:"frame_origin,omitempty"`
}

// Link is valid for 15 minutes.
type Link struct {
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

// Calls

// CallOutcome is completed, answered, handoff, hangup or error.
type CallOutcome string

// Call outcomes.
const (
	OutcomeCompleted CallOutcome = "completed"
	OutcomeAnswered  CallOutcome = "answered"
	OutcomeHandoff   CallOutcome = "handoff"
	OutcomeHangup    CallOutcome = "hangup"
	OutcomeError     CallOutcome = "error"
)

// CallChannel is "phone" (Twilio) or "web" (WebRTC). Tool requests from Tools.Try have "test".
type CallChannel string

// Call channels.
const (
	ChannelPhone CallChannel = "phone"
	ChannelWeb   CallChannel = "web"
	// ChannelTest is only in ToolCallRequest.Call, for requests sent by Tools.Try.
	ChannelTest CallChannel = "test"
)

// CallOutput is "audio" (the AI speaks) or "text" (replies as text only).
type CallOutput string

// Call outputs.
const (
	OutputAudio CallOutput = "audio"
	OutputText  CallOutput = "text"
)

// CallAgent is the agent version a call used.
type CallAgent struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// Call is one phone or Web call.
type Call struct {
	ID       string    `json:"id"`
	TenantID string    `json:"tenant_id"`
	Agent    CallAgent `json:"agent"`
	// ProviderCallID is Twilio's CallSid for phone calls, the session ID for Web calls.
	ProviderCallID *string      `json:"provider_call_id"`
	From           *string      `json:"from"`
	StartedAt      int64        `json:"started_at"`
	EndedAt        *int64       `json:"ended_at"`
	DurationMs     *int64       `json:"duration_ms"`
	Outcome        *CallOutcome `json:"outcome"`
	Reason         *string      `json:"reason"`
	// Fields are what was collected from the caller.
	Fields map[string]string `json:"fields"`
	// BilledYen is the fee for the call (yen before tax, up to 3 decimals): the amount taken from the balance (if not
	// yet deducted, seconds × your per-minute price ÷ 60). nil until the call ends.
	BilledYen *float64 `json:"billed_yen"`
	// Summary is written after the call. nil until call.summary_ready.
	Summary       *string     `json:"summary"`
	Category      *string     `json:"category"`
	HasTranscript bool        `json:"has_transcript"`
	Channel       CallChannel `json:"channel"`
	Output        CallOutput  `json:"output"`
}

// CallListParams filters Calls.List.
type CallListParams struct {
	TenantID string
	// Before: only calls that started before this time (Unix milliseconds). 0 is not sent.
	Before int64
	// Limit is 1 to 100. Default 50. 0 is not sent.
	Limit int
}

// TranscriptTurn is one turn of the conversation. Role is "assistant" or "user".
type TranscriptTurn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// Transcript of a call.
type Transcript struct {
	CallID string           `json:"call_id"`
	Turns  []TranscriptTurn `json:"turns"`
}

// Sessions

// SessionCreateParams creates a session for a browser or an app.
type SessionCreateParams struct {
	TenantID string `json:"tenant_id"`
	// AgentID defaults to the tenant's agent.
	AgentID string `json:"agent_id,omitempty"`
	// Output defaults to OutputAudio.
	Output CallOutput `json:"output,omitempty"`
}

// Session for talking from a browser or an app.
type Session struct {
	// ID is used with Calls.GetByProvider after the call.
	ID       string     `json:"id"`
	TenantID string     `json:"tenant_id"`
	AgentID  string     `json:"agent_id"`
	Output   CallOutput `json:"output"`
	// ClientToken (vct_…) is passed to the browser or app. It expires in 2 minutes and can be used once.
	ClientToken string `json:"client_token"`
	ExpiresAt   int64  `json:"expires_at"`
	// WebRTCURL is the signalling URL. Pass it to the browser or app.
	WebRTCURL string `json:"webrtc_url"`
}

// Usage

// UsageParams selects the period and grouping of Usage.Get.
type UsageParams struct {
	// From is YYYY-MM-DD (Japan time, inclusive) or Unix milliseconds as a string. Default: the start of this month.
	From string
	// To is YYYY-MM-DD (Japan time, inclusive) or Unix milliseconds (exclusive) as a string. Default: now.
	To string
	// GroupBy is "tenant", "day" or "channel".
	GroupBy string
	// Channel keeps only "phone" or "web" calls. With GroupBy "tenant", phone minutes per tenant.
	Channel CallChannel
	// TenantID keeps only this tenant (ten_…).
	TenantID string
	// ExternalID keeps only the existing tenant with this external_id.
	ExternalID string
}

// UsageFilter is the filters applied (nil when not given).
type UsageFilter struct {
	Channel    *CallChannel `json:"channel"`
	TenantID   *string      `json:"tenant_id"`
	ExternalID *string      `json:"external_id"`
}

// UsageTotals are counts for a period or a group.
type UsageTotals struct {
	Calls   int64 `json:"calls"`
	Seconds int64 `json:"seconds"`
	Minutes int64 `json:"minutes"`
	// AmountYen is an estimate before tax: seconds / 60 × UnitPriceYen.
	AmountYen float64 `json:"amount_yen"`
}

// UsageTenant identifies the tenant of a usage row.
type UsageTenant struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
}

// UsageRow is one group. Tenant, Day or Channel is set depending on GroupBy.
type UsageRow struct {
	UsageTotals
	Tenant  *UsageTenant `json:"tenant,omitempty"`
	Day     string       `json:"day,omitempty"`
	Channel CallChannel  `json:"channel,omitempty"`
}

// Usage is the minutes used in a period.
type Usage struct {
	// Environment is "live" or "test".
	Environment  string      `json:"environment"`
	From         int64       `json:"from"`
	To           int64       `json:"to"`
	GroupBy      *string     `json:"group_by"`
	Filter       UsageFilter `json:"filter"`
	UnitPriceYen float64     `json:"unit_price_yen"`
	Total        UsageTotals `json:"total"`
	// Data is empty when GroupBy is not given.
	Data []UsageRow `json:"data"`
}

// Connectors and connections

// ConnectorInput is asked before connecting.
type ConnectorInput struct {
	Name string `json:"name"`
	// Type is "text", "password", "url" or "number".
	Type string `json:"type"`
}

// ConnectorAction is an operation the AI can use. Tool is the name shown to the AI.
type ConnectorAction struct {
	Name        string `json:"name"`
	Tool        string `json:"tool"`
	Description string `json:"description"`
	Confirm     bool   `json:"confirm"`
}

// Connector is a service that can be connected.
type Connector struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Category is "scheduling", "crm", "notify" or "custom".
	Category string `json:"category"`
	// Auth is "oauth2", "api_key" or "webhook_url".
	Auth       string            `json:"auth"`
	Available  bool              `json:"available"`
	NeedsSetup bool              `json:"needs_setup"`
	Notify     bool              `json:"notify"`
	EnvOnly    bool              `json:"env_only"`
	Inputs     []ConnectorInput  `json:"inputs"`
	Actions    []ConnectorAction `json:"actions"`
}

// ConnectionStatus is active, needs_setup or needs_reauth.
type ConnectionStatus string

// Connection statuses.
const (
	ConnectionActive      ConnectionStatus = "active"
	ConnectionNeedsSetup  ConnectionStatus = "needs_setup"
	ConnectionNeedsReauth ConnectionStatus = "needs_reauth"
)

// Connection to a calendar, CRM or chat.
type Connection struct {
	ID        string  `json:"id"`
	Connector string  `json:"connector"`
	TenantID  *string `json:"tenant_id"`
	// Label is the account on the other side, such as clinic@example.com.
	Label     string           `json:"label"`
	Status    ConnectionStatus `json:"status"`
	Config    map[string]any   `json:"config"`
	CreatedBy *string          `json:"created_by"`
	CreatedAt int64            `json:"created_at"`
	UpdatedAt int64            `json:"updated_at"`
}

// ConnectionListParams filters Connections.List. The zero value lists every connection.
type ConnectionListParams struct {
	// TenantID: not set lists every connection, Null[string]() the environment's connections only, Value(id) that tenant's.
	TenantID Nullable[string]
}

// ConnectionCreateParams connects a connector set up with an API key or a webhook URL (teams, http).
type ConnectionCreateParams struct {
	Connector string `json:"connector"`
	// Secret is the API key or webhook URL. Stored encrypted; never returned.
	Secret string            `json:"secret"`
	Input  map[string]string `json:"input,omitempty"`
	// TenantID: empty for an environment connection.
	TenantID string `json:"tenant_id,omitempty"`
}

// ConnectionUpdateParams changes only the config fields given.
type ConnectionUpdateParams struct {
	Config map[string]any `json:"config"`
}

// WebhookPortalParams creates a link to the webhook portal.
type WebhookPortalParams struct {
	// FrameOrigin is the https origin of the page that embeds the portal in an iframe.
	FrameOrigin string `json:"frame_origin,omitempty"`
}

// Billing (one per organization, shared by the live and test environments)

// BillingContract is the organization's contract.
type BillingContract struct {
	PerMinuteYen float64 `json:"per_minute_yen"`
}

// BillingPrice is the price per minute (yen before tax).
type BillingPrice struct {
	PerMinuteYen float64 `json:"per_minute_yen"`
}

// BillingUsage is this month's usage of the organization.
type BillingUsage struct {
	Calls   int64 `json:"calls"`
	Seconds int64 `json:"seconds"`
	Minutes int64 `json:"minutes"`
}

// BillingExpiry is the next top-up to expire.
type BillingExpiry struct {
	Yen float64 `json:"yen"`
	At  int64   `json:"at"`
}

// BillingBalance is the prepaid balance.
type BillingBalance struct {
	// Yen is rounded down to the yen.
	Yen      float64 `json:"yen"`
	MilliYen int64   `json:"milli_yen"`
	// Depleted is true at 0 or below: new calls are refused (balance_depleted).
	Depleted   bool           `json:"depleted"`
	NextExpiry *BillingExpiry `json:"next_expiry"`
}

// BillingAutoRecharge is the auto-recharge setting.
type BillingAutoRecharge struct {
	Enabled               bool     `json:"enabled"`
	ThresholdYen          float64  `json:"threshold_yen"`
	AmountYen             float64  `json:"amount_yen"`
	MonthlyCapYen         *float64 `json:"monthly_cap_yen"`
	RechargedThisMonthYen float64  `json:"recharged_this_month_yen"`
	Failures              int      `json:"failures"`
	MaxFailures           int      `json:"max_failures"`
}

// BillingTopupRange is the range of a top-up.
type BillingTopupRange struct {
	MinYen       int64   `json:"min_yen"`
	MaxYen       int64   `json:"max_yen"`
	QuickYen     []int64 `json:"quick_yen"`
	CreditMonths int     `json:"credit_months"`
}

// BillingThresholdRange is the range of the auto-recharge threshold.
type BillingThresholdRange struct {
	MinYen int64 `json:"min_yen"`
	MaxYen int64 `json:"max_yen"`
}

// Billing is the organization's billing. Calls are paid from a prepaid balance topped up by card.
type Billing struct {
	OrganizationID string `json:"organization_id"`
	// Month is YYYY-MM (Japan time).
	Month         string          `json:"month"`
	Contract      BillingContract `json:"contract"`
	Price         BillingPrice    `json:"price"`
	StripeEnabled bool            `json:"stripe_enabled"`
	HasCard       bool            `json:"has_card"`
	// Usage is the calls that ended this month, live and test environments together.
	Usage BillingUsage `json:"usage"`
	// EstimateYen is what was taken from the balance this month (yen before tax).
	EstimateYen  float64               `json:"estimate_yen"`
	Balance      BillingBalance        `json:"balance"`
	AutoRecharge BillingAutoRecharge   `json:"auto_recharge"`
	Topup        BillingTopupRange     `json:"topup"`
	Threshold    BillingThresholdRange `json:"threshold"`
	// SpendingCapYen is the monthly cap on auto-recharge (yen before tax). nil means no cap.
	SpendingCapYen *float64 `json:"spending_cap_yen"`
	// CapReached is true when the auto-recharge monthly cap has been reached.
	CapReached bool `json:"cap_reached"`
	// Tax is "excluded": amounts are before tax.
	Tax     string  `json:"tax"`
	TaxRate float64 `json:"tax_rate"`
}

// TopupParams creates a top-up.
type TopupParams struct {
	// AmountYen is 500 to 300,000 yen, before tax.
	AmountYen int64 `json:"amount_yen"`
}

// Topup is a Stripe Checkout URL for a top-up. Open it in a browser to pay.
type Topup struct {
	URL string `json:"url"`
	// AmountYen is before tax.
	AmountYen int64 `json:"amount_yen"`
	TaxYen    int64 `json:"tax_yen"`
}

// Retention is how long call records (from the end of the call) and recordings (from when they were saved) are
// kept in the API key's environment.
type Retention struct {
	// RetentionDays is 1, 7, 14 or 30.
	RetentionDays int `json:"retention_days"`
	// Choices is the values you can set: [1, 7, 14, 30].
	Choices []int `json:"choices"`
}

// RetentionUpdateParams changes the retention period. RetentionDays is 1, 7, 14 or 30 (30 is the default and the
// maximum).
type RetentionUpdateParams struct {
	RetentionDays int `json:"retention_days"`
}

// ToolsSecret is the environment's signing secret for in-call tool requests.
type ToolsSecret struct {
	Secret string `json:"secret"`
	// PreviousSecretExpiresAt: until this time the previous secret also signs (after a rotation). nil otherwise.
	PreviousSecretExpiresAt *int64 `json:"previous_secret_expires_at"`
}

// ToolTryParams tests a tool. Set ToolName (a tool saved in AgentID's agent) or Tool (a definition, before saving).
type ToolTryParams struct {
	// ToolName is the Name of a tool saved in AgentID's agent.
	ToolName string
	// Tool is a tool definition. It must carry its own Auth.Value: the agent's stored API key is used only with
	// ToolName (a definition with Auth and no Value returns 422 missing_api_key).
	Tool *AgentTool
	// AgentID is required with ToolName.
	AgentID string
	// Arguments for the tool. nil sends none.
	Arguments map[string]any
	// TenantID is sent as call.tenant. Empty: call.tenant is null.
	TenantID string
}

// MarshalJSON sends Tool when set, else ToolName, as "tool".
func (p ToolTryParams) MarshalJSON() ([]byte, error) {
	var tool any = p.ToolName
	if p.Tool != nil {
		tool = p.Tool
	}
	return json.Marshal(struct {
		AgentID   string         `json:"agent_id,omitempty"`
		Tool      any            `json:"tool"`
		Arguments map[string]any `json:"arguments,omitempty"`
		TenantID  string         `json:"tenant_id,omitempty"`
	}{p.AgentID, tool, p.Arguments, p.TenantID})
}

// ToolTryResult is the outcome of Tools.Try. A failing tool is still a 200 response with OK false.
type ToolTryResult struct {
	// Tool is the tool's Name.
	Tool string `json:"tool"`
	OK   bool   `json:"ok"`
	// Result is what the AI would get (when OK).
	Result json.RawMessage `json:"result,omitempty"`
	// Error is set when not OK: "unauthorized", "not_found", "upstream_error", "timeout", "invalid_arguments",
	// "missing_api_key" and so on.
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	// Status is the HTTP status of the tool's response; nil when it did not respond.
	Status *int `json:"status"`
	// Body is the tool's response body, up to 10,000 characters; nil when it did not respond.
	Body *string `json:"body"`
	// Truncated is true when Body was cut at 10,000 characters.
	Truncated  bool  `json:"truncated"`
	DurationMS int64 `json:"duration_ms"`
}
