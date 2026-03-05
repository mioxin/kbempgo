package dump

import (
	"log/slog"
	"time"

	"github.com/mioxin/kbempgo/internal/worker"
)

type Config struct {
	worker.Config
	Workers         int
	Limit           int
	RootRazd        string
	OpTimeout       time.Duration
	WaitDataTimeout time.Duration
	Debug           int

	Lg *slog.Logger
}
