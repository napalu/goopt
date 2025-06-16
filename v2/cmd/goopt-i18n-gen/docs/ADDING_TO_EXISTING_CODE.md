# Adding i18n to existing goopt applications

## The Problem

When adding i18n to an existing goopt application, there's a chicken-and-egg problem:
- If you add `descKey` tags first, goopt immediately tries to use them
- But the translations don't exist yet, so it displays raw keys like "app.global.help_desc"
- This creates a broken intermediate state where your app shows untranslated keys

## The Solution: Correct Order of Operations

The 360° workflow must follow this specific order to avoid broken states:

### Step 1: Initialize
```bash
goopt-i18n-gen -i locales/en.json init
```
Creates an empty translation file.

### Step 2: Scan and Generate Translations (do not update source yet)
```bash
goopt-i18n-gen -i locales/en.json scan -d -g --key-prefix myapp
```
This:
- Finds fields without descKey tags
- Shows what descKeys it would add
- **Generates the translation entries in your JSON file**
- **Does not modify your source code yet**

### Step 3: Verify Translations Exist
Check that locales/en.json now contains all the translations:
```bash
cat locales/en.json | jq . | head -20
```

### Step 4: Now Update Source Code
```bash
goopt-i18n-gen -i locales/en.json scan -d -u --key-prefix myapp
```
Only NOW is it safe to add descKeys to your source, because the translations exist.

### Step 5: Generate Constants
```bash
goopt-i18n-gen -i locales/en.json generate -o messages/keys.go -p messages
```

## Why Order Matters

Consider this example:

```go
type Config struct {
    Help bool `goopt:"short:h;desc:Show help"`
}
```

**Wrong Order (scan -u before scan -g):**
1. You run `scan -d -u` which adds `descKey:app.config.help_desc`
2. Your struct becomes: `Help bool `goopt:"short:h;desc:Show help;descKey:app.config.help_desc"`
3. You run your app with `--help`
4. goopt looks for translation "app.config.help_desc" but it doesn't exist
5. Output shows: "--help or -h "app.config.help_desc"" 

**Correct Order (scan -g before scan -u):**
1. You run `scan -d -g` which adds to locales/en.json: `"app.config.help_desc": "Show help"`
2. You run `scan -d -u` which adds the descKey to your struct
3. You run your app with `--help`
4. goopt finds the translation and shows: "--help or -h "Show help"" 

## Multi-Language Workflow

```bash
# 1. Start with base language
goopt-i18n-gen -i locales/en.json init
goopt-i18n-gen -i locales/en.json scan -d -g --key-prefix myapp
goopt-i18n-gen -i locales/en.json scan -d -u --key-prefix myapp

# 2. Add more languages
cp locales/en.json locales/de.json
cp locales/en.json locales/fr.json
# Edit the new files to translate values

# 3. Generate constants from ALL locales
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go -p messages

# 4. Validate
goopt-i18n-gen -i "locales/*.json" validate -s "*.go"
```

## Common Pitfalls

### Pitfall 1: Running scan -u without scan -g
**Symptom**: Your app shows raw translation keys instead of translated text.
**Fix**: Run `scan -d -g` first to generate translations, then `scan -d -u`.

### Pitfall 2: Forgetting to regenerate after adding fields
**Symptom**: New fields show raw keys.
**Fix**: Re-run the scan workflow for new fields.

### Pitfall 3: Using only one locale file for generation
**Symptom**: Constants missing for locale-specific keys.
**Fix**: Always use `goopt-i18n-gen -i "locales/*.json" generate` to include all keys.

### Pitfall 4: Manual descKey addition
**Symptom**: Typos in descKey values, mismatched keys.
**Fix**: Always use `scan -d -u` for consistency.

## Quick Reference

```bash
# Full 360° workflow in correct order
goopt-i18n-gen -i locales/en.json init                          # Create file
goopt-i18n-gen -i locales/en.json scan -d -g --key-prefix app   # Generate translations
goopt-i18n-gen -i locales/en.json scan -d -u --key-prefix app   # Update source
goopt-i18n-gen -i "locales/*.json" generate -o messages/keys.go # Generate constants (use wildcards!)
```

Remember: **Translations first, descKeys second!**

---

## For goopt-i18n-gen Maintainers Only

### Dealing with the Chicken-and-Egg Problem

When renaming commands or making structural changes to goopt-i18n-gen itself, you may encounter a chicken-and-egg problem where the code references message keys that don't exist yet in the generated messages file.

#### Example: Renaming a command (like scan → audit)

1. **The Problem**:
    - Your code uses `messages.Keys.AppAudit.SomeField`
    - But the messages file still has `AppScan` because it hasn't been regenerated
    - You can't build to regenerate because the code won't compile

2. **The Solution**:
   ```bash
   # Step 1: Temporarily revert code to use old keys
   cd /<src_path>/goopt/v2/cmd/goopt-i18n-gen
   sed -i '' 's/messages\.Keys\.AppAudit/messages.Keys.AppScan/g' main.go
   
   # Step 2: Regenerate messages with the new locale keys
   go run . -i "locales/*.json" generate -o messages/messages.go -p messages
   
   # Step 3: Change code back to use new keys
   sed -i '' 's/messages\.Keys\.AppScan/messages.Keys.AppAudit/g' main.go
   
   # Step 4: Build and test
   go build
   ```

3. **Prevention**:
    - Always update locale JSON files first
    - Keep old message keys temporarily when making structural changes
    - Consider using go:generate with build tags for bootstrap scenarios

### Other Maintenance Notes

- The tool uses its own i18n system (eat-your-dog-food)
- Always test with multiple locales after changes
- Keep in mind that goopt's bundle validation requires all locales to have identical keys
- The generated messages file is committed to avoid bootstrap issues