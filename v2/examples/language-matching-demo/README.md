# Language Matching Demo

This example demonstrates how goopt's i18n system handles language matching using RFC 4647 language matching algorithm.

## How Language Matching Works

When you request a language that's not exactly available, the system will find the best match:

1. **Exact match**: If the exact language tag exists, use it
2. **Regional fallback**: If a regional variant is requested but not available, fall back to the base language or another regional variant
3. **Base language expansion**: If only the base language is requested (e.g., "en"), match to any available regional variant
4. **Default fallback**: If no suitable match is found, use the bundle's default language

## Examples

### Exact Matches
- Request `en-US` → Get `en-US` (American English)
- Request `de-CH` → Get `de-CH` (Swiss German)

### Regional Fallbacks
- Request `en-NZ` (New Zealand) → Get `en-GB` or `en-US` (closest English variant)
- Request `de-LI` (Liechtenstein) → Get `de-CH` or `de` (closest German variant)
- Request `fr-BE` (Belgian French) → Get `fr` (standard French)

### Base Language Expansion
- Request `en` → Get `en-US`, `en-GB`, `en-CA`, or `en-AU` (any available English)
- Request `de` → Get `de` if available, otherwise `de-DE`, `de-CH`, or `de-AT`

### No Match Fallback
- Request `ja` (Japanese) → Get the bundle's default language (usually English)
- Request `pt` (Portuguese) → Get the bundle's default language

## Running the Demo

```bash
# Test exact match
go run main.go -l en-US

# Test regional fallback (New Zealand English not available)
go run main.go -l en-NZ

# Test base language (will match to any English variant)
go run main.go -l en

# Test with verbose output to see all available languages
go run main.go -l de-LI -v

# Test complete fallback (language not available)
go run main.go -l ja -v
```

## Regional Differences

The demo shows how different regional variants can have different:
- Greetings (e.g., "Hello" vs "G'day")
- Spelling (e.g., "color" vs "colour", "center" vs "centre")
- Currency symbols
- Date formats
- Other locale-specific content

## Implementation Notes

The language matching is implemented using `golang.org/x/text/language.Matcher`, which implements the RFC 4647 algorithm for matching language tags. This ensures that users get the most appropriate language variant based on their preferences and what's available.