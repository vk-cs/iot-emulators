package internal

import (
	"context"
)

type Emulator interface {
	Bootstrap(ctx context.Context) error
	Run(ctx context.Context) error
}
