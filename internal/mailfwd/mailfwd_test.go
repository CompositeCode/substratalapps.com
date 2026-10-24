package mailfwd

import (
	"net/mail"
	"strings"
	"testing"
)

var fwd = mail.Address{Address: "forward@mail.example.com"}

func header(t *testing.T, msg []byte) mail.Header {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(string(msg)))
	if err != nil {
		t.Fatalf("forward doesn't parse: %v\n%s", err, msg)
	}
	return m.Header
}

func TestRewrite(t *testing.T) {
	raw := "Return-Path: <bounce@sender.example>\r\n" +
		"DKIM-Signature: v=1; a=rsa-sha256; d=sender.example;\r\n\tb=abc123\r\n" +
		"From: Pat Doe <pat@sender.example>\r\n" +
		"To: privacy@example.com\r\n" +
		"Subject: Data request\r\n" +
		"Message-ID: <1@sender.example>\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Please delete my data.\r\n\r\nThanks\r\n"
	out, err := Rewrite([]byte(raw), fwd)
	if err != nil {
		t.Fatal(err)
	}
	h := header(t, out)
	if got := h.Get("From"); got != `"Pat Doe via Substratal" <forward@mail.example.com>` {
		t.Fatalf("From = %q", got)
	}
	if got := h.Get("Reply-To"); got != "Pat Doe <pat@sender.example>" {
		t.Fatalf("Reply-To = %q", got)
	}
	for _, k := range []string{"DKIM-Signature", "Return-Path", "Message-ID"} {
		if h.Get(k) != "" {
			t.Fatalf("%s carried over", k)
		}
	}
	if h.Get("Subject") != "Data request" || h.Get("To") != "privacy@example.com" {
		t.Fatalf("kept headers lost: %v", h)
	}
	if !strings.HasSuffix(string(out), "\r\n\r\nPlease delete my data.\r\n\r\nThanks\r\n") {
		t.Fatalf("body changed:\n%q", out)
	}
}

func TestRewriteKeepsOriginalReplyTo(t *testing.T) {
	raw := "From: list@sender.example\nReply-To: Pat <pat@sender.example>\nSubject: x\n\nbody\n"
	out, err := Rewrite([]byte(raw), fwd)
	if err != nil {
		t.Fatal(err)
	}
	h := header(t, out)
	if h.Get("Reply-To") != "Pat <pat@sender.example>" {
		t.Fatalf("Reply-To = %q", h.Get("Reply-To"))
	}
	if h.Get("From") != `"list@sender.example via Substratal" <forward@mail.example.com>` {
		t.Fatalf("From = %q", h.Get("From"))
	}
	if strings.Contains(string(out), "\r\n") {
		t.Fatalf("line endings changed: %q", out)
	}
}

func TestRewriteNoHeaders(t *testing.T) {
	if _, err := Rewrite([]byte("just text"), fwd); err == nil {
		t.Fatal("want an error for a message with no header block")
	}
}
