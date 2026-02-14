package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func New(environment, level string) (zerolog.Logger, error) {
	parsedLevel, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("parse LOG_LEVEL=%q: %w", level, err)
	}

	var writer io.Writer = os.Stdout
	if strings.EqualFold(strings.TrimSpace(environment), "local") {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	logger := zerolog.New(writer).
		Level(parsedLevel).
		With().
		Timestamp().
		Str("service", "scoop").
		Logger()

	return logger, nil
}
