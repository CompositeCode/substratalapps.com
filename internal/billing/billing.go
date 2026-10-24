// Package billing is the Stripe side of the Free/Standard subscription
// (infra/README.md → Stripe Billing): Customers for subscribing Users and
// Organizations, Standard Checkout, the Customer Portal, the inbound
// webhook, subscription sync, and the daily Organization quantity sync.
// Third-party developers' own billing never touches it.
package billing

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v87"

	"github.com/CompositeCode/substratalapps.com/internal/core"
	"github.com/CompositeCode/substratalapps.com/internal/platform/config"
)

// LookupStandard is the Standard price's lookup key (Pricing → Stripe
// catalog). Code resolves the price by lookup key, never by hard-coded
// price id, so test and live mode share code.
const LookupStandard = "standard_monthly_usd"

// Client implements core.Billing over stripe-go.
type Client struct {
	sc     *stripe.Client
	prices config.StripePrices
	live   bool
	log    *slog.Logger
}

var _ core.Billing = (*Client)(nil)

// New returns a Stripe-backed Client.
func New(secretKey string, prices config.StripePrices, log *slog.Logger) *Client {
	live := strings.HasPrefix(secretKey, "sk_live_") || strings.HasPrefix(secretKey, "rk_live_")
	return &Client{sc: stripe.NewClient(secretKey), prices: prices, live: live, log: log}
}

// LiveMode reports whether the key is a live-mode key.
func (c *Client) LiveMode() bool { return c.live }

// metadata identifies the subscriber on every Stripe object.
func metadata(who core.BillingSubscriber) map[string]string {
	return map[string]string{"subscriber_type": who.Type, "subscriber_id": who.ID}
}

// EnsureCustomer creates the subscriber's Customer. The idempotency key
// makes a retried request safe to repeat.
func (c *Client) EnsureCustomer(ctx context.Context, who core.BillingSubscriber, email string) (string, error) {
	p := &stripe.CustomerCreateParams{Metadata: metadata(who)}
	if email != "" {
		p.Email = stripe.String(email)
	}
	p.SetIdempotencyKey("customer:" + who.ID)
	cust, err := c.sc.V1Customers.Create(ctx, p)
	if err != nil {
		return "", err
	}
	return cust.ID, nil
}

// priceID resolves the Standard price: a configured id wins, otherwise the
// lookup key.
func (c *Client) priceID(ctx context.Context) (string, error) {
	if c.prices.Standard != "" {
		return c.prices.Standard, nil
	}
	lp := &stripe.PriceListParams{LookupKeys: []*string{stripe.String(LookupStandard)}}
	for p, err := range c.sc.V1Prices.List(ctx, lp).All(ctx) {
		if err != nil {
			return "", err
		}
		if p.LookupKey == LookupStandard {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("billing: Standard price not found; run the catalog sync script")
}

// CheckoutSession starts a Standard subscription in Stripe Checkout.
func (c *Client) CheckoutSession(ctx context.Context, in core.CheckoutInput) (string, time.Time, error) {
	price, err := c.priceID(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	meta := metadata(in.Subscriber)
	meta["subscription_id"] = in.SubscriptionID
	p := &stripe.CheckoutSessionCreateParams{
		Mode:                     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		Customer:                 stripe.String(in.CustomerID),
		ClientReferenceID:        stripe.String(in.SubscriptionID),
		SuccessURL:               stripe.String(in.SuccessURL),
		CancelURL:                stripe.String(in.CancelURL),
		BillingAddressCollection: stripe.String("required"),
		AutomaticTax:             &stripe.CheckoutSessionCreateAutomaticTaxParams{Enabled: stripe.Bool(true)},
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{Price: stripe.String(price), Quantity: stripe.Int64(int64(max(in.Quantity, 1)))},
		},
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{Metadata: meta},
	}
	sess, err := c.sc.V1CheckoutSessions.Create(ctx, p)
	if err != nil {
		return "", time.Time{}, err
	}
	return sess.URL, time.Unix(sess.ExpiresAt, 0), nil
}

// PortalSession opens the Customer Portal (configured in Stripe without
// plan switching).
// OpenCheckout reports whether the Customer still has an open Checkout for
// our subscription id, so an older session expiring doesn't mark the row
// abandoned while a newer one can still complete.
func (c *Client) OpenCheckout(ctx context.Context, customerID, subscriptionID string) (bool, error) {
	p := &stripe.CheckoutSessionListParams{Customer: stripe.String(customerID), Status: stripe.String("open")}
	for s, err := range c.sc.V1CheckoutSessions.List(ctx, p).All(ctx) {
		if err != nil {
			return false, err
		}
		if s.ClientReferenceID == subscriptionID {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) PortalSession(ctx context.Context, customerID, returnURL string) (string, time.Time, error) {
	sess, err := c.sc.V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{
		Customer: stripe.String(customerID), ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	// Portal sessions are short-lived; Stripe doesn't return an expiry, so
	// report the conservative five minutes the dashboard should assume.
	return sess.URL, time.Unix(sess.Created, 0).Add(5 * time.Minute), nil
}

// Subscription fetches a subscription's current state, the only source
// the sync trusts (out-of-order webhooks are harmless that way).
func (c *Client) Subscription(ctx context.Context, id string) (core.StripeSubscription, error) {
	p := &stripe.SubscriptionRetrieveParams{}
	p.AddExpand("items.data.price.product")
	s, err := c.sc.V1Subscriptions.Retrieve(ctx, id, p)
	if err != nil {
		return core.StripeSubscription{}, err
	}
	out := core.StripeSubscription{ID: s.ID, Status: string(s.Status), CancelAtPeriodEnd: s.CancelAtPeriodEnd,
		SubscriptionID: s.Metadata["subscription_id"],
		Subscriber:     core.BillingSubscriber{Type: s.Metadata["subscriber_type"], ID: s.Metadata["subscriber_id"]}}
	if s.Items != nil {
		for _, it := range s.Items.Data {
			if it.CurrentPeriodStart > 0 {
				ps, pe := time.Unix(it.CurrentPeriodStart, 0).UTC(), time.Unix(it.CurrentPeriodEnd, 0).UTC()
				out.PeriodStart, out.PeriodEnd = &ps, &pe
			}
			if it.Price == nil {
				continue
			}
			if it.Price.LookupKey == LookupStandard || (c.prices.Standard != "" && it.Price.ID == c.prices.Standard) {
				out.ItemID, out.Quantity = it.ID, int(it.Quantity)
			}
			if it.Price.Product != nil {
				if plan := it.Price.Product.Metadata["substratal_plan"]; plan != "" {
					out.Plan = plan
				}
			}
		}
	}
	return out, nil
}

// SetQuantity writes an Organization subscription's covered-member count,
// once per subscription per day, with no proration (Pricing → Stripe catalog).
func (c *Client) SetQuantity(ctx context.Context, subscriptionID, itemID string, quantity int, day string) error {
	p := &stripe.SubscriptionItemUpdateParams{Quantity: stripe.Int64(int64(quantity)),
		ProrationBehavior: stripe.String("none")}
	p.SetIdempotencyKey("quantity-sync:" + subscriptionID + ":" + day + ":" + strconv.Itoa(quantity))
	_, err := c.sc.V1SubscriptionItems.Update(ctx, itemID, p)
	return err
}

// Event re-fetches an event from Stripe, for reprocessing one whose first
// attempt failed.
func (c *Client) Event(ctx context.Context, id string) (stripe.Event, error) {
	ev, err := c.sc.V1Events.Retrieve(ctx, id, nil)
	if err != nil {
		return stripe.Event{}, err
	}
	return *ev, nil
}
