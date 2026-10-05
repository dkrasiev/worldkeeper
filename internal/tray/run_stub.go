//go:build darwin && !cgo

package tray

import (
	"context"
	"log/slog"
)

// Available is false: the macOS menu bar icon needs cgo (Objective-C).
const Available = false

func Run(ctx context.Context, c Controller, locale string, log *slog.Logger) error {
	return ErrUnavailable
}
