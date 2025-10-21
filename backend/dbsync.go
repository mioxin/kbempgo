package backend

import (
	"log/slog"

	"github.com/mioxin/kbempgo/internal/config"
)

type dbSyncCommand struct {
	Glob *config.Globals
	Lg   *slog.Logger `kong:"-"`
}
