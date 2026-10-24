// Command mail-forward is the SES receipt-rule Lambda that forwards mail
// for the published contact addresses (privacy@, legal@, security@ and
// dmarc@ on the apex domain) to the operator's inbox. SES stores each
// message in S3 first; this reads it back, rewrites the sender
// (internal/mailfwd), and re-sends it through SES. Spam and virus
// verdicts of FAIL are dropped, and the stored copy stays in S3 until the
// bucket's lifecycle rule removes it.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/CompositeCode/substratalapps.com/internal/mailfwd"
)

// maxRaw stays under SES's 10 MB limit for a raw message; anything bigger
// is announced instead of forwarded.
const maxRaw = 9 << 20

type forwarder struct {
	s3     *s3.Client
	ses    *sesv2.Client
	bucket string
	prefix string
	to     string
	from   mail.Address
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	f := forwarder{
		s3:     s3.NewFromConfig(cfg),
		ses:    sesv2.NewFromConfig(cfg),
		bucket: mustEnv("INBOUND_BUCKET"),
		prefix: os.Getenv("INBOUND_PREFIX"),
		to:     mustEnv("FORWARD_TO"),
		from:   mail.Address{Address: mustEnv("FORWARD_FROM")},
	}
	lambda.Start(f.handle)
}

func (f forwarder) handle(ctx context.Context, ev events.SimpleEmailEvent) error {
	for _, rec := range ev.Records {
		m, r := rec.SES.Mail, rec.SES.Receipt
		if r.SpamVerdict.Status == "FAIL" || r.VirusVerdict.Status == "FAIL" {
			log.Printf("dropped message_id=%s spam=%s virus=%s", m.MessageID, r.SpamVerdict.Status, r.VirusVerdict.Status)
			continue
		}
		if err := f.forward(ctx, m.MessageID, r.Recipients); err != nil {
			return fmt.Errorf("forward %s: %w", m.MessageID, err)
		}
		log.Printf("forwarded message_id=%s recipients=%s", m.MessageID, strings.Join(r.Recipients, ","))
	}
	return nil
}

func (f forwarder) forward(ctx context.Context, id string, recipients []string) error {
	key := f.prefix + id
	obj, err := f.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: &f.bucket, Key: &key})
	if err != nil {
		return err
	}
	defer func() { _ = obj.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(obj.Body, maxRaw+1))
	if err != nil {
		return err
	}
	if len(raw) > maxRaw {
		return f.notice(ctx, fmt.Sprintf("A message to %s was too large to forward. It's stored at s3://%s/%s.",
			strings.Join(recipients, ", "), f.bucket, key))
	}
	out, err := mailfwd.Rewrite(raw, f.from)
	if err != nil {
		return f.notice(ctx, fmt.Sprintf("A message to %s couldn't be forwarded (%v). It's stored at s3://%s/%s.",
			strings.Join(recipients, ", "), err, f.bucket, key))
	}
	_, err = f.ses.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(f.from.Address),
		Destination:      &types.Destination{ToAddresses: []string{f.to}},
		Content:          &types.EmailContent{Raw: &types.RawMessage{Data: out}},
	})
	return err
}

func (f forwarder) notice(ctx context.Context, text string) error {
	_, err := f.ses.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(f.from.Address),
		Destination:      &types.Destination{ToAddresses: []string{f.to}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String("Substratal: inbound message not forwarded")},
			Body:    &types.Body{Text: &types.Content{Data: aws.String(text)}},
		}},
	})
	return err
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s is required", k)
	}
	return v
}
