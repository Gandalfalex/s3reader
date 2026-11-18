package ui

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/aws/smithy-go"
)

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
