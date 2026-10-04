package voicast

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Kind classifies an [Error].
type Kind string

// Error kinds. Match them with errors.Is and the Err… sentinels, or read [Error.Kind].
const (
	// KindAuthentication is 401: the API key is missing, wrong or revoked.
	KindAuthentication Kind = "authentication"
	// KindPaymentRequired is 402: a new call or session was refused for billing
	// (balance_depleted).
	KindPaymentRequired Kind = "payment_required"
	// KindPermission is 403: the operation is not allowed.
	KindPermission Kind = "permission"
	// KindNotFound is 404: the resource does not exist in the key's environment.
	KindNotFound Kind = "not_found"
	// KindConflict is 409: conflicts with the current state (duplicate external_id, agent in_use, tenant_disabled, needs_reauth).
	KindConflict Kind = "conflict"
	// KindValidation is 400 or 422: the request body or parameters are invalid. See Details.
	KindValidation Kind = "validation"
	// KindRateLimit is 429: too many requests, returned after retries are used up.
	KindRateLimit Kind = "rate_limit"
	// KindAPI is any other error response, including 5xx.
	KindAPI Kind = "api"
	// KindConnection means the request could not reach the API (DNS, TCP, TLS) or was cancelled.
	KindConnection Kind = "connection"
	// KindTimeout means the request did not complete within the timeout. It also matches ErrConnection.
	KindTimeout Kind = "timeout"
	// KindWebhookVerification means signature verification of an incoming webhook or tool call failed.
	KindWebhookVerification Kind = "webhook_verification"
	// KindInvalidArgument is a usage error, such as a missing API key or an invalid webhook secret.
	KindInvalidArgument Kind = "invalid_argument"
)

// Sentinels for errors.Is.
var (
	ErrAuthentication      = errors.New("voicast: authentication error")
	ErrPaymentRequired     = errors.New("voicast: payment required")
	ErrPermission          = errors.New("voicast: permission error")
	ErrNotFound            = errors.New("voicast: not found")
	ErrConflict            = errors.New("voicast: conflict")
	ErrValidation          = errors.New("voicast: validation error")
	ErrRateLimit           = errors.New("voicast: rate limited")
	ErrAPI                 = errors.New("voicast: api error")
	ErrConnection          = errors.New("voicast: connection error")
	ErrTimeout             = errors.New("voicast: timeout")
	ErrWebhookVerification = errors.New("voicast: webhook verification failed")
	ErrInvalidArgument     = errors.New("voicast: invalid argument")
)

var sentinels = map[Kind]error{
	KindAuthentication:      ErrAuthentication,
	KindPaymentRequired:     ErrPaymentRequired,
	KindPermission:          ErrPermission,
	KindNotFound:            ErrNotFound,
	KindConflict:            ErrConflict,
	KindValidation:          ErrValidation,
	KindRateLimit:           ErrRateLimit,
	KindAPI:                 ErrAPI,
	KindConnection:          ErrConnection,
	KindTimeout:             ErrTimeout,
	KindWebhookVerification: ErrWebhookVerification,
	KindInvalidArgument:     ErrInvalidArgument,
}

// Error is every error returned by the SDK. Use errors.As to read it, or errors.Is with a sentinel:
//
//	var e *voicast.Error
//	if errors.As(err, &e) { fmt.Println(e.Status, e.Code, e.Details, e.RequestID) }
//	if errors.Is(err, voicast.ErrPaymentRequired) { … }
type Error struct {
	Kind Kind
	// Message from the API, or a description of the failure.
	Message string
	// Status is the HTTP status. 0 when no response was received.
	Status int
	// Code is the API error code, such as "invalid_request", "in_use" or "balance_depleted".
	Code string
	// Details is extra information from the API, such as the list of invalid fields for a 422. nil when absent.
	Details json.RawMessage
	// RequestID (x-request-id, or Cloudflare's cf-ray) to quote when contacting support.
	RequestID string
	// RetryAfter is the wait from retry-after on a 429, or nil.
	RetryAfter *time.Duration
	// Err is the underlying cause, if any.
	Err error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.Err }

// Is reports whether target is the sentinel for e's kind. A timeout also matches ErrConnection.
func (e *Error) Is(target error) bool {
	if target == sentinels[e.Kind] {
		return true
	}
	return e.Kind == KindTimeout && target == ErrConnection
}

func newError(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, Err: cause}
}

// parseRetryAfter returns the wait from retry-after (seconds or HTTP date).
func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return time.Duration(math.Max(0, n) * float64(time.Second)), true
	}
	at, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	d := at.Sub(now)
	if d < 0 {
		d = 0
	}
	return d, true
}

func requestIDOf(h http.Header) string {
	if v := h.Get("x-request-id"); v != "" {
		return v
	}
	return h.Get("cf-ray")
}

// jsonString decodes raw when it is a JSON string (not null, a number or an object).
func jsonString(raw json.RawMessage) (string, bool) {
	var s string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// errorFromResponse builds the error for a non-2xx response.
func errorFromResponse(status int, body []byte, h http.Header, now time.Time) *Error {
	var parsed map[string]json.RawMessage
	_ = json.Unmarshal(body, &parsed) // HTML from a proxy, empty body, etc. leave it nil
	e := &Error{Status: status, Message: fmt.Sprintf("HTTP %d", status), RequestID: requestIDOf(h)}
	if s, ok := jsonString(parsed["error"]); ok {
		e.Code = s
	}
	if s, ok := jsonString(parsed["message"]); ok {
		e.Message = s
	}
	if d, ok := parsed["details"]; ok && string(d) != "null" {
		e.Details = d
	}
	switch status {
	case 401:
		e.Kind = KindAuthentication
	case 402:
		e.Kind = KindPaymentRequired
	case 403:
		e.Kind = KindPermission
	case 404:
		e.Kind = KindNotFound
	case 409:
		e.Kind = KindConflict
	case 400, 422:
		e.Kind = KindValidation
	case 429:
		e.Kind = KindRateLimit
		if d, ok := parseRetryAfter(h.Get("retry-after"), now); ok {
			e.RetryAfter = &d
		}
	default:
		e.Kind = KindAPI
	}
	return e
}
