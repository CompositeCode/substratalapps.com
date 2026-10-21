package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Provider endpoints.
const (
	APNsProduction = "https://api.push.apple.com"
	APNsSandbox    = "https://api.sandbox.push.apple.com"
	FCMEndpoint    = "https://fcm.googleapis.com"
	fcmScope       = "https://www.googleapis.com/auth/firebase.messaging"
)

// sendTimeout is the per-request timeout (API Reference → Push
// Notifications → Delivery: a 5-second timeout counts as a retryable failure).
const sendTimeout = 5 * time.Second

// Live sends to the real providers. The zero value is not usable; build it
// with NewLive. Provider auth (the APNs provider JWT, the FCM OAuth access
// token, and the VAPID JWT) is cached per credential, so a warm worker
// signs or fetches each at most once per validity window.
type Live struct {
	// Client must speak HTTP/2 to reach APNs; Go's default transport does
	// over TLS.
	Client *http.Client

	APNsProductionURL string
	APNsSandboxURL    string
	FCMURL            string
	// TokenURL, when set, overrides every service account's token_uri.
	TokenURL string

	// CheckHost vets a Web Push endpoint's host at send time (the same
	// public-address rule as webhook URLs). Nil skips the check.
	CheckHost func(ctx context.Context, host string) error
	// AllowHTTP accepts plain-http Web Push endpoints (local fakes only).
	AllowHTTP bool

	Now func() time.Time

	mu     sync.Mutex
	tokens map[string]cachedToken
}

type cachedToken struct {
	value   string
	expires time.Time
}

// NewLive returns a Live sender for the real provider endpoints.
func NewLive() *Live {
	return &Live{
		Client:            &http.Client{Timeout: sendTimeout, CheckRedirect: noRedirects},
		APNsProductionURL: APNsProduction, APNsSandboxURL: APNsSandbox, FCMURL: FCMEndpoint,
		Now: time.Now,
	}
}

func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

func (l *Live) now() time.Time {
	if l.Now == nil {
		return time.Now()
	}
	return l.Now()
}

func (l *Live) cached(key string, mint func() (string, time.Time, error)) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tokens == nil {
		l.tokens = map[string]cachedToken{}
	}
	if t, ok := l.tokens[key]; ok && l.now().Before(t.expires) {
		return t.value, nil
	}
	v, exp, err := mint()
	if err != nil {
		return "", err
	}
	l.tokens[key] = cachedToken{v, exp}
	return v, nil
}

func (l *Live) forget(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.tokens, key)
}

// Send sends one message and classifies the answer.
func (l *Live) Send(ctx context.Context, m Message) Result {
	start := time.Now()
	var r Result
	switch m.Target.Provider {
	case APNs:
		r = l.sendAPNs(ctx, m)
	case FCM:
		r = l.sendFCM(ctx, m)
	case WebPush:
		r = l.sendWebPush(ctx, m)
	default:
		r = Result{Outcome: Failed, Reason: "unknown provider"}
	}
	r.Duration = time.Since(start)
	return r
}

// do runs one request, returning the status and up to 4 KB of body. A
// transport failure is a retryable timeout or connection error.
func (l *Live) do(ctx context.Context, req *http.Request) (int, []byte, http.Header, *Result) {
	rctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	resp, err := l.Client.Do(req.WithContext(rctx))
	if err != nil {
		reason := "connection_error"
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Timeout") {
			reason = "timeout"
		}
		return 0, nil, nil, &Result{Outcome: Retry, Reason: reason}
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	return resp.StatusCode, b, resp.Header, nil
}

