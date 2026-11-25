package store

import (
	"context"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StaleUpload is the age after which an unfinished multipart upload is
// counted as abandoned rather than in flight.
const StaleUpload = time.Hour

// maxSized caps the ListParts calls per scan; the oldest uploads are sized
// first since those are the ones most likely abandoned.
const maxSized = 25

// OpenUpload is one unfinished multipart upload.
type OpenUpload struct {
	Key       string
	ID        string
	Initiated time.Time
	Parts     int
	Bytes     int64
	Sized     bool
}

// Uploads summarises the unfinished multipart uploads below a prefix. Their
// parts are billed storage that no object listing shows.
type Uploads struct {
	Count  int
	Stale  int // older than StaleUpload
	Oldest OpenUpload
	Bytes  int64 // sum over the sized uploads
	Sized  int
	Err    error
}

// ListUploads lists unfinished multipart uploads below prefix and sizes the
// oldest maxSized of them with ListParts.
func ListUploads(ctx context.Context, c *s3.Client, bucket, prefix string) Uploads {
	var res Uploads
	var all []OpenUpload
	p := s3.NewListMultipartUploadsPaginator(c, &s3.ListMultipartUploadsInput{Bucket: &bucket, Prefix: &prefix})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			res.Err = err
			return res
		}
		for _, u := range out.Uploads {
			all = append(all, OpenUpload{Key: aws.ToString(u.Key), ID: aws.ToString(u.UploadId), Initiated: aws.ToTime(u.Initiated)})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Initiated.Before(all[j].Initiated) })
	res.Count = len(all)
	for i := range all {
		if time.Since(all[i].Initiated) > StaleUpload {
			res.Stale++
		}
		if i >= maxSized {
			continue
		}
		pp := s3.NewListPartsPaginator(c, &s3.ListPartsInput{Bucket: &bucket, Key: &all[i].Key, UploadId: &all[i].ID})
		for pp.HasMorePages() {
			out, err := pp.NextPage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					res.Err = ctx.Err()
					return res
				}
				break // upload completed or aborted meanwhile
			}
			for _, part := range out.Parts {
				all[i].Parts++
				all[i].Bytes += aws.ToInt64(part.Size)
			}
			all[i].Sized = true
		}
		if all[i].Sized {
			res.Sized++
			res.Bytes += all[i].Bytes
		}
	}
	if len(all) > 0 {
		res.Oldest = all[0]
	}
	return res
}

// CountUploads reads only the first page of unfinished uploads, for the
// bucket info page: count (truncated means "at least") and oldest.
func CountUploads(ctx context.Context, c *s3.Client, bucket string) (n int, truncated bool, oldest time.Time, err error) {
	out, err := c.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{Bucket: &bucket})
	if err != nil {
		return 0, false, time.Time{}, err
	}
	for _, u := range out.Uploads {
		if t := aws.ToTime(u.Initiated); oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	return len(out.Uploads), aws.ToBool(out.IsTruncated), oldest, nil
}
