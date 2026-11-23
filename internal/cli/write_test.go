package cli

import (
	"testing"

	"s3reader/internal/config"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		in      string
		want    Target
		wantErr bool
	}{
		{in: "prod", want: Target{Profile: "prod"}},
		{in: "prod.my.bucket", want: Target{Profile: "prod", Bucket: "my.bucket", HasPath: true}},
		{in: "prod.b/dir/", want: Target{Profile: "prod", Bucket: "b", Key: "dir/", HasPath: true}},
		{in: "prod.b/dir/x.txt", want: Target{Profile: "prod", Bucket: "b", Key: "dir/x.txt", HasPath: true}},
		{in: "", wantErr: true},
		{in: ".b", wantErr: true},
		{in: "prod.", wantErr: true},
		{in: "prod./x", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseTarget(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseTarget(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("ParseTarget(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestResolveAndKey(t *testing.T) {
	p := config.Profile{Name: "p", Bucket: "cfg", Prefix: "exports"}
	b, k, err := Target{Profile: "p"}.resolve(p)
	if err != nil || b != "cfg" || k != "exports/" {
		t.Fatalf("resolve = %q %q %v", b, k, err)
	}
	if got := objectKey(k, "/tmp/a/report.csv"); got != "exports/report.csv" {
		t.Errorf("objectKey = %q", got)
	}
	if got := objectKey("", "x.bin"); got != "x.bin" {
		t.Errorf("objectKey root = %q", got)
	}
	if got := objectKey("dir/name.bin", "x.bin"); got != "dir/name.bin" {
		t.Errorf("objectKey exact = %q", got)
	}
	if _, _, err := (Target{Profile: "p"}).resolve(config.Profile{Name: "p"}); err == nil {
		t.Error("expected error for profile without bucket")
	}
}
