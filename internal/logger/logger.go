package logger

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level string) (*zap.SugaredLogger, error) {
	zapLevel, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return nil, errors.Wrap(err, "log level parsing error")
	}

	encoding := "json"
	if isatty.IsTerminal(os.Stdout.Fd()) {
		encoding = "console"
	}

	cfg := zap.Config{
		Development:      false,
		Level:            zapLevel,
		Encoding:         encoding,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:     "timestamp",
			EncodeTime:  zapcore.ISO8601TimeEncoder,
			MessageKey:  "message",
			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalLevelEncoder,
		},
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, errors.Wrap(err, "logger building error")
	}

	return logger.Sugar(), nil
}
