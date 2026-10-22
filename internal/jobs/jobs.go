// Package jobs is every scheduled job (infra/README.md → Build checklist,
// step 6), one entry point each. EventBridge Scheduler invokes the jobs
// Lambda with {"job": "<name>"}; locally, `make job name=<name>` runs one.
package jobs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/CompositeCode/substratalapps.com/internal/billing"
	"github.com/CompositeCode/substratalapps.com/internal/core"
	"github.com/CompositeCode/substratalapps.com/internal/platform/wire"
)

// Runner holds what the jobs need.
type Runner struct {
	Base   *wire.Base
	Core   *core.Server
	Stripe *billing.Client // nil without Stripe configuration
}

// New builds a Runner.
func New(base *wire.Base) *Runner {
	r := &Runner{Base: base, Core: base.Server(nil)}
	if base.Config.StripeSecretKey != "" {
		r.Stripe = billing.New(base.Config.StripeSecretKey, base.Config.StripePrices, base.Log)
	}
	return r
}

// Jobs maps a job name to its function.
func (r *Runner) Jobs() map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		// every 5 minutes
		"entitlement-sweep": func(ctx context.Context) error {
			exp, started, err := r.Core.SweepEntitlements(ctx)
			r.Base.Log.Info("entitlement sweep", "expired", exp, "started", started)
			if err != nil {
				return err
			}
			users, err := r.Core.ExpireTestUsers(ctx)
			r.Base.Log.Info("test user expiry", "expired", users)
			return err
		},
		// every minute: the safety net under SQS nudges, for webhooks and
		// push deliveries alike (push events can produce webhooks, so
		// pushes go first)
		"webhook-dispatch": func(ctx context.Context) error {
			if err := r.Core.RunPush(ctx); err != nil {
				return err
			}
			return r.Base.Deliverer().Run(ctx)
		},
		// daily: stale devices, inactive-device deletion, inbox retention
		"push-maintenance": func(ctx context.Context) error { return r.Core.PushMaintenance(ctx) },
		// hourly
		"erasure-cascade": func(ctx context.Context) error {
			n, err := r.Core.RunErasures(ctx)
			r.Base.Log.Info("erasure cascade", "completed", n)
			return err
		},
		"webhook-health": func(ctx context.Context) error {
			n, err := r.Base.Deliverer().DisableLongUnhealthy(ctx)
			r.Base.Log.Info("webhook health", "disabled", n)
			return err
		},
		// the safety net under inline reconciles: free and subscription
		// Entitlements match access tiers and subscriptions
		"subscription-access": func(ctx context.Context) error { return r.Core.ReconcileAllAccess(ctx) },
		// every 10 minutes: events whose first processing failed
		"stripe-events": r.stripeEvents,
		// daily 00:15 UTC
		"seat-sync": r.seatSync,
		// nightly
		"cleanup": func(ctx context.Context) error { return r.Core.Cleanup(ctx) },
		"audit-archive": func(ctx context.Context) error {
			n, err := r.Core.ArchiveAudit(ctx, r.archiver())
			r.Base.Log.Info("audit archive", "archived", n)
			return err
		},
		"settings-projections": func(ctx context.Context) error { return r.Core.RebuildAllProjections(ctx) },
	}
}

// Names lists the jobs, sorted.
func (r *Runner) Names() []string {
	var out []string
	for k := range r.Jobs() {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Run runs one job by name.
func (r *Runner) Run(ctx context.Context, name string) error {
	f, ok := r.Jobs()[name]
	if !ok {
		return fmt.Errorf("jobs: unknown job %q (known: %v)", name, r.Names())
	}
	start := time.Now()
	err := f(ctx)
	r.Base.Log.Info("job finished", "job", name, "duration_ms", time.Since(start).Milliseconds(), "error", errString(err))
	return err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// stripeEvents reprocesses recorded events that failed the first time, by
// re-fetching each from Stripe (processing is idempotent: it always reads
// the Subscription's current state).
func (r *Runner) stripeEvents(ctx context.Context) error {
	if r.Stripe == nil {
		return nil
	}
	ids, err := r.Core.UnprocessedStripeEvents(ctx)
	if err != nil {
		return err
	}
	wh := &billing.Webhook{Stripe: r.Stripe, Core: r.Core, Log: r.Base.Log}
	for _, id := range ids {
		ev, err := r.Stripe.Event(ctx, id)
		if err != nil {
			r.Base.Log.Error("stripe events: fetch", "event", id, "err", err)
			continue
		}
		if err := wh.Process(ctx, ev); err != nil {
			r.Base.Log.Error("stripe events: process", "event", id, "err", err)
		}
	}
	return nil
}

// seatSync writes each Organization subscription's covered-member count
// to Stripe once a day, with no proration (Pricing → Stripe catalog).
// Individual subscriptions are always quantity 1.
func (r *Runner) seatSync(ctx context.Context) error {
	if r.Stripe == nil {
		return nil
	}
	targets, err := r.Core.OrganizationQuantities(ctx)
	if err != nil {
		return err
	}
	day := time.Now().UTC().Format("2006-01-02")
	for _, t := range targets {
		if t.LastSynced != nil && *t.LastSynced == t.Quantity {
			continue
		}
		sub, err := r.Stripe.Subscription(ctx, t.StripeSubscriptionID)
		if err != nil {
			r.Base.Log.Error("seat sync: read subscription", "subscription", t.SubscriptionID, "err", err)
			continue
		}
		if sub.ItemID == "" {
			continue
		}
		if err := r.Stripe.SetQuantity(ctx, t.SubscriptionID, sub.ItemID, t.Quantity, day); err != nil {
			r.Base.Log.Error("seat sync: write", "subscription", t.SubscriptionID, "err", err)
			continue
		}
		if err := r.Core.RecordQuantitySync(ctx, t.SubscriptionID, t.Quantity); err != nil {
			return err
		}
	}
	return nil
}

// archiver writes shape-only audit batches as gzipped JSON lines: to S3
// (Glacier Deep Archive storage class) when AUDIT_ARCHIVE_BUCKET is set,
// otherwise to ./.dev/audit-archive.
func (r *Runner) archiver() core.Archiver {
	bucket := os.Getenv("AUDIT_ARCHIVE_BUCKET")
	return func(ctx context.Context, batch []core.ArchivedEvent) error {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		enc := json.NewEncoder(zw)
		for _, e := range batch {
			if err := enc.Encode(e); err != nil {
				return err
			}
		}
		if err := zw.Close(); err != nil {
			return err
		}
		first := batch[0].Timestamp.UTC()
		key := fmt.Sprintf("audit/%s/%s-%s.jsonl.gz", first.Format("2006/01/02"), batch[0].ID, batch[len(batch)-1].ID)
		if bucket == "" {
			path := filepath.Join(".dev", "audit-archive", key)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, buf.Bytes(), 0o644)
		}
		awsCfg, err := wire.AWS(ctx)
		if err != nil {
			return err
		}
		_, err = s3.NewFromConfig(awsCfg).PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket, Key: &key, Body: bytes.NewReader(buf.Bytes()),
			StorageClass: s3types.StorageClassDeepArchive, ContentType: strp("application/gzip"),
		})
		return err
	}
}

func strp(s string) *string { return &s }
