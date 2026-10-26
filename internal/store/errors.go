package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/smithy-go/logging"
)

// IsCancel reports errors caused by the user aborting an operation.
func IsCancel(err error) bool {
	return errors.Is(err, context.Canceled)
}

// sdkLogger routes AWS SDK log output into slog.
type sdkLogger struct{}

func (sdkLogger) Logf(c logging.Classification, format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	if c == logging.Warn {
		slog.Warn(msg, "src", "aws-sdk")
		return
	}
	slog.Debug(msg, "src", "aws-sdk")
}
