package ui

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/aws/smithy-go"
	"github.com/charmbracelet/x/ansi"
)

func pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := lipgloss.Width(s)
	if sw > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-sw)
}

func padLeft(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	return strings.Repeat(" ", w-sw) + s
}

func trunc(s string, w int) string { return ansi.Truncate(s, w, "…") }

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 2*365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
}

func fmtInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// shortErr condenses AWS SDK errors ("operation error S3: ..., https response
// error StatusCode: 403, RequestID: ..., api error AccessDenied: ...") for the
// status bar. The full text goes to the log and the error screen.
func shortErr(err error) string {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		if msg := ae.ErrorMessage(); msg != "" && msg != ae.ErrorCode() {
			return ae.ErrorCode() + ": " + msg
		}
		return ae.ErrorCode()
	}
	var ne *net.OpError
	if errors.As(err, &ne) {
		return ne.Error() // e.g. "dial tcp 10.0.0.1:9000: i/o timeout"
	}
	return err.Error()
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
