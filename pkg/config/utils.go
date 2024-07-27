package config

import (
	"os"
	"strconv"
)

// lookupEnvOrString returns the value of the environment variable named by the key,
// or the default value if the variable is not set.
func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return defaultVal
}

// lookupEnvOrString returns the value of the environment variable named by the key,
// or the default value if the variable is not set.
func lookupEnvOrBool(key string, defaultVal bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal
	}

	if parseBool, err := strconv.ParseBool(val); err == nil {
		return parseBool
	}

	return defaultVal
}
