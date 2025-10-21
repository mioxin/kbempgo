package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/dustin/go-humanize"
)

func Type[T any](key string, value T) slog.Attr {
	return slog.String(key, fmt.Sprintf("%T", value))
}

func JSON[T any](key string, value T) slog.Attr {
	buf, _ := json.Marshal(value)
	return slog.String(key, string(buf))
}

func HumanBytes[T ~int64 | ~uint64 | ~int32 | ~uint32 | ~int | ~uint](key string, size T) slog.Attr {
	sz := humanize.IBytes(uint64(size))
	return slog.String(key, sz)
}
