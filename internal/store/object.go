// Package store wraps the AWS SDK S3 calls used by the UI.
package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const PreviewBytes = 64 * 1024

type ObjectDetail struct {
	Key     string
	Head    *s3.HeadObjectOutput
	Tags    [][2]string
	Preview []byte
	Err     error
}

func FetchObject(ctx context.Context, c *s3.Client, bucket, key string) ObjectDetail {
	d := ObjectDetail{Key: key}
	h, err := c.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		d.Err = err
		return d
	}
	d.Head = h
	if t, err := c.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{Bucket: &bucket, Key: &key}); err == nil {
		for _, tag := range t.TagSet {
			d.Tags = append(d.Tags, [2]string{aws.ToString(tag.Key), aws.ToString(tag.Value)})
		}
	} else if !IsCancel(err) {
		slog.Debug("GetObjectTagging failed", "key", key, "err", err)
	}
	if aws.ToInt64(h.ContentLength) == 0 {
		return d
	}
	if h.StorageClass == types.StorageClassGlacier || h.StorageClass == types.StorageClassDeepArchive {
		return d // not readable without restore
	}
	g, err := c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket, Key: &key,
		Range: aws.String(fmt.Sprintf("bytes=0-%d", PreviewBytes-1)),
	})
	if err != nil {
		d.Err = fmt.Errorf("preview: %w", err)
		return d
	}
	defer g.Body.Close()
	d.Preview, err = io.ReadAll(io.LimitReader(g.Body, PreviewBytes))
	if err != nil {
		d.Err = fmt.Errorf("preview read: %w", err)
	}
	return d
}
