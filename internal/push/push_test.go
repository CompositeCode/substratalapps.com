package push

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := unb64(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// RFC 8291 Appendix A: the worked example, byte for byte.
func TestEncryptRFC8291Vector(t *testing.T) {
	asPriv, err := ecdh.P256().NewPrivateKey(mustB64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := encryptAES128GCM([]byte("When I grow up, I want to be a watermelon"),
		mustB64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"),
		mustB64(t, "BTBZMqHH6r4Tts7J_aSIgg"), asPriv, mustB64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if b64(got) != want {
		t.Fatalf("ciphertext mismatch:\n got %s\nwant %s", b64(got), want)
	}
	uaPriv, _ := ecdh.P256().NewPrivateKey(mustB64(t, "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"))
	plain, err := decryptAES128GCM(got, uaPriv, mustB64(t, "BTBZMqHH6r4Tts7J_aSIgg"))
	if err != nil || string(plain) != "When I grow up, I want to be a watermelon" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	msg := []byte(`{"title":"hello"}`)
	body, err := encryptAES128GCM(msg, ua.PublicKey().Bytes(), auth, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != len(msg)+webPushOverhead {
		t.Fatalf("overhead = %d, want %d", len(body)-len(msg), webPushOverhead)
	}
	plain, err := decryptAES128GCM(body, ua, auth)
	if err != nil || string(plain) != string(msg) {
		t.Fatalf("round trip = %q, %v", plain, err)
	}
}

func p8(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), k
}

func TestParseCredentials(t *testing.T) {
	pemText, _ := p8(t)
	if _, err := ParseAPNsKey(pemText); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAPNsKey("not a key"); err == nil {
		t.Fatal("garbage parsed as an APNs key")
	}
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(rk)
	rsaPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	if _, err := ParseAPNsKey(rsaPEM); err == nil {
		t.Fatal("an RSA key parsed as an APNs key")
	}
	good, _ := json.Marshal(map[string]string{"type": "service_account", "project_id": "p", "private_key": rsaPEM, "client_email": "a@p.iam.gserviceaccount.com"})
	sa, err := ParseServiceAccount(good)
	if err != nil || sa.TokenURI != googleTokenURI {
		t.Fatalf("service account = %+v, %v", sa, err)
	}
	if _, err := ParseServiceAccount([]byte(`{"type":"service_account"}`)); err == nil ||
		!strings.Contains(err.Error(), "client_email, private_key, project_id") {
		t.Fatalf("missing fields err = %v", err)
	}
	pub, priv, err := GenerateVAPID()
	if err != nil || len(mustB64(t, pub)) != 65 || len(mustB64(t, priv)) != 32 {
		t.Fatalf("vapid = %q %q %v", pub, priv, err)
	}
	if _, err := parseVAPID(priv); err != nil {
		t.Fatal(err)
	}
}

func note() Notification {
	app := "app_timetrack"
	body, key, badge, sound := "Due Friday.", "timesheet-due", 1, "default"
	return Notification{ID: "ntf_1", ApplicationID: &app, Source: "application", Category: "timesheet.reminder",
		Priority: "high", Title: "Timesheet due", Body: &body, Data: map[string]string{"timesheet_id": "ts_1"},
		Badge: &badge, Sound: &sound, CollapseKey: &key, TTL: time.Hour}
}

// An APNs fake over HTTP/2 + TLS, checking what Apple checks.
func TestAPNs(t *testing.T) {
	pemText, key := p8(t)
	var mu sync.Mutex
	var got *http.Request
	var gotBody []byte
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got, gotBody = r, b
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/gone"):
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
		case strings.HasSuffix(r.URL.Path, "/badkey"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"reason":"InvalidProviderToken"}`))
		case strings.HasSuffix(r.URL.Path, "/busy"):
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"reason":"TooManyRequests"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	l := NewLive()
	l.Client = srv.Client()
	l.APNsProductionURL, l.APNsSandboxURL = srv.URL, srv.URL+"/sandbox"
	cred := Credential{Provider: APNs, TeamID: "ABCDE12345", KeyID: "XYZ987WVU6", Topic: "com.example.app", PrivateKey: pemText}
	send := func(token, env string) Result {
		return l.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: APNs, Token: token, Environment: env}, Notification: note()})
	}

	if r := send("abc123", "production"); r.Outcome != Sent {
		t.Fatalf("send = %+v", r)
	}
	mu.Lock()
	if got.ProtoMajor != 2 {
		t.Errorf("proto = %s, APNs requires HTTP/2", got.Proto)
	}
	if got.URL.Path != "/3/device/abc123" || got.Header.Get("apns-topic") != "com.example.app" ||
		got.Header.Get("apns-priority") != "10" || got.Header.Get("apns-push-type") != "alert" ||
		got.Header.Get("apns-collapse-id") != "timesheet-due" || got.Header.Get("apns-expiration") == "0" {
		t.Errorf("headers = %v %v", got.URL.Path, got.Header)
	}
	tok := strings.TrimPrefix(got.Header.Get("Authorization"), "bearer ")
	if !verifyES256(&key.PublicKey, tok) {
		t.Error("provider JWT doesn't verify against the .p8 key")
	}
	var hdr map[string]string
	_ = json.Unmarshal(mustB64(t, strings.Split(tok, ".")[0]), &hdr)
	if hdr["kid"] != "XYZ987WVU6" || hdr["alg"] != "ES256" {
		t.Errorf("jwt header = %v", hdr)
	}
	var payload map[string]any
	_ = json.Unmarshal(gotBody, &payload)
	aps := payload["aps"].(map[string]any)
	if aps["alert"].(map[string]any)["title"] != "Timesheet due" || aps["interruption-level"] != "time-sensitive" ||
		payload["timesheet_id"] != "ts_1" || payload["substratal"].(map[string]any)["notification_id"] != "ntf_1" {
		t.Errorf("payload = %s", gotBody)
	}
	mu.Unlock()

	// The provider token is cached, not re-signed per push.
	send("abc123", "production")
	mu.Lock()
	if strings.TrimPrefix(got.Header.Get("Authorization"), "bearer ") != tok {
		t.Error("provider JWT wasn't reused")
	}
	mu.Unlock()

	send("abc123", "sandbox")
	mu.Lock()
	if !strings.HasPrefix(got.URL.Path, "/sandbox/3/device/") {
		t.Errorf("sandbox device went to %s", got.URL.Path)
	}
	mu.Unlock()
	if r := send("gone", "production"); r.Outcome != TokenGone || r.Reason != "Unregistered" {
		t.Errorf("410 = %+v", r)
	}
	if r := send("badkey", "production"); r.Outcome != CredentialRejected {
		t.Errorf("403 = %+v", r)
	}
	if r := send("busy", "production"); r.Outcome != Retry || r.RetryAfter != 7*time.Second {
		t.Errorf("429 = %+v", r)
	}
}

func TestFCM(t *testing.T) {
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(rk)
	saJSON, _ := json.Marshal(map[string]string{"type": "service_account", "project_id": "proj-1", "private_key_id": "kid1",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email": "sender@proj-1.iam.gserviceaccount.com"})
	sa, err := ParseServiceAccount(saJSON)
	if err != nil {
		t.Fatal(err)
	}
	tokenCalls := 0
	var msg map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			tokenCalls++
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || strings.Count(r.Form.Get("assertion"), ".") != 2 {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"ya29.test","expires_in":3600}`))
		case r.URL.Path == "/v1/projects/proj-1/messages:send":
			if r.Header.Get("Authorization") != "Bearer ya29.test" {
				w.WriteHeader(401)
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			msg = body["message"].(map[string]any)
			if msg["token"] == "dead" {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"error":{"code":404,"status":"NOT_FOUND","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"name":"projects/proj-1/messages/1"}`))
		}
	}))
	defer srv.Close()
	l := NewLive()
	l.FCMURL, l.TokenURL = srv.URL, srv.URL+"/token"
	cred := Credential{Provider: FCM, ServiceAccount: sa}
	for i := 0; i < 2; i++ {
		if r := l.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: FCM, Token: "tok-1"}, Notification: note()}); r.Outcome != Sent {
			t.Fatalf("send = %+v", r)
		}
	}
	if tokenCalls != 1 {
		t.Errorf("token endpoint called %d times; the access token should be cached", tokenCalls)
	}
	data := msg["data"].(map[string]any)
	android := msg["android"].(map[string]any)
	if data["timesheet_id"] != "ts_1" || !strings.Contains(data["substratal"].(string), "ntf_1") ||
		android["priority"] != "HIGH" || android["ttl"] != "3600s" || android["collapse_key"] != "timesheet-due" {
		t.Errorf("message = %v", msg)
	}
	if r := l.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: FCM, Token: "dead"}, Notification: note()}); r.Outcome != TokenGone || r.Reason != "UNREGISTERED" {
		t.Errorf("unregistered = %+v", r)
	}
	// A rejected assertion is the credential's fault.
	l2 := NewLive()
	l2.FCMURL, l2.TokenURL = srv.URL, srv.URL+"/token?x=1"
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	})
	if r := l2.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: FCM, Token: "tok-1"}, Notification: note()}); r.Outcome != CredentialRejected {
		t.Errorf("invalid_grant = %+v", r)
	}
}

