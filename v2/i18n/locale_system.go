package i18n

import (
	"golang.org/x/text/language"
)

// SystemLocaleDetector provides a common interface for detecting system locale
// across different platforms
type SystemLocaleDetector interface {
	// DetectLocale detects the system locale
	DetectLocale() (language.Tag, error)
}

// DefaultSystemLocaleDetector is the default implementation that uses
// platform-specific detection methods
type DefaultSystemLocaleDetector struct{}

// DetectLocale implements SystemLocaleDetector
func (d *DefaultSystemLocaleDetector) DetectLocale() (language.Tag, error) {
	return GetSystemLocale()
}
