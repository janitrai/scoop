package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// EnvLoader loads .env files with a predictable override order.
type EnvLoader struct {
	value       *string
	defaultPath string
}

// AddEnvFlag registers an --env flag and returns an EnvLoader.
func AddEnvFlag(fs *flag.FlagSet, defaultPath, description string) *EnvLoader {
	if fs == nil {
		fs = flag.CommandLine
	}
	if defaultPath == "" {
		defaultPath = ".env"
	}
	if description == "" {
		description = "Path to the .env file"
	}

	value := fs.String("env", defaultPath, description)
	return &EnvLoader{
		value:       value,
		defaultPath: defaultPath,
	}
}

// Load resolves and loads environment variables using the configured flag value.
func (l *EnvLoader) Load() (string, error) {
	if l == nil {
		return "", fmt.Errorf("env loader is nil")
	}

	log.SetOutput(os.Stderr)

	overrideVars := []string{"NEWS_PIPELINE_ENV_FILE", "HORSE_ENV_FILE"}
	for _, envVar := range overrideVars {
		if custom := strings.TrimSpace(os.Getenv(envVar)); custom != "" {
			if err := godotenv.Overload(custom); err == nil {
				log.Printf("Loaded environment from %s: %s", envVar, custom)
				return custom, nil
			}
			log.Printf("Warning: failed to load %s=%s", envVar, custom)
		}
	}

	requested := strings.TrimSpace(derefString(l.value))
	if requested == "" {
		requested = l.defaultPath
	}

	if err := godotenv.Overload(requested); err == nil {
		log.Printf("Loaded environment from: %s", requested)
		return requested, nil
	}

	base := filepath.Base(requested)
	if base != "" && base != requested {
		if err := godotenv.Overload(base); err == nil {
			log.Printf("Loaded environment from basename fallback: %s", base)
			return base, nil
		}
	}

	if requested != l.defaultPath {
		if err := godotenv.Overload(l.defaultPath); err == nil {
			log.Printf("Loaded environment from fallback: %s", l.defaultPath)
			return l.defaultPath, nil
		}
	}

	return "", fmt.Errorf("failed to load env file from %s", requested)
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