// A browser push service fake that decrypts what it receives, so the test
// proves a real user agent could read it.
func TestWebPush(t *testing.T) {
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	pub, priv, _ := GenerateVAPID()
	vapidPub, _ := unb64(pub)
	vk, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), vapidPub)
	if err != nil {
		t.Fatal(err)
	}
	var plain []byte
	var hdr http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/expired") {
			w.WriteHeader(http.StatusGone)
			return
		}
		hdr = r.Header
		body, _ := io.ReadAll(r.Body)
		var err error
		plain, err = decryptAES128GCM(body, ua, auth)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	l := NewLive()
	l.AllowHTTP = true
	sub := &Subscription{Endpoint: srv.URL + "/push/abc"}
	sub.Keys.P256dh, sub.Keys.Auth = b64(ua.PublicKey().Bytes()), b64(auth)
	cred := Credential{Provider: WebPush, Subject: "mailto:ops@example.com", VAPIDPublicKey: pub, VAPIDPrivate: priv}
	r := l.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: WebPush, Subscription: sub}, Notification: note()})
	if r.Outcome != Sent || r.Status != 201 {
		t.Fatalf("send = %+v", r)
	}
	var payload map[string]any
	if err := json.Unmarshal(plain, &payload); err != nil || payload["title"] != "Timesheet due" {
		t.Fatalf("decrypted payload = %s", plain)
	}
	if hdr.Get("Content-Encoding") != "aes128gcm" || hdr.Get("Urgency") != "high" || hdr.Get("TTL") != "3600" ||
		hdr.Get("Topic") != "timesheet-due" {
		t.Errorf("headers = %v", hdr)
	}
	authz := hdr.Get("Authorization")
	if !strings.HasPrefix(authz, "vapid t=") || !strings.HasSuffix(authz, ", k="+pub) {
		t.Fatalf("authorization = %q", authz)
	}
	jwt := strings.TrimSuffix(strings.TrimPrefix(authz, "vapid t="), ", k="+pub)
	if !verifyES256(vk, jwt) {
		t.Error("VAPID JWT doesn't verify against the public key")
	}
	var claims map[string]any
	_ = json.Unmarshal(mustB64(t, strings.Split(jwt, ".")[1]), &claims)
	su, _ := url.Parse(srv.URL)
	if claims["aud"] != "http://"+su.Host || claims["sub"] != "mailto:ops@example.com" {
		t.Errorf("claims = %v", claims)
	}
	sub.Endpoint = srv.URL + "/push/expired"
	if r := l.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: WebPush, Subscription: sub}, Notification: note()}); r.Outcome != TokenGone {
		t.Errorf("410 = %+v", r)
	}
	// Plain http is refused unless explicitly allowed.
	l.AllowHTTP = false
	if r := l.Send(context.Background(), Message{Credential: cred, Target: Target{Provider: WebPush, Subscription: sub}, Notification: note()}); r.Outcome != TokenGone {
		t.Errorf("http endpoint = %+v", r)
	}
}

func TestTopicAndSize(t *testing.T) {
	if webPushTopic("short_key-1") != "short_key-1" {
		t.Error("a safe key should pass through")
	}
	if got := webPushTopic("timesheet.due.2026-10-12"); len(got) != 32 || strings.Contains(got, ".") {
		t.Errorf("hashed topic = %q", got)
	}
	n := note()
	if s := Size(n); s <= len(APNsPayload(n)) && s <= len(WebPushPayload(n)) {
		t.Errorf("Size = %d", s)
	}
	big := strings.Repeat("x", 4000)
	n.Body = &big
	if Size(n) <= MaxPayload {
		t.Error("a 4000-character body must exceed the budget")
	}
}
