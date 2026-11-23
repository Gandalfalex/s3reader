package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ErrExists is returned by Download when dest exists and overwrite is false.
var ErrExists = errors.New("file exists (add ! to overwrite)")

// Download streams the whole object to dest. It writes to dest+".part" and
// renames it into place once complete, so dest never holds a partial file.
// progress, if set, is called with the bytes written so far and the object
// size (-1 when unknown) after every chunk.
func Download(ctx context.Context, c *s3.Client, bucket, key, dest string, overwrite bool, progress func(done, total int64)) (int64, error) {
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return 0, ErrExists
		}
	}
	g, err := c.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return 0, err
	}
	defer g.Body.Close()
	total := int64(-1)
	if g.ContentLength != nil {
		total = aws.ToInt64(g.ContentLength)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, &progressReader{r: g.Body, total: total, fn: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && total >= 0 && n != total {
		err = fmt.Errorf("short download: got %d of %d bytes", n, total)
	}
	if err == nil {
		err = os.Rename(tmp, dest)
	}
	if err != nil {
		os.Remove(tmp)
		return n, err
	}
	return n, nil
}

type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	fn    func(done, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.fn != nil && n > 0 {
		p.fn(p.done, p.total)
	}
	return n, err
}
