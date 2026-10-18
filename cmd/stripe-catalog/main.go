// Command stripe-catalog creates the Stripe Product and Price in
// Pricing → Stripe catalog, once per Stripe mode. It's idempotent: a Price
// whose lookup_key already exists is left alone, and the Product uses a
// fixed id, so it's safe to re-run. Point it at test mode first.
//
//	STRIPE_SECRET_KEY=sk_test_... go run ./cmd/stripe-catalog
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/stripe/stripe-go/v87"
)

const (
	productID = "substratal_standard"
	lookupKey = "standard_monthly_usd"
	// Standard is $6 per user per month (Pricing → The tiers).
	unitAmount = 600
)

func main() {
	key := os.Getenv("STRIPE_SECRET_KEY")
	if key == "" {
		log.Fatal("stripe-catalog: STRIPE_SECRET_KEY is required")
	}
	if strings.HasPrefix(key, "sk_live_") && os.Getenv("CONFIRM_LIVE") != "yes" {
		log.Fatal("stripe-catalog: refusing live mode without CONFIRM_LIVE=yes")
	}
	sc := stripe.NewClient(key)
	ctx := context.Background()

	p := &stripe.ProductCreateParams{ID: stripe.String(productID), Name: stripe.String("Substratal Standard"),
		Metadata: map[string]string{"substratal_plan": "standard"}}
	if _, err := sc.V1Products.Create(ctx, p); err != nil && !strings.Contains(err.Error(), "already exists") {
		log.Fatalf("product %s: %v", productID, err)
	}
	fmt.Println("product", productID)

	lp := &stripe.PriceListParams{LookupKeys: []*string{stripe.String(lookupKey)}}
	for pr, err := range sc.V1Prices.List(ctx, lp).All(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		if pr.LookupKey == lookupKey {
			fmt.Println("price", lookupKey, "(exists)")
			return
		}
	}
	_, err := sc.V1Prices.Create(ctx, &stripe.PriceCreateParams{
		Currency: stripe.String("usd"), LookupKey: stripe.String(lookupKey), Product: stripe.String(productID),
		UnitAmount: stripe.Int64(unitAmount), BillingScheme: stripe.String("per_unit"),
		Recurring: &stripe.PriceCreateRecurringParams{Interval: stripe.String("month"), UsageType: stripe.String("licensed")},
	})
	if err != nil {
		log.Fatalf("price %s: %v", lookupKey, err)
	}
	fmt.Println("price", lookupKey, "(created)")
}
