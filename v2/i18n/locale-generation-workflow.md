# Workflow for Adding New Languages to goopt

This document describes the recommended workflow for adding new language support to goopt.

## Step 1: Create a Stub Locale File

Create a minimal JSON file with a few basic translations:

```bash
# Create stub for Arabic
cat > i18n/all_locales/ar.json << 'EOF'
{
  "goopt.msg.optional": "اختياري",
  "goopt.msg.commands": "الأوامر"
}
EOF
```

## Step 2: Sync with Reference Locale

Use `goopt-i18n-gen sync` to add all missing keys with [TODO] markers:

```bash
goopt-i18n-gen sync \
  -i "i18n/locales/en.json" \
  -t "i18n/all_locales/ar.json" \
  --todo-prefix "[TODO]"
```

This will:
- Compare ar.json with en.json (reference)
- Add all missing keys with "[TODO]" prefix
- Preserve any existing translations

## Step 3: Translate

Have native speakers replace the [TODO] markers with proper translations:

```json
{
  "goopt.msg.optional": "اختياري",
  "goopt.msg.required": "[TODO] required",  // Needs translation
  "goopt.msg.commands": "الأوامر",
  // ... all other keys with [TODO] markers
}
```

## Step 4: Generate Locale Package

Once translated, generate the Go package:

```bash
goopt-i18n-gen generate-locales \
  -i "i18n/all_locales/ar.json" \
  -o "i18n/locales/"
```

This creates `i18n/locales/ar/ar_gen.go` with the locale package.

## Step 5: Use in Application

```go
import arLocale "github.com/napalu/goopt/v2/i18n/locales/ar"

parser, _ := goopt.NewParser(
    goopt.WithSystemLocales(
        goopt.NewSystemLocale(arLocale.Tag, arLocale.SystemTranslations),
    ),
)
```

## Benefits

1. **Consistency**: All languages have the same keys
2. **Maintainability**: Easy to update when new messages are added
3. **Trackability**: [TODO] markers show what needs translation
4. **Automation**: Most of the process is automated
5. **Quality**: Professional translators can focus on translation, not technical details

## Updating Existing Languages

When new messages are added to goopt:

```bash
# Sync all locale files with English reference
for locale in i18n/all_locales/*.json; do
    if [[ $locale != *"en.json" ]]; then
        goopt-i18n-gen sync -i "i18n/locales/en.json" -t "$locale"
    fi
done

# Regenerate all locale packages
goopt-i18n-gen generate-locales -i "i18n/all_locales/*.json" -o "i18n/locales/"
```

This ensures all languages stay in sync with new features!