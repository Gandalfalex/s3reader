package store

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func fakeS3(t *testing.T, body string) *s3.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bkt/dir/file.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(srv.URL),
		UsePathStyle: true,
		Credentials:  aws.AnonymousCredentials{},
	})
}

func TestDownload(t *testing.T) {
	c := fakeS3(t, "hello world")
	dest := filepath.Join(t.TempDir(), "sub", "out.txt")
	var last int64
	n, err := Download(context.Background(), c, "bkt", "dir/file.txt", dest, false, func(done, total int64) {
		last = done
		if total != 11 {
			t.Errorf("total %d", total)
		}
	})
	if err != nil || n != 11 || last != 11 {
		t.Fatalf("n=%d last=%d err=%v", n, last, err)
	}
	if b, _ := os.ReadFile(dest); string(b) != "hello world" {
		t.Fatalf("content %q", b)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part left behind")
	}

	if _, err := Download(context.Background(), c, "bkt", "dir/file.txt", dest, false, nil); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	if _, err := Download(context.Background(), c, "bkt", "dir/file.txt", dest, true, nil); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
}

func TestDownloadMissing(t *testing.T) {
	c := fakeS3(t, "x")
	dest := filepath.Join(t.TempDir(), "out")
	if _, err := Download(context.Background(), c, "bkt", "nope", dest, false, nil); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest created on error")
	}
}
