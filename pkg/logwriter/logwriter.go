package logwriter

import (
	"io"

	"github.com/rs/zerolog"
)

type LogWriter struct {
	log   *zerolog.Logger
	level zerolog.Level
}

func New(log *zerolog.Logger, level zerolog.Level) io.Writer {
	return LogWriter{log, level}
}

func (l LogWriter) Write(p []byte) (n int, err error) {
	l.log.WithLevel(l.level).Msg(string(p))
	return len(p), nil
}
