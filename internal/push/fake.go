package push

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"sync"
)

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

// Recorder is a Sender that records every message instead of sending it,
// for tests and for local development without Apple or Google
// credentials. Every push succeeds unless an outcome is scripted for its
// token (or, for Web Push, its endpoint).
type Recorder struct {
	mu       sync.Mutex
	sent     []Message
	outcomes map[string]Result
}

// Send records m and returns its scripted result, or Sent.
func (r *Recorder) Send(_ context.Context, m Message) Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	if res, ok := r.outcomes[TargetKey(m.Target)]; ok {
		return res
	}
	return Result{Outcome: Sent, Status: 200}
}

// Script makes every push to key (a token or a Web Push endpoint) return res.
func (r *Recorder) Script(key string, res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.outcomes == nil {
		r.outcomes = map[string]Result{}
	}
	r.outcomes[key] = res
}

// Sent returns every recorded message, oldest first.
func (r *Recorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}

// Reset forgets recorded messages and scripted outcomes.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent, r.outcomes = nil, nil
}

// TargetKey is what identifies a device to a provider: its token, or a
// browser subscription's endpoint.
func TargetKey(t Target) string {
	if t.Subscription != nil {
		return t.Subscription.Endpoint
	}
	return t.Token
}

// Log is a Sender that only logs (PUSH_BACKEND=log): every push succeeds,
// and nothing leaves the machine. The token is never logged.
type Log struct{ Logger *slog.Logger }

// Send logs m and reports success.
func (l Log) Send(_ context.Context, m Message) Result {
	if l.Logger != nil {
		l.Logger.Info("push (log backend)", "provider", m.Target.Provider, "notification", m.Notification.ID,
			"category", m.Notification.Category, "title", m.Notification.Title)
	}
	return Result{Outcome: Sent, Status: 200}
}
