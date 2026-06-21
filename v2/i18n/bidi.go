package i18n

// Unicode bidirectional isolate controls (all zero-width). They let a run of one
// direction be embedded in text of the other without the two reordering each
// other under the Unicode Bidi Algorithm. See UAX #9.
const (
	fsi = "⁨" // First Strong Isolate — auto-detects the run's direction
	rli = "⁧" // Right-to-Left Isolate — forces an RTL base direction
	pdi = "⁩" // Pop Directional Isolate — closes FSI/RLI/LRI
)

// Isolate wraps s in FSI…PDI so an embedded run cannot reorder the surrounding
// text (or be reordered by it). FSI auto-detects s's direction from its first
// strong character, so it is correct for a value of unknown direction. This is
// the single isolation primitive shared by error formatting and help rendering —
// keep both on it so the two never drift to different bidi strategies.
func Isolate(s string) string { return fsi + s + pdi }

// IsolateRTL wraps s in RLI…PDI, asserting a right-to-left base direction for the
// whole run. Help lines frequently start with a neutral "--", so first-strong
// auto-detection would otherwise frame an RTL-locale line left-to-right; wrapping
// the assembled line in RLI fixes the base direction.
func IsolateRTL(s string) string { return rli + s + pdi }
