// Package mailfwd rewrites an inbound message so SES can forward it.
//
// SES only sends from a verified identity, and the original sender's
// DKIM signature breaks once the message is re-sent, so a forward goes
// out From a Substratal address with the original sender in Reply-To.
// The body is passed through byte for byte.
package mailfwd

import (
	"bytes"
	"errors"
	"net/mail"
	"strings"
)

// dropped are the headers a forward must not carry over: the sender
// identity it replaces, and the signatures and envelope fields that no
// longer hold for the re-sent message.
var dropped = map[string]bool{
	"from":           true,
	"reply-to":       true,
	"sender":         true,
	"return-path":    true,
	"dkim-signature": true,
	"message-id":     true,
}

// Rewrite returns raw with its sender replaced by from, and the original
// sender (or the original Reply-To, when there was one) in Reply-To.
func Rewrite(raw []byte, from mail.Address) ([]byte, error) {
	head, body, sep := split(raw)
	if head == nil {
		return nil, errors.New("mailfwd: no header block")
	}
	nl := "\n"
	if bytes.Contains(head, []byte("\r\n")) {
		nl = "\r\n"
	}
	var kept []string
	var origFrom, origReplyTo string
	for _, field := range fields(string(head)) {
		name, value, _ := strings.Cut(field, ":")
		key := strings.ToLower(strings.TrimSpace(name))
		switch key {
		case "from":
			origFrom = unfold(value)
		case "reply-to":
			origReplyTo = unfold(value)
		}
		if !dropped[key] {
			kept = append(kept, field)
		}
	}
	replyTo := origReplyTo
	if replyTo == "" {
		replyTo = origFrom
	}
	label := "Unknown sender"
	if a, err := mail.ParseAddress(origFrom); err == nil {
		label = a.Address
		if a.Name != "" {
			label = a.Name
		}
	} else if origFrom != "" {
		label = origFrom
	}
	sender := mail.Address{Name: label + " via Substratal", Address: from.Address}

	var out bytes.Buffer
	out.WriteString("From: " + sender.String() + nl)
	if replyTo != "" {
		out.WriteString("Reply-To: " + replyTo + nl)
		out.WriteString("X-Original-From: " + origFrom + nl)
	}
	for _, f := range kept {
		out.WriteString(strings.ReplaceAll(f, "\n", nl) + nl)
	}
	out.WriteString(sep)
	out.Write(body)
	return out.Bytes(), nil
}

// split separates the header block from the body at the first blank line.
func split(raw []byte) (head, body []byte, sep string) {
	for _, s := range []string{"\r\n\r\n", "\n\n"} {
		if i := bytes.Index(raw, []byte(s)); i >= 0 {
			// The separator's first line break ends the last header line,
			// which Rewrite writes itself; keep only the blank line.
			return raw[:i], raw[i+len(s):], s[len(s)/2:]
		}
	}
	return nil, nil, ""
}

// fields groups header lines with their folded continuations, joined by
// "\n".
func fields(head string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(head, "\r\n", "\n"), "\n") {
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(out) > 0 {
			out[len(out)-1] += "\n" + line
			continue
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func unfold(v string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(v), " "))
}
