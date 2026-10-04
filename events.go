package voicast

import "encoding/json"

// Webhooks sent by voicast (Standard Webhooks body {type, timestamp, data}) and in-call tool requests.

// EventType is the type of a webhook sent by voicast.
type EventType string

// Event types.
const (
	EventCallStarted            EventType = "call.started"
	EventCallHandoffRequested   EventType = "call.handoff_requested"
	EventCallEnded              EventType = "call.ended"
	EventCallTranscriptReady    EventType = "call.transcript_ready"
	EventCallSummaryReady       EventType = "call.summary_ready"
	EventConnectionNeedsReauth  EventType = "connection.needs_reauth"
	EventBillingBalanceLow      EventType = "billing.balance_low"
	EventBillingBalanceDepleted EventType = "billing.balance_depleted"
	EventBillingRechargeFailed  EventType = "billing.recharge_failed"
	EventBillingCapReached      EventType = "billing.cap_reached"
)

// Event is a webhook sent by voicast.
type Event struct {
	Type EventType
	// Timestamp is ISO 8601.
	Timestamp string
	// Data is the event's data. Use ParseData or UnmarshalData.
	Data json.RawMessage
	// Raw is the whole verified body.
	Raw json.RawMessage
}

// UnmarshalData decodes Data into v.
func (e *Event) UnmarshalData(v any) error { return json.Unmarshal(e.Data, v) }

