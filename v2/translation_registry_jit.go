package goopt

import (
	"strings"
	"sync"

	"golang.org/x/text/language"
)

// TranslatableFlag stores metadata for a translatable flag
type TranslatableFlag struct {
	Argument    *Argument
	CommandPath string
}

// TranslatableCommand stores metadata for a translatable command
type TranslatableCommand struct {
	Command *Command
}

// JITTranslationRegistry provides Just-In-Time translation with minimal memory overhead
// Instead of pre-computing all translations for all languages, it only builds
// translations for the currently active language
type JITTranslationRegistry struct {
	mu sync.RWMutex

	// Metadata storage (no translations, just references)
	translatableFlags    map[string]*TranslatableFlag    // canonicalName -> metadata
	translatableCommands map[string]*TranslatableCommand // canonicalPath -> metadata

	// Cache for current language only
	currentLang        language.Tag
	currentFlagForward map[string]string // translated -> canonical
	currentFlagReverse map[string]string // canonical -> translated
	currentCmdForward  map[string]string // translated -> canonical
	currentCmdReverse  map[string]string // canonical -> translated

	// Parser reference for on-demand translation
	parser *Parser

	// Performance optimization: track if cache needs rebuilding
	needsRebuild bool
}

// NewJITTranslationRegistry creates a new JIT translation registry
func NewJITTranslationRegistry(parser *Parser) *JITTranslationRegistry {
	return &JITTranslationRegistry{
		translatableFlags:    make(map[string]*TranslatableFlag),
		translatableCommands: make(map[string]*TranslatableCommand),
		currentFlagForward:   make(map[string]string),
		currentFlagReverse:   make(map[string]string),
		currentCmdForward:    make(map[string]string),
		currentCmdReverse:    make(map[string]string),
		parser:               parser,
	}
}

// RegisterFlagMetadata stores flag metadata without computing translations
func (jit *JITTranslationRegistry) RegisterFlagMetadata(canonicalName string, arg *Argument, commandPath string) {
	jit.mu.Lock()
	defer jit.mu.Unlock()

	key := canonicalName
	if commandPath != "" {
		key = canonicalName + "@" + commandPath
	}

	// Always register the flag, even if it has no translation key
	// This allows direct matching to work
	jit.translatableFlags[key] = &TranslatableFlag{
		Argument:    arg,
		CommandPath: commandPath,
	}

	// Mark cache as needing rebuild (more efficient than immediate invalidation)
	jit.needsRebuild = true
}

// RegisterCommandMetadata stores command metadata without computing translations
func (jit *JITTranslationRegistry) RegisterCommandMetadata(canonicalPath string, cmd *Command) {
	if cmd.NameKey == "" {
		return // No translation key, nothing to register
	}

	jit.mu.Lock()
	defer jit.mu.Unlock()

	// fmt.Printf("DEBUG RegisterCommandMetadata: path=%s, nameKey=%s\n", canonicalPath, cmd.NameKey)
	jit.translatableCommands[canonicalPath] = &TranslatableCommand{
		Command: cmd,
	}

	// Mark cache as needing rebuild (more efficient than immediate invalidation)
	jit.needsRebuild = true
}

// ensureLanguageCached builds the translation cache for a specific language if needed
func (jit *JITTranslationRegistry) ensureLanguageCached(lang language.Tag) {
	jit.mu.Lock()
	defer jit.mu.Unlock()

	// Only rebuild if language changed OR new items were added
	if lang == jit.currentLang && !jit.needsRebuild {
		// Cache is still valid
		return
	}

	// Building cache for language or because new items were added

	// Clear existing cache
	jit.currentLang = lang
	jit.currentFlagForward = make(map[string]string)
	jit.currentFlagReverse = make(map[string]string)
	jit.currentCmdForward = make(map[string]string)
	jit.currentCmdReverse = make(map[string]string)
	jit.needsRebuild = false

	// Get translator
	translator := jit.parser.GetTranslator()

	// Build flag translations for this language only
	for key, metadata := range jit.translatableFlags {
		if metadata.Argument.NameKey != "" {
			translated := translator.TL(lang, metadata.Argument.NameKey)
			if translated != "" && translated != metadata.Argument.NameKey {
				// Extract canonical name (remove command path suffix if present)
				canonicalName := strings.Split(key, "@")[0]

				// Store bidirectional mappings
				jit.currentFlagForward[translated] = key
				jit.currentFlagReverse[canonicalName] = translated
			}
		}
	}

	// Build command translations for this language only
	// Build command translations
	for canonicalPath, metadata := range jit.translatableCommands {
		if metadata.Command.NameKey != "" {
			translated := translator.TL(lang, metadata.Command.NameKey)
			// Debug logging
			// Store translation if it's different from the key
			if translated != "" && translated != metadata.Command.NameKey {
				// Store bidirectional mappings
				jit.currentCmdForward[translated] = canonicalPath
				jit.currentCmdReverse[canonicalPath] = translated
			}
		}
	}
}

