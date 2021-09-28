package logger

import (
	"encoding/json"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func New(level string) (*zap.SugaredLogger, error) {
	var cfg zap.Config
	enc := "json"

	if isatty.IsTerminal(os.Stdout.Fd()) {
		enc = "console"
	}

	cfgJSON := []byte(`{
		"development": false,
	  "level": "` + level + `",
	  "encoding": "` + enc + `",
	  "outputPaths": ["stderr"],
	  "errorOutputPaths": ["stderr"],
	  "encoderConfig": {
			"timeKey": "timestamp",
			"timeEncoder": "iso8601",
	    "messageKey": "message",
	    "levelKey": "level",
	    "levelEncoder": "capital"
	  }
	}`)

	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return nil, errors.Wrap(err, "json unmarshalling error")
	}

	logger, err := cfg.Build()
	if err != nil {
		return nil, errors.Wrap(err, "logger building error")
	}

	return logger.Sugar(), nil
}
