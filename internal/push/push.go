// Package push sends push notifications through the three providers the
// API supports (API Reference → Push Notifications): Apple Push
// Notification service (APNs), Firebase Cloud Messaging (FCM HTTP v1), and
// Web Push (RFC 8030, with VAPID per RFC 8292 and payload encryption per
// RFC 8291).
//
// It knows nothing about the database. The core package decides who gets a
// notification and records the outcome; this package turns one
// notification for one device into one provider request, and classifies
// the provider's answer so the caller knows whether to retry, deactivate
// the device, or mark the credential failed.
package push

import (
	"context"
	"encoding/json"
	"time"
)

// Providers.
const (
	APNs    = "apns"
	FCM     = "fcm"
	WebPush = "web_push"
)

// Providers lists every supported provider.
var Providers = []string{APNs, FCM, WebPush}

// Credential is an Application's decrypted credential for one provider.
type Credential struct {
	Provider string

	// APNs (token-based authentication).
	TeamID     string
	KeyID      string
	Topic      string
	PrivateKey string // the .p8 file: PEM PKCS#8, P-256

	// FCM.
	ServiceAccount *ServiceAccount

	// Web Push (VAPID).
	Subject        string
	VAPIDPublicKey string // base64url, uncompressed P-256 point
	VAPIDPrivate   string // base64url, 32-byte scalar
}

// Target is one device: the provider token, or a browser subscription.
type Target struct {
	Provider     string
	Token        string        // apns, fcm
	Environment  string        // apns: production | sandbox
	Subscription *Subscription // web_push
}

// Subscription is a browser PushSubscription.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Notification is what a device shows, plus the platform's own block.
type Notification struct {
	ID            string // ntf_…; empty for a device test
	ApplicationID *string
	Source        string // application | platform
	Category      string
	Priority      string // normal | high
	Title         string
	Body          *string
	Data          map[string]string
	ActionURL     *string
	Badge         *int
	Sound         *string
	CollapseKey   *string
	TTL           time.Duration
}

// Message is one push to one device.
type Message struct {
	Credential   Credential
	Target       Target
	Notification Notification
}

// Outcome classifies a provider's answer.
type Outcome int

const (
	// Sent: the provider accepted the push.
	Sent Outcome = iota
	// Retry: a transient failure (429, 5xx, timeout); try again later.
	Retry
	// TokenGone: the provider says the device token is invalid or
	// unregistered; deactivate the device and never retry.
	TokenGone
	// CredentialRejected: the provider rejected the Application's
	// credential itself; mark it failed.
	CredentialRejected
	// Failed: a permanent failure for this push only.
	Failed
)

func (o Outcome) String() string {
	return [...]string{"sent", "retry", "token_gone", "credential_rejected", "failed"}[o]
}

// Result is the classified answer to one Send.
type Result struct {
	Outcome    Outcome
	Status     int    // the provider's HTTP status; 0 when no response arrived
	Reason     string // the provider's reason code, or "timeout"
	RetryAfter time.Duration
	Duration   time.Duration
}

// Sender sends one message.
type Sender interface {
	Send(ctx context.Context, m Message) Result
}

// MaxPayload is the largest push payload all three providers accept.
const MaxPayload = 4096

// webPushOverhead is the aes128gcm header (86 bytes for a 65-byte key id),
// the padding delimiter, and the AEAD tag, which come out of the 4096-byte
// record Web Push services accept.
const webPushOverhead = 86 + 1 + 16

// substratalBlock is the platform's own block in every payload.
func substratalBlock(n Notification) map[string]any {
	b := map[string]any{"application_id": n.ApplicationID, "source": n.Source, "category": n.Category}
	if n.ID != "" {
		b["notification_id"] = n.ID
	} else {
		b["notification_id"] = nil
	}
	if n.ActionURL != nil {
		b["action_url"] = *n.ActionURL
	}
	return b
}

// APNsPayload is the JSON body sent to APNs.
func APNsPayload(n Notification) []byte {
	alert := map[string]any{"title": n.Title}
	if n.Body != nil {
		alert["body"] = *n.Body
	}
	aps := map[string]any{"alert": alert, "category": n.Category}
	if n.Priority == "high" {
		aps["interruption-level"] = "time-sensitive"
	} else {
		aps["interruption-level"] = "active"
	}
	if n.Badge != nil {
		aps["badge"] = *n.Badge
	}
	if n.Sound != nil {
		aps["sound"] = *n.Sound
	}
	body := map[string]any{}
	for k, v := range n.Data {
		body[k] = v
	}
	body["aps"] = aps
	body["substratal"] = substratalBlock(n)
	b, _ := json.Marshal(body)
	return b
}

// WebPushPayload is the plaintext JSON a service worker receives.
func WebPushPayload(n Notification) []byte {
	body := map[string]any{"title": n.Title, "category": n.Category, "priority": n.Priority,
		"data": n.Data, "substratal": substratalBlock(n)}
	if n.Data == nil {
		body["data"] = map[string]string{}
	}
	if n.Body != nil {
		body["body"] = *n.Body
	}
	if n.Badge != nil {
		body["badge"] = *n.Badge
	}
	if n.CollapseKey != nil {
		body["tag"] = *n.CollapseKey
	}
	b, _ := json.Marshal(body)
	return b
}

// fcmData is the string-only data map FCM carries: the notification's own
// data plus the substratal block as a JSON string.
func fcmData(n Notification) map[string]string {
	out := map[string]string{}
	for k, v := range n.Data {
		out[k] = v
	}
	sb, _ := json.Marshal(substratalBlock(n))
	out["substratal"] = string(sb)
	out["category"] = n.Category
	return out
}

// Size is the largest payload n produces across the providers, in the
// 4096-byte budget they share: the APNs body, and the Web Push plaintext
// plus its encryption overhead. FCM's data message is no larger than
// either. It's what 422 notification_too_large reports.
func Size(n Notification) int {
	a := len(APNsPayload(n))
	w := len(WebPushPayload(n)) + webPushOverhead
	if w > a {
		return w
	}
	return a
}
