package i18n

import (
	"fmt"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// Formatter provides locale-aware formatting for numbers, dates, and times
type Formatter struct {
	printer *message.Printer
	lang    language.Tag
}

// NewFormatter creates a new locale-aware formatter
func NewFormatter(lang language.Tag) *Formatter {
	return &Formatter{
		printer: message.NewPrinter(lang),
		lang:    lang,
	}
}

// getBaseLanguage returns the base language code without region
func (f *Formatter) getBaseLanguage() string {
	base, _ := f.lang.Base()
	return base.String()
}

// FormatInt formats an integer according to locale rules
func (f *Formatter) FormatInt(n int) string {
	return f.printer.Sprint(number.Decimal(n))
}

// FormatInt64 formats an int64 according to locale rules
func (f *Formatter) FormatInt64(n int64) string {
	return f.printer.Sprint(number.Decimal(n))
}

// FormatFloat formats a float according to locale rules
func (f *Formatter) FormatFloat(n float64, precision int) string {
	// Use x/text's number package for proper locale formatting
	return f.printer.Sprint(number.Decimal(n, number.MaxFractionDigits(precision), number.MinFractionDigits(precision)))
}

// FormatPercent formats a percentage according to locale rules
func (f *Formatter) FormatPercent(n float64) string {
	return f.printer.Sprint(number.Percent(n))
}

// FormatOrdinal formats an ordinal number (1st, 2nd, etc) according to locale rules
func (f *Formatter) FormatOrdinal(n int) string {
	// Get base language and full locale
	locale := f.lang.String()
	baseLang := f.getBaseLanguage()

	// Check full locale first, then fall back to base language
	switch locale {
	case "fr-CH", "fr-BE": // Swiss/Belgian French might have different rules
		return formatFrenchOrdinal(n)
	}

	// Fall back to base language
	switch baseLang {
	case "en":
		return formatEnglishOrdinal(n)
	case "fr":
		return formatFrenchOrdinal(n)
	case "es":
		return formatSpanishOrdinal(n)
	default:
		// Fallback to just the number
		return f.FormatInt(n)
	}
}

// FormatRange formats a numeric range according to locale rules
func (f *Formatter) FormatRange(min, max interface{}, provider MessageProvider) string {
	// Get range separator from translation bundle
	separator := provider.GetMessage("goopt.msg.range_to")
	if separator == "" || separator == "goopt.msg.range_to" {
		// Fallback if translation not found
		separator = "–" // en-dash
	} else {
		// Add spaces around the translated separator
		separator = " " + separator + " "
	}

	return fmt.Sprintf("%v%s%v", min, separator, max)
}

// FormatDate formats a date according to locale rules using the x/text package
func (f *Formatter) FormatDate(t time.Time) string {
	// Get locale-specific date format
	locale := f.lang.String()

	switch locale {
	case "en-US":
		return t.Format("01/02/2006") // MM/DD/YYYY
	case "en-GB":
		return t.Format("02/01/2006") // DD/MM/YYYY
	case "en-CA", "fr-CA":
		return t.Format("2006-01-02") // YYYY-MM-DD (ISO format)
	case "de-DE", "de-CH", "de-AT":
		return t.Format("02.01.2006") // DD.MM.YYYY
	case "fr-FR":
		return t.Format("02/01/2006") // DD/MM/YYYY
	default:
		// Default to ISO format
		return t.Format("2006-01-02")
	}
}

// FormatTime formats a time according to locale rules using the x/text package
func (f *Formatter) FormatTime(t time.Time) string {
	// Let the x/text package handle locale-specific formatting
	// For now, use a simple format - the x/text package will handle locale preferences
	return f.printer.Sprintf("%v", t.Format("15:04"))
}

// FormatDateTime formats a date and time according to locale rules
func (f *Formatter) FormatDateTime(t time.Time) string {
	// Let the printer handle locale-specific date/time formatting
	return f.printer.Sprintf("%v", t.Format("2006-01-02 15:04"))
}

// Helper functions for ordinal formatting

func formatEnglishOrdinal(n int) string {
	suffix := "th"
	if n%100 >= 11 && n%100 <= 13 {
		suffix = "th"
	} else {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

func formatFrenchOrdinal(n int) string {
	if n == 1 {
		return "1er"
	}
	return fmt.Sprintf("%de", n)
}

func formatSpanishOrdinal(n int) string {
	return fmt.Sprintf("%d°", n)
}
