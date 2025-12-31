package observability

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger creates a production or development logger
func InitLogger(isDev bool) (*zap.Logger, error) {
	var config zap.Config

	if isDev {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	// Custom output paths
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// SugaredLogger wraps zap.Logger for easier Printf-style logging
type SugaredLogger struct {
	*zap.SugaredLogger
}

// NewSugaredLogger creates a sugared logger from zap.Logger
func NewSugaredLogger(logger *zap.Logger) *SugaredLogger {
	return &SugaredLogger{logger.Sugar()}
}
