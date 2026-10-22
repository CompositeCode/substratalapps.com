package wire

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/CompositeCode/substratalapps.com/internal/core"
	"github.com/CompositeCode/substratalapps.com/internal/platform/config"
)

// Exports is where finished Audit Event exports go: the AUDIT_EXPORT_BUCKET
// S3 bucket in AWS, or .dev/audit-exports locally.
func Exports(ctx context.Context, cfg config.Config) (core.ExportStore, error) {
	if cfg.AuditExportBucket == "" {
		return LocalExports{Dir: filepath.Join(".dev", "audit-exports")}, nil
	}
	awsCfg, err := AWS(ctx)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg)
	return S3Exports{Client: client, Presign: s3.NewPresignClient(client), Bucket: cfg.AuditExportBucket}, nil
}

// S3Exports stores exports in a private bucket and hands out presigned GETs.
type S3Exports struct {
	Client  *s3.Client
	Presign *s3.PresignClient
	Bucket  string
}

func (e S3Exports) Put(ctx context.Context, key, contentType string, body io.ReadSeeker) error {
	_, err := e.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: &e.Bucket, Key: &key, Body: body,
		ContentType: &contentType, ServerSideEncryption: s3types.ServerSideEncryptionAes256})
	return err
}

func (e S3Exports) URL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := e.Presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &e.Bucket, Key: &key},
		s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// LocalExports writes exports under Dir, for local development.
type LocalExports struct{ Dir string }

func (e LocalExports) Put(_ context.Context, key, _ string, body io.ReadSeeker) error {
	path := filepath.Join(e.Dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (e LocalExports) URL(_ context.Context, key string, _ time.Duration) (string, error) {
	abs, err := filepath.Abs(filepath.Join(e.Dir, filepath.FromSlash(key)))
	if err != nil {
		return "", err
	}
	return "file://" + filepath.ToSlash(abs), nil
}
