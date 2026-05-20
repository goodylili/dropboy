// Package s3 wraps the AWS S3 client for dropboy's needs (uploads, downloads,
// listings, deletes). Backed by aws-sdk-go-v2 with credentials sourced from
// the standard chain (env, shared profile, IMDS — see PRD §6).
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type Object struct {
	Key          string
	Size         int64
	ETag         string
	VersionID    string
	LastModified time.Time
	Metadata     map[string]string
}

type Client interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, metadata map[string]string) (Object, error)
	Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
	List(ctx context.Context, prefix string) ([]Object, error)
	Delete(ctx context.Context, key string) error
	Head(ctx context.Context, key string) (Object, error)
}

var ErrNotFound = errors.New("object not found")

// Options control client construction.
type Options struct {
	Bucket  string
	Region  string
	Profile string
}

type awsClient struct {
	s3     *awss3.Client
	bucket string
}

// New returns a real S3 client. Credentials resolve via the standard SDK
// chain (env → shared profile → IMDS); pass an empty profile to use whichever
// the environment selects.
func New(ctx context.Context, opt Options) (Client, error) {
	if opt.Bucket == "" {
		return nil, errors.New("bucket required")
	}
	loaders := []func(*awsconfig.LoadOptions) error{
		// Adaptive retries back off on throttling/5xx and respect S3's
		// recommended cadence — meaningful for sync engines that hammer
		// PutObject/ListObjectsV2 from a single client.
		awsconfig.WithRetryer(func() aws.Retryer {
			return retry.NewAdaptiveMode(func(o *retry.AdaptiveModeOptions) {
				o.StandardOptions = append(o.StandardOptions, func(so *retry.StandardOptions) {
					so.MaxAttempts = 5
				})
			})
		}),
	}
	if opt.Region != "" {
		loaders = append(loaders, awsconfig.WithRegion(opt.Region))
	}
	if opt.Profile != "" {
		loaders = append(loaders, awsconfig.WithSharedConfigProfile(opt.Profile))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loaders...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &awsClient{s3: awss3.NewFromConfig(cfg), bucket: opt.Bucket}, nil
}

func (c *awsClient) Put(ctx context.Context, key string, body io.Reader, size int64, metadata map[string]string) (Object, error) {
	in := &awss3.PutObjectInput{
		Bucket:        &c.bucket,
		Key:           &key,
		Body:          body,
		ContentLength: &size,
		Metadata:      metadata,
	}
	out, err := c.s3.PutObject(ctx, in)
	if err != nil {
		return Object{}, fmt.Errorf("put %s: %w", key, err)
	}
	obj := Object{Key: key, Size: size, Metadata: metadata}
	if out.ETag != nil {
		obj.ETag = strings.Trim(*out.ETag, `"`)
	}
	if out.VersionId != nil {
		obj.VersionID = *out.VersionId
	}
	obj.LastModified = time.Now().UTC()
	return obj, nil
}

func (c *awsClient) Get(ctx context.Context, key string) (io.ReadCloser, Object, error) {
	out, err := c.s3.GetObject(ctx, &awss3.GetObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return nil, Object{}, ErrNotFound
		}
		return nil, Object{}, fmt.Errorf("get %s: %w", key, err)
	}
	obj := Object{Key: key, Metadata: out.Metadata}
	if out.ContentLength != nil {
		obj.Size = *out.ContentLength
	}
	if out.ETag != nil {
		obj.ETag = strings.Trim(*out.ETag, `"`)
	}
	if out.VersionId != nil {
		obj.VersionID = *out.VersionId
	}
	if out.LastModified != nil {
		obj.LastModified = *out.LastModified
	}
	return out.Body, obj, nil
}

func (c *awsClient) Head(ctx context.Context, key string) (Object, error) {
	out, err := c.s3.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("head %s: %w", key, err)
	}
	obj := Object{Key: key, Metadata: out.Metadata}
	if out.ContentLength != nil {
		obj.Size = *out.ContentLength
	}
	if out.ETag != nil {
		obj.ETag = strings.Trim(*out.ETag, `"`)
	}
	if out.VersionId != nil {
		obj.VersionID = *out.VersionId
	}
	if out.LastModified != nil {
		obj.LastModified = *out.LastModified
	}
	return obj, nil
}

func (c *awsClient) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	pager := awss3.NewListObjectsV2Paginator(c.s3, &awss3.ListObjectsV2Input{
		Bucket: &c.bucket,
		Prefix: &prefix,
	})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, o := range page.Contents {
			obj := Object{}
			if o.Key != nil {
				obj.Key = *o.Key
			}
			if o.Size != nil {
				obj.Size = *o.Size
			}
			if o.ETag != nil {
				obj.ETag = strings.Trim(*o.ETag, `"`)
			}
			if o.LastModified != nil {
				obj.LastModified = *o.LastModified
			}
			out = append(out, obj)
		}
	}
	return out, nil
}

func (c *awsClient) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var rerr *smithyhttp.ResponseError
	if errors.As(err, &rerr) && rerr.Response != nil && rerr.Response.StatusCode == 404 {
		return true
	}
	return false
}
