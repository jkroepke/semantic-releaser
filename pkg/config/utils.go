package config

import (
	"os"
)

// lookupEnvOrString returns the value of the environment variable named by the key,
// or the default value if the variable is not set.
func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return defaultVal
}
