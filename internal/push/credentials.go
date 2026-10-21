package push

import (
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// ServiceAccount is the subset of a Google service-account JSON key the
// FCM HTTP v1 OAuth flow needs.
type ServiceAccount struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	TokenURI     string `json:"token_uri"`
}

// googleTokenURI is where service-account assertions are exchanged.
const googleTokenURI = "https://oauth2.googleapis.com/token"

// ParseAPNsKey parses a .p8 key: PEM PKCS#8 holding a P-256 ECDSA key.
func ParseAPNsKey(pemText string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("not a PEM block")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("not a PKCS#8 private key: %w", err)
	}
	ec, ok := k.(*ecdsa.PrivateKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, errors.New("not a P-256 ECDSA key")
	}
	return ec, nil
}

// ParseServiceAccount parses and checks a service-account JSON key.
func ParseServiceAccount(raw []byte) (*ServiceAccount, error) {
	var sa ServiceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, errors.New("not a JSON object")
	}
	var missing []string
	for name, v := range map[string]string{"type": sa.Type, "project_id": sa.ProjectID,
		"private_key": sa.PrivateKey, "client_email": sa.ClientEmail} {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("missing %s", strings.Join(missing, ", "))
	}
	if sa.Type != "service_account" {
		return nil, errors.New(`type must be "service_account"`)
	}
	if _, err := parseRSAKey(sa.PrivateKey); err != nil {
		return nil, fmt.Errorf("private_key: %w", err)
	}
	if sa.TokenURI == "" {
		sa.TokenURI = googleTokenURI
	}
	return &sa, nil
}

func parseRSAKey(pemText string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("not a PEM block")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("not an RSA key")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// GenerateVAPID returns a fresh VAPID key pair: the public key as base64url
// of the uncompressed P-256 point (what PushManager.subscribe takes as
// applicationServerKey), and the private scalar as base64url.
func GenerateVAPID() (public, private string, err error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	pub, err := k.PublicKey.Bytes()
	if err != nil {
		return "", "", err
	}
	priv, err := k.Bytes()
	if err != nil {
		return "", "", err
	}
	return b64(pub), b64(priv), nil
}

func parseVAPID(private string) (*ecdsa.PrivateKey, error) {
	raw, err := unb64(private)
	if err != nil {
		return nil, err
	}
	return ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
}

// ── JWTs ───────────────────────────────────────────────────────────────

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// unb64 accepts base64url with or without padding (browsers and libraries
// differ), and standard base64 as a courtesy.
func unb64(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func jwtSigningInput(header, claims any) (string, error) {
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return b64(h) + "." + b64(c), nil
}

// signES256 returns a JWT signed with ES256: the signature is r‖s, each
// left-padded to 32 bytes (RFC 7518 §3.4), not ASN.1 DER.
func signES256(k *ecdsa.PrivateKey, header, claims any) (string, error) {
	in, err := jwtSigningInput(header, claims)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(in))
	r, s, err := ecdsa.Sign(rand.Reader, k, sum[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return in + "." + b64(sig), nil
}

// verifyES256 checks an ES256 JWT's signature (tests and fakes).
func verifyES256(pub *ecdsa.PublicKey, token string) bool {
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		return false
	}
	sig, err := unb64(token[i+1:])
	if err != nil || len(sig) != 64 {
		return false
	}
	sum := sha256.Sum256([]byte(token[:i]))
	return ecdsa.Verify(pub, sum[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]))
}

func signRS256(k *rsa.PrivateKey, header, claims any) (string, error) {
	in, err := jwtSigningInput(header, claims)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(in))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return in + "." + b64(sig), nil
}

// ValidateSubscriptionKeys checks a browser subscription's keys: p256dh
// must be an uncompressed P-256 point and auth 16 bytes, both base64url.
func ValidateSubscriptionKeys(p256dh, auth string) error {
	pub, err := unb64(p256dh)
	if err != nil {
		return errors.New("p256dh isn't base64url")
	}
	if _, err := ecdh.P256().NewPublicKey(pub); err != nil {
		return errors.New("p256dh isn't an uncompressed P-256 public key")
	}
	a, err := unb64(auth)
	if err != nil || len(a) != 16 {
		return errors.New("auth must be 16 bytes of base64url")
	}
	return nil
}