// GetCanonicalFlagName returns the canonical name for a potentially translated flag
func (jit *JITTranslationRegistry) GetCanonicalFlagName(name string, lang language.Tag) (string, bool) {
	// Ensure cache is built for this language
	jit.ensureLanguageCached(lang)

	jit.mu.RLock()
	defer jit.mu.RUnlock()

	// Check if it's a translated name
	if canonical, ok := jit.currentFlagForward[name]; ok {
		// Return just the flag name part (without command context)
		parts := strings.Split(canonical, "@")
		return parts[0], true
	}

	// Check if it's already canonical (with or without command context)
	// First check exact match
	if _, ok := jit.translatableFlags[name]; ok {
		return name, true
	}

	// Then check all registered flags to see if any match just the flag name
	for key := range jit.translatableFlags {
		parts := strings.Split(key, "@")
		if parts[0] == name {
			// Return just the flag name (without command context)
			return parts[0], true
		}
	}

	// Not found
	return "", false
}

// GetCanonicalCommandPath returns the canonical path for a potentially translated command
func (jit *JITTranslationRegistry) GetCanonicalCommandPath(name string, lang language.Tag) (string, bool) {
	// Ensure cache is built for this language
	jit.ensureLanguageCached(lang)

	jit.mu.RLock()
	defer jit.mu.RUnlock()

	// Look up in translation cache

	// Check if it's a translated name
	if canonical, ok := jit.currentCmdForward[name]; ok {
		return canonical, true
	}

	// Check if it's already canonical
	if _, ok := jit.translatableCommands[name]; ok {
		return name, true
	}

	// Not found
	return "", false
}

// GetFlagTranslation returns the translated name for a flag in the current language
func (jit *JITTranslationRegistry) GetFlagTranslation(canonicalName string, lang language.Tag) (string, bool) {
	// Ensure cache is built for this language
	jit.ensureLanguageCached(lang)

	jit.mu.RLock()
	defer jit.mu.RUnlock()

	if translated, ok := jit.currentFlagReverse[canonicalName]; ok {
		return translated, true
	}

	return "", false
}

// GetCommandTranslation returns the translated name for a command in the current language
func (jit *JITTranslationRegistry) GetCommandTranslation(canonicalPath string, lang language.Tag) (string, bool) {
	// Ensure cache is built for this language
	jit.ensureLanguageCached(lang)

	jit.mu.RLock()
	defer jit.mu.RUnlock()

	if translated, ok := jit.currentCmdReverse[canonicalPath]; ok {
		return translated, true
	}

	return "", false
}

// GetAllFlagTranslations returns all translations for a flag (requires building cache for each language)
func (jit *JITTranslationRegistry) GetAllFlagTranslations(canonicalName string) map[language.Tag]string {
	// This is expensive with JIT, but rarely used
	result := make(map[language.Tag]string)

	// Check all supported languages
	for _, lang := range jit.parser.GetSupportedLanguages() {
		if translated, ok := jit.GetFlagTranslation(canonicalName, lang); ok {
			result[lang] = translated
		}
	}

	return result
}

// GetAllCommandTranslations returns all translations for a command (requires building cache for each language)
func (jit *JITTranslationRegistry) GetAllCommandTranslations(canonicalPath string) map[language.Tag]string {
	// This is expensive with JIT, but rarely used
	result := make(map[language.Tag]string)

	// Check all supported languages
	for _, lang := range jit.parser.GetSupportedLanguages() {
		if translated, ok := jit.GetCommandTranslation(canonicalPath, lang); ok {
			result[lang] = translated
		}
	}

	return result
}

// Merge merges another JIT registry into this one
func (jit *JITTranslationRegistry) Merge(other *JITTranslationRegistry) {
	if other == nil {
		return
	}

	other.mu.RLock()
	defer other.mu.RUnlock()

	jit.mu.Lock()
	defer jit.mu.Unlock()

	// Merge metadata only (no translations)
	for key, metadata := range other.translatableFlags {
		jit.translatableFlags[key] = metadata
	}

	for key, metadata := range other.translatableCommands {
		jit.translatableCommands[key] = metadata
	}

	// Clear cache to force rebuild on next access
	jit.currentLang = language.Und
}