func retryAfter(h http.Header) time.Duration {
	if h == nil {
		return 0
	}
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

// ── APNs ───────────────────────────────────────────────────────────────

// apnsJWTLifetime keeps the provider token inside Apple's 20–60 minute
// refresh window: refreshed after 50 minutes, never more often than 20.
const apnsJWTLifetime = 50 * time.Minute

func (l *Live) apnsToken(c Credential) (string, error) {
	return l.cached("apns:"+c.TeamID+":"+c.KeyID, func() (string, time.Time, error) {
		k, err := ParseAPNsKey(c.PrivateKey)
		if err != nil {
			return "", time.Time{}, err
		}
		now := l.now()
		tok, err := signES256(k, map[string]string{"alg": "ES256", "kid": c.KeyID},
			map[string]any{"iss": c.TeamID, "iat": now.Unix()})
		return tok, now.Add(apnsJWTLifetime), err
	})
}

func (l *Live) sendAPNs(ctx context.Context, m Message) Result {
	c, n := m.Credential, m.Notification
	tok, err := l.apnsToken(c)
	if err != nil {
		return Result{Outcome: CredentialRejected, Reason: "InvalidProviderKey"}
	}
	base := l.APNsProductionURL
	if m.Target.Environment == "sandbox" {
		base = l.APNsSandboxURL
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/3/device/"+url.PathEscape(m.Target.Token), bytes.NewReader(APNsPayload(n)))
	req.Header.Set("Authorization", "bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-topic", c.Topic)
	req.Header.Set("apns-push-type", "alert")
	if n.Priority == "high" {
		req.Header.Set("apns-priority", "10")
	} else {
		req.Header.Set("apns-priority", "5")
	}
	exp := "0"
	if n.TTL > 0 {
		exp = strconv.FormatInt(l.now().Add(n.TTL).Unix(), 10)
	}
	req.Header.Set("apns-expiration", exp)
	if n.CollapseKey != nil {
		req.Header.Set("apns-collapse-id", *n.CollapseKey)
	}
	status, body, h, fail := l.do(ctx, req)
	if fail != nil {
		return *fail
	}
	var e struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(body, &e)
	r := Result{Status: status, Reason: e.Reason, RetryAfter: retryAfter(h)}
	switch {
	case status == http.StatusOK:
		r.Outcome = Sent
	case status == http.StatusGone, e.Reason == "BadDeviceToken", e.Reason == "DeviceTokenNotForTopic", e.Reason == "Unregistered":
		r.Outcome = TokenGone
	case e.Reason == "ExpiredProviderToken":
		// Ours to fix: mint a fresh token and try again.
		l.forget("apns:" + c.TeamID + ":" + c.KeyID)
		r.Outcome = Retry
	case status == http.StatusForbidden, e.Reason == "TopicDisallowed", e.Reason == "BadTopic":
		r.Outcome = CredentialRejected
	case status == http.StatusTooManyRequests || status >= 500:
		r.Outcome = Retry
	default:
		r.Outcome = Failed
	}
	return r
}

// ── FCM HTTP v1 ────────────────────────────────────────────────────────

var errCredential = errors.New("credential rejected")

func (l *Live) fcmToken(ctx context.Context, sa *ServiceAccount) (string, error) {
	return l.cached("fcm:"+sa.ClientEmail, func() (string, time.Time, error) {
		key, err := parseRSAKey(sa.PrivateKey)
		if err != nil {
			return "", time.Time{}, errCredential
		}
		tokenURL := sa.TokenURI
		if l.TokenURL != "" {
			tokenURL = l.TokenURL
		}
		if tokenURL == "" {
			tokenURL = googleTokenURI
		}
		now := l.now()
		assertion, err := signRS256(key, map[string]string{"alg": "RS256", "typ": "JWT", "kid": sa.PrivateKeyID},
			map[string]any{"iss": sa.ClientEmail, "scope": fcmScope, "aud": tokenURL,
				"iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
		if err != nil {
			return "", time.Time{}, err
		}
		form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
		req, _ := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		status, body, _, fail := l.do(ctx, req)
		if fail != nil {
			return "", time.Time{}, fmt.Errorf("token endpoint: %s", fail.Reason)
		}
		if status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden {
			return "", time.Time{}, errCredential
		}
		var tr struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if status != http.StatusOK || json.Unmarshal(body, &tr) != nil || tr.AccessToken == "" {
			return "", time.Time{}, fmt.Errorf("token endpoint: HTTP %d", status)
		}
		life := time.Duration(tr.ExpiresIn) * time.Second
		if life <= 0 {
			life = time.Hour
		}
		// Refresh five minutes before Google says it expires.
		return tr.AccessToken, now.Add(life - 5*time.Minute), nil
	})
}

// fcmMessage is the HTTP v1 message body.
func fcmMessage(token string, n Notification) []byte {
	notif := map[string]any{"title": n.Title}
	if n.Body != nil {
		notif["body"] = *n.Body
	}
	android := map[string]any{"priority": "NORMAL", "ttl": strconv.FormatInt(int64(n.TTL/time.Second), 10) + "s"}
	if n.Priority == "high" {
		android["priority"] = "HIGH"
	}
	if n.CollapseKey != nil {
		android["collapse_key"] = *n.CollapseKey
	}
	an := map[string]any{}
	if n.Sound != nil {
		an["sound"] = *n.Sound
	}
	if n.Badge != nil {
		an["notification_count"] = *n.Badge
	}
	if len(an) > 0 {
		android["notification"] = an
	}
	b, _ := json.Marshal(map[string]any{"message": map[string]any{
		"token": token, "notification": notif, "data": fcmData(n), "android": android}})
	return b
}

func (l *Live) sendFCM(ctx context.Context, m Message) Result {
	sa := m.Credential.ServiceAccount
	if sa == nil {
		return Result{Outcome: CredentialRejected, Reason: "missing_service_account"}
	}
	tok, err := l.fcmToken(ctx, sa)
	if errors.Is(err, errCredential) {
		return Result{Outcome: CredentialRejected, Reason: "invalid_grant"}
	}
	if err != nil {
		return Result{Outcome: Retry, Reason: "token_unavailable"}
	}
	req, _ := http.NewRequest(http.MethodPost, l.FCMURL+"/v1/projects/"+url.PathEscape(sa.ProjectID)+"/messages:send",
		bytes.NewReader(fcmMessage(m.Target.Token, m.Notification)))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	status, body, h, fail := l.do(ctx, req)
	if fail != nil {
		return *fail
	}
	var e struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	code := e.Error.Status
	for _, d := range e.Error.Details {
		if d.ErrorCode != "" {
			code = d.ErrorCode
		}
	}
	r := Result{Status: status, Reason: code, RetryAfter: retryAfter(h)}
	switch {
	case status == http.StatusOK:
		r.Outcome, r.Reason = Sent, ""
	case code == "UNREGISTERED", code == "SENDER_ID_MISMATCH", status == http.StatusNotFound:
		r.Outcome = TokenGone
	case code == "INVALID_ARGUMENT" && strings.Contains(strings.ToLower(e.Error.Message), "registration token"):
		r.Outcome = TokenGone
	case status == http.StatusUnauthorized || code == "THIRD_PARTY_AUTH_ERROR" || code == "PERMISSION_DENIED" || status == http.StatusForbidden:
		l.forget("fcm:" + sa.ClientEmail)
		r.Outcome = CredentialRejected
	case status == http.StatusTooManyRequests || status >= 500:
		r.Outcome = Retry
	default:
		r.Outcome = Failed
	}
	return r
}

// ── Web Push ───────────────────────────────────────────────────────────

// vapidLifetime is how long a VAPID JWT is valid (RFC 8292 allows up to
// 24 hours); it's reused until an hour before it expires.
const vapidLifetime = 12 * time.Hour

func (l *Live) vapidToken(c Credential, audience string) (string, error) {
	return l.cached("vapid:"+c.VAPIDPublicKey+":"+audience, func() (string, time.Time, error) {
		k, err := parseVAPID(c.VAPIDPrivate)
		if err != nil {
			return "", time.Time{}, errCredential
		}
		exp := l.now().Add(vapidLifetime)
		tok, err := signES256(k, map[string]string{"typ": "JWT", "alg": "ES256"},
			map[string]any{"aud": audience, "exp": exp.Unix(), "sub": c.Subject})
		return tok, exp.Add(-time.Hour), err
	})
}

// webPushTopic fits a collapse key into the Topic header's alphabet
// (base64url) and 32-character limit, hashing anything that doesn't.
func webPushTopic(key string) string {
	ok := len(key) <= 32
	for _, r := range key {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			ok = false
		}
	}
	if ok {
		return key
	}
	return b64(sha256Sum([]byte(key)))[:32]
}

func (l *Live) sendWebPush(ctx context.Context, m Message) Result {
	s := m.Target.Subscription
	if s == nil {
		return Result{Outcome: TokenGone, Reason: "missing_subscription"}
	}
	u, err := url.Parse(s.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(l.AllowHTTP && u.Scheme == "http")) {
		return Result{Outcome: TokenGone, Reason: "invalid_endpoint"}
	}
	if l.CheckHost != nil {
		if err := l.CheckHost(ctx, u.Hostname()); err != nil {
			return Result{Outcome: TokenGone, Reason: "endpoint_not_public"}
		}
	}
	uaPublic, err1 := unb64(s.Keys.P256dh)
	auth, err2 := unb64(s.Keys.Auth)
	if err1 != nil || err2 != nil {
		return Result{Outcome: TokenGone, Reason: "invalid_keys"}
	}
	body, err := encryptAES128GCM(WebPushPayload(m.Notification), uaPublic, auth, nil, nil)
	if err != nil {
		return Result{Outcome: TokenGone, Reason: "invalid_keys"}
	}
	tok, err := l.vapidToken(m.Credential, u.Scheme+"://"+u.Host)
	if err != nil {
		return Result{Outcome: CredentialRejected, Reason: "invalid_vapid_key"}
	}
	req, _ := http.NewRequest(http.MethodPost, s.Endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "vapid t="+tok+", k="+m.Credential.VAPIDPublicKey)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", strconv.FormatInt(int64(m.Notification.TTL/time.Second), 10))
	if m.Notification.Priority == "high" {
		req.Header.Set("Urgency", "high")
	} else {
		req.Header.Set("Urgency", "normal")
	}
	if m.Notification.CollapseKey != nil {
		req.Header.Set("Topic", webPushTopic(*m.Notification.CollapseKey))
	}
	status, _, h, fail := l.do(ctx, req)
	if fail != nil {
		return *fail
	}
	r := Result{Status: status, RetryAfter: retryAfter(h)}
	switch {
	case status >= 200 && status < 300:
		r.Outcome = Sent
	case status == http.StatusNotFound || status == http.StatusGone:
		r.Outcome, r.Reason = TokenGone, "subscription_expired"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		r.Outcome, r.Reason = CredentialRejected, "vapid_rejected"
	case status == http.StatusTooManyRequests || status >= 500:
		r.Outcome, r.Reason = Retry, http.StatusText(status)
	default:
		r.Outcome, r.Reason = Failed, http.StatusText(status)
	}
	return r
}
