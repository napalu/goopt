package i18n

import "sync"

// MessageProvider defines an interface for getting default messages
type MessageProvider interface {
	GetMessage(key string) string
}

// Package-level provider management
var (
	defaultProvider    MessageProvider
	defaultProviderMux sync.RWMutex
)

// SetDefaultMessageProvider allows users to set their own provider
func SetDefaultMessageProvider(p MessageProvider) {
	defaultProviderMux.Lock()
	defer defaultProviderMux.Unlock()
	defaultProvider = p
}

func getDefaultProvider() MessageProvider {
	defaultProviderMux.RLock()
	if defaultProvider != nil {
		defer defaultProviderMux.RUnlock()
		return defaultProvider
	}
	defaultProviderMux.RUnlock()

	// Upgrade to write lock for initialization
	defaultProviderMux.Lock()
	defer defaultProviderMux.Unlock()

	if defaultProvider == nil {
		defaultProvider = NewLayeredMessageProvider(Default(), nil, nil)
	}

	return defaultProvider
}