// ParseData decodes Data into the type for the event: *CallStartedData, *CallHandoffRequestedData, *CallEndedData,
// *CallTranscriptReadyData, *CallSummaryReadyData, *ConnectionNeedsReauthData, *BillingBalanceLowData,
// *BillingBalanceDepletedData, *BillingRechargeFailedData or *BillingCapReachedData. An unknown type gives map[string]any.
//
//	switch d := data.(type) {
//	case *voicast.CallEndedData:
//		fmt.Println(d.Outcome, d.Tenant.ExternalID)
//	}
func (e *Event) ParseData() (any, error) {
	var v any
	switch e.Type {
	case EventCallStarted:
		v = &CallStartedData{}
	case EventCallHandoffRequested:
		v = &CallHandoffRequestedData{}
	case EventCallEnded:
		v = &CallEndedData{}
	case EventCallTranscriptReady:
		v = &CallTranscriptReadyData{}
	case EventCallSummaryReady:
		v = &CallSummaryReadyData{}
	case EventConnectionNeedsReauth:
		v = &ConnectionNeedsReauthData{}
	case EventBillingBalanceLow:
		v = &BillingBalanceLowData{}
	case EventBillingBalanceDepleted:
		v = &BillingBalanceDepletedData{}
	case EventBillingRechargeFailed:
		v = &BillingRechargeFailedData{}
	case EventBillingCapReached:
		v = &BillingCapReachedData{}
	default:
		m := map[string]any{}
		if err := json.Unmarshal(e.Data, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	if err := json.Unmarshal(e.Data, v); err != nil {
		return nil, err
	}
	return v, nil
}

// EventTenant is the tenant a call event belongs to.
type EventTenant struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
}

// CallStartedData is the data of call.started.
type CallStartedData struct {
	CallID         string      `json:"call_id"`
	ProviderCallID *string     `json:"provider_call_id"`
	From           *string     `json:"from"`
	StartedAt      int64       `json:"started_at"`
	Tenant         EventTenant `json:"tenant"`
}

// CallHandoffRequestedData is the data of call.handoff_requested.
type CallHandoffRequestedData struct {
	CallID         string            `json:"call_id"`
	ProviderCallID *string           `json:"provider_call_id"`
	Reason         *string           `json:"reason"`
	Fields         map[string]string `json:"fields"`
	Tenant         EventTenant       `json:"tenant"`
}

// CallEndedData is the data of call.ended: the Call and its tenant.
type CallEndedData struct {
	Call
	Tenant EventTenant `json:"tenant"`
}

// CallTranscriptReadyData is the data of call.transcript_ready.
type CallTranscriptReadyData struct {
	CallID         string           `json:"call_id"`
	ProviderCallID *string          `json:"provider_call_id"`
	Turns          []TranscriptTurn `json:"turns"`
	Tenant         EventTenant      `json:"tenant"`
}

// CallSummaryReadyData is the data of call.summary_ready.
type CallSummaryReadyData struct {
	CallID         string            `json:"call_id"`
	ProviderCallID *string           `json:"provider_call_id"`
	Summary        string            `json:"summary"`
	Category       string            `json:"category"`
	Fields         map[string]string `json:"fields"`
	Tenant         EventTenant       `json:"tenant"`
}

// ConnectionNeedsReauthData is the data of connection.needs_reauth.
type ConnectionNeedsReauthData struct {
	ConnectionID string           `json:"connection_id"`
	Connector    string           `json:"connector"`
	Label        string           `json:"label"`
	Status       ConnectionStatus `json:"status"`
	// Tenant is nil for an environment connection.
	Tenant *EventTenant `json:"tenant"`
}

// BillingBalanceLowData is the data of billing.balance_low.
type BillingBalanceLowData struct {
	OrganizationID string  `json:"organization_id"`
	BalanceYen     float64 `json:"balance_yen"`
	ThresholdYen   float64 `json:"threshold_yen"`
}

// BillingBalanceDepletedData is the data of billing.balance_depleted.
type BillingBalanceDepletedData struct {
	OrganizationID string  `json:"organization_id"`
	BalanceYen     float64 `json:"balance_yen"`
	CallID         string  `json:"call_id"`
}

// BillingRechargeFailedData is the data of billing.recharge_failed.
type BillingRechargeFailedData struct {
	OrganizationID       string  `json:"organization_id"`
	InvoiceID            string  `json:"invoice_id"`
	AmountYen            float64 `json:"amount_yen"`
	Attempt              int     `json:"attempt"`
	MaxAttempts          int     `json:"max_attempts"`
	AutoRechargeDisabled bool    `json:"auto_recharge_disabled"`
	DeclineCode          *string `json:"decline_code"`
	BalanceYen           float64 `json:"balance_yen"`
}

// BillingCapReachedData is the data of billing.cap_reached: the auto-recharge monthly cap was reached. Kind is
// "auto_recharge".
type BillingCapReachedData struct {
	OrganizationID string `json:"organization_id"`
	Kind           string `json:"kind"`
	// Month is YYYY-MM (Japan time).
	Month          string  `json:"month"`
	SpendingCapYen float64 `json:"spending_cap_yen"`
	RechargedYen   float64 `json:"recharged_yen"`
	AmountYen      float64 `json:"amount_yen"`
	BalanceYen     float64 `json:"balance_yen"`
}

// ToolCallContext is the call a tool call belongs to.
type ToolCallContext struct {
	// ID is try_… in a test (Tools.Try).
	ID string `json:"id"`
	// Channel is ChannelTest in a test (Tools.Try).
	Channel        CallChannel `json:"channel"`
	ProviderCallID *string     `json:"provider_call_id"`
	// From is the caller's number (phone calls, when your Twilio app passed it).
	From *string `json:"from"`
	// Tenant is nil in a test without a TenantID.
	Tenant *EventTenant `json:"tenant"`
}

// ToolCallRequest is the body of an in-call tool request (config.tools[].url). Reply with 2xx and JSON within
// 8 seconds; the JSON is given to the AI as is.
type ToolCallRequest struct {
	Type string `json:"type"`
	// Tool is the tool's Name.
	Tool string `json:"tool"`
	// ToolCallID is unique per call of a tool; also sent as idempotency-key. Use it to avoid booking twice.
	ToolCallID string `json:"tool_call_id"`
	// Arguments from the AI. Use BindArguments to decode them into your own type.
	Arguments json.RawMessage `json:"arguments"`
	Call      ToolCallContext `json:"call"`
	// Test is true when sent by Tools.Try (or the dashboard's test button), not during a call.
	Test bool `json:"test,omitempty"`
	// Raw is the whole verified body.
	Raw json.RawMessage `json:"-"`
}

// BindArguments decodes Arguments into v.
func (t *ToolCallRequest) BindArguments(v any) error { return json.Unmarshal(t.Arguments, v) }
