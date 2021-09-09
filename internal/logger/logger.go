package logger

import (
	"encoding/json"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

func New(level string) (types.InventoryLogger, error) {
	var cfg zap.Config
	cfgJSON := []byte(`{
		"development": false,
	  "level": "` + level + `",
	  "encoding": "console",
	  "outputPaths": ["stdout"],
	  "errorOutputPaths": ["stderr"],
	  "encoderConfig": {
			"timeKey": "timestamp",
			"timeEncoder": "iso8601",
	    "messageKey": "message",
	    "levelKey": "level",
	    "levelEncoder": "capitalColor"
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
