// Package store wraps the AWS SDK S3 calls used by the UI.
package store

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"s3reader/internal/config"
)

// Debug enables AWS SDK retry/response logging (set by -debug).
var Debug bool

// Entry is one row in any list: profile, bucket, prefix ("dir") or object.
type Entry struct {
	Key      string // full key (objects/prefixes) or name (profiles/buckets)
	Name     string // display name
	IsDir    bool
	Size     int64
	Modified time.Time
	Class    string
	ETag     string
}

func NewClient(ctx context.Context, p config.Profile) (*s3.Client, error) {
	region := p.Region
	if region == "" {
		region = "us-east-1"
	}
	// Fail fast on unreachable endpoints: the default dial timeout (30s) times
	// 3 retries leaves the UI spinning for ~1.5 minutes.
	httpClient := awshttp.NewBuildableClient().WithDialerOptions(func(d *net.Dialer) {
		d.Timeout = 5 * time.Second
	})
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region), awsconfig.WithHTTPClient(httpClient)}
	if p.AWSProfile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(p.AWSProfile))
	}
	if p.AccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(p.AccessKeyID, p.SecretAccessKey, p.SessionToken)))
	}
	if Debug {
		// LogResponse logs status + headers only; request logging is left off
		// because it would include the Authorization header.
		opts = append(opts, awsconfig.WithLogger(sdkLogger{}),
			awsconfig.WithClientLogMode(aws.LogRetries|aws.LogResponse|aws.LogDeprecatedUsage))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if p.Endpoint != "" {
			o.BaseEndpoint = aws.String(p.Endpoint)
		}
		o.UsePathStyle = p.PathStyle
		// Non-AWS providers often choke on the newer default checksums.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}), nil
}

// ClientForBucket returns a client pinned to the bucket's real region (AWS only).
func ClientForBucket(ctx context.Context, c *s3.Client, p config.Profile, bucket string) (*s3.Client, string) {
	region := c.Options().Region
	if p.Endpoint != "" {
		return c, region
	}
	out, err := c.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: &bucket})
	if err != nil {
		if !IsCancel(err) {
			slog.Warn("GetBucketLocation failed, using profile region", "bucket", bucket, "region", region, "err", err)
		}
		return c, region
	}
	loc := string(out.LocationConstraint)
	if loc == "" {
		loc = "us-east-1"
	} else if loc == "EU" {
		loc = "eu-west-1"
	}
	if loc == region {
		return c, region
	}
	slog.Debug("bucket in other region", "bucket", bucket, "region", loc)
	o := c.Options()
	o.Region = loc
	return s3.New(o), loc
}

func ListBuckets(ctx context.Context, c *s3.Client) ([]Entry, error) {
	var res []Entry
	p := s3.NewListBucketsPaginator(c, &s3.ListBucketsInput{})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, b := range out.Buckets {
			name := aws.ToString(b.Name)
			res = append(res, Entry{Key: name, Name: name, Modified: aws.ToTime(b.CreationDate)})
		}
	}
	return res, nil
}

// ListDir lists one "directory" level using the "/" delimiter.
func ListDir(ctx context.Context, c *s3.Client, bucket, prefix string) ([]Entry, error) {
	var res []Entry
	p := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    &prefix,
		Delimiter: aws.String("/"),
	})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, cp := range out.CommonPrefixes {
			k := aws.ToString(cp.Prefix)
			res = append(res, Entry{Key: k, Name: strings.TrimPrefix(k, prefix), IsDir: true})
		}
		for _, o := range out.Contents {
			k := aws.ToString(o.Key)
			if k == prefix { // folder placeholder object
				continue
			}
			res = append(res, ObjEntry(o, strings.TrimPrefix(k, prefix)))
		}
	}
	return res, nil
}

func ObjEntry(o types.Object, name string) Entry {
	return Entry{
		Key:      aws.ToString(o.Key),
		Name:     name,
		Size:     aws.ToInt64(o.Size),
		Modified: aws.ToTime(o.LastModified),
		Class:    string(o.StorageClass),
		ETag:     strings.Trim(aws.ToString(o.ETag), `"`),
	}
}

// Walk streams every object below prefix (no delimiter) page by page.
func Walk(ctx context.Context, c *s3.Client, bucket, prefix string, fn func([]types.Object) error) error {
	p := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &prefix})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return err
		}
		if err := fn(out.Contents); err != nil {
			return err
		}
	}
	return nil
}
