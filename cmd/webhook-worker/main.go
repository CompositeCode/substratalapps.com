// Command webhook-worker is the SQS-triggered Lambda that delivers
// outbound webhooks and push notifications. A message only nudges it; it
// always drains both outboxes and every due retry, so a lost or duplicate
// message is harmless. Pushes go first, because their outcomes can emit
// webhook events (device.deactivated, notification.push_failed).
package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/CompositeCode/substratalapps.com/internal/platform/wire"
)

func main() {
	ctx := context.Background()
	base, err := wire.NewBase(ctx)
	if err != nil {
		log.Fatal(err)
	}
	d := base.Deliverer()
	api := base.Server(nil)
	lambda.Start(func(ctx context.Context, ev events.SQSEvent) error {
		if err := api.RunPush(ctx); err != nil {
			return err
		}
		return d.Run(ctx)
	})
}
