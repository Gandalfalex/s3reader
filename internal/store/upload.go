package store

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MaxPutSize is the largest object a single PutObject may carry.
const MaxPutSize = 5 << 30

// Upload puts the local file src at bucket/key in one PutObject call and
// returns the number of bytes sent. progress, if set, is called after every
// chunk with the bytes read so far and the file size.
func Upload(ctx context.Context, c *s3.Client, bucket, key, src string, progress func(done, total int64)) (int64, error) {
	f, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if !st.Mode().IsRegular() {
		return 0, fmt.Errorf("%s: not a regular file", src)
	}
	size := st.Size()
	if size > MaxPutSize {
		return 0, fmt.Errorf("%s: %d bytes exceeds single-upload limit of 5 GiB", src, size)
	}
	in := &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &key,
		Body:          &progressReader{r: f, total: size, fn: progress},
		ContentLength: aws.Int64(size),
	}
	if ct := mime.TypeByExtension(filepath.Ext(src)); ct != "" {
		in.ContentType = aws.String(ct)
	}
	if _, err := c.PutObject(ctx, in); err != nil {
		return 0, err
	}
	return size, nil
}
