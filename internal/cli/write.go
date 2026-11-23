// Package cli implements the non-interactive subcommands.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"s3reader/internal/config"
	"s3reader/internal/store"
)

const WriteUsage = `usage: s3reader [flags] write <target> <file>...

target:
  profile                    bucket and prefix from the profile's settings
  profile.bucket             bucket root
  profile.bucket/dir/        into dir/ (keeps file names)
  profile.bucket/dir/name    exact key (only with a single file)`

// Target is a parsed write destination.
type Target struct {
	Profile string
	Bucket  string // empty: use the profile's bucket
	Key     string // key or prefix; prefix when empty or ending in "/"
	HasPath bool   // a bucket was given explicitly, so Key overrides the profile prefix
}

// ParseTarget splits "profile[.bucket[/key]]". The profile name ends at the
// first "." because bucket names may themselves contain dots.
func ParseTarget(s string) (Target, error) {
	if s == "" {
		return Target{}, errors.New("empty target")
	}
	prof, rest, hasBucket := strings.Cut(s, ".")
	if prof == "" {
		return Target{}, fmt.Errorf("target %q: missing profile", s)
	}
	t := Target{Profile: prof}
	if !hasBucket {
		return t, nil
	}
	bucket, key, _ := strings.Cut(rest, "/")
	if bucket == "" {
		return Target{}, fmt.Errorf("target %q: missing bucket after %q", s, prof+".")
	}
	t.Bucket, t.Key, t.HasPath = bucket, key, true
	return t, nil
}

// resolve fills in bucket and key prefix from the profile when not given.
func (t Target) resolve(p config.Profile) (bucket, key string, err error) {
	if t.HasPath {
		return t.Bucket, t.Key, nil
	}
	if p.Bucket == "" {
		return "", "", fmt.Errorf("profile %q has no bucket configured, use %s.<bucket>", p.Name, p.Name)
	}
	prefix := p.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return p.Bucket, prefix, nil
}

// objectKey picks the final key for src: dir-style keys get the base name.
func objectKey(key, src string) string {
	if key == "" || strings.HasSuffix(key, "/") {
		return key + filepath.Base(src)
	}
	return key
}

// Write uploads files to target and reports progress on out.
func Write(settings *config.Settings, args []string, out io.Writer) error {
	if len(args) < 2 {
		return errors.New(WriteUsage)
	}
	t, err := ParseTarget(args[0])
	if err != nil {
		return err
	}
	files := args[1:]
	var prof *config.Profile
	for i := range settings.Profiles {
		if settings.Profiles[i].Name == t.Profile {
			prof = &settings.Profiles[i]
			break
		}
	}
	if prof == nil {
		return fmt.Errorf("profile %q not found", t.Profile)
	}
	bucket, key, err := t.resolve(*prof)
	if err != nil {
		return err
	}
	if len(files) > 1 && key != "" && !strings.HasSuffix(key, "/") {
		return fmt.Errorf("key %q must end in / when writing %d files", key, len(files))
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	c, err := store.NewClient(ctx, *prof)
	if err != nil {
		return err
	}
	c, _ = store.ClientForBucket(ctx, c, *prof, bucket)

	for _, f := range files {
		k := objectKey(key, f)
		dest := fmt.Sprintf("s3://%s/%s", bucket, k)
		slog.Info("upload", "profile", prof.Name, "src", f, "dest", dest)
		last := -1
		n, err := store.Upload(ctx, c, bucket, k, f, func(done, total int64) {
			if total <= 0 {
				return
			}
			if pct := int(done * 100 / total); pct != last {
				last = pct
				fmt.Fprintf(out, "\r%s -> %s %3d%%", f, dest, pct)
			}
		})
		if err != nil {
			fmt.Fprintln(out)
			return fmt.Errorf("%s: %w", f, err)
		}
		fmt.Fprintf(out, "\r%s -> %s (%d bytes)\n", f, dest, n)
	}
	return nil
}
