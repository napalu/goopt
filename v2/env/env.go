package env

import "os"

// Resolver defines an interface for environment resolution.
type Resolver interface {
	// Get returns the value of the environment variable named by the key.
	// It returns an empty string if the variable is not present.
	Get(key string) string

	// Set sets the value of the environment variable named by the key.
	// It returns an error, if any.
	Set(key, value string) error

	// Environ returns a slice of strings in the form "key=value" representing the environment,
	// similar to os.Environ.
	Environ() []string
}

// DefaultEnvResolver is the default implementation of the Resolver interface
// that encapsulates environment resolution using the os package.
type DefaultEnvResolver struct{}

// Get returns the value of the environment variable associated with the given key.
func (r *DefaultEnvResolver) Get(key string) string {
	return os.Getenv(key)
}

// Set sets the value of the environment variable identified by key.
func (r *DefaultEnvResolver) Set(key, value string) error {
	return os.Setenv(key, value)
}

// Environ returns a copy of strings representing the environment, as "key=value" pairs.
func (r *DefaultEnvResolver) Environ() []string {
	return os.Environ()
}
