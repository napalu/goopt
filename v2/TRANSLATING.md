# 🌍 Translating `goopt`

We want `goopt` to be accessible to developers **everywhere**—regardless of their native language. You can help by contributing translations of system messages used in command-line output and help text.

## Current Translations

The following languages are already supported:

- English (`en`)
- German (`de`)
- French (`fr`)
- Spanish (`es`)
- Chinese (`zh`)
- Japanese (`ja`)
- Hebrew (`he`)
- Arabic (`ar`)
- Hindi (`hi`)
- Portuguese (`pt`)

We're aiming for broad coverage—**every language is welcome**. If yours isn't listed yet, feel free to start it!

---

## Translation Guidelines

We want high-quality, professional translations that feel natural to technical users. Here's how to help us get there:

### Be Idiomatic, Not Literal

- ✅ Use **common, natural phrasing**—how a developer would expect it in their language
- ✅ Prefer **existing CLI conventions** in your language
- 🚫 Avoid word-for-word or overly formal translations

For example, instead of translating "Show this help" literally, use the phrasing you'd expect from `--help` output in your local tools.

### Translate for a Technical Audience

This is developer tooling. Use terminology and tone appropriate for:

- Programmers
- DevOps/SRE engineers
- Command-line power users

### Keep Placeholders Intact

Many strings contain numbered placeholders like `%[1]s`. **Do not change or remove these**—just place them appropriately in your translation.

Example:

```json
"goopt.msg.usage": "Usage: %[1]s"
```

Translate as:

```json
"goopt.msg.usage": "Uso: %[1]s"
```

### Consistency

- Stick to correct spelling, punctuation, and spacing
- Match plural forms and capitalization where needed
- Use UTF-8 encoding

---

## File Format

Each language gets its own JSON file (e.g. `lang/pt.json`, `lang/fi.json`). The format matches the structure in `lang/en.json`. Please:

- Use the same keys
- Keep the file valid JSON (test it before submitting)
- Sort keys alphabetically if possible

---

## How to Contribute

1. Fork the repository
2. Add a new file under `v2/i18n/locales` (e.g., `v2/i18n/locales/it.json`)
3. Translate any strings you'd like (partial contributions are welcome!)
4. Submit a pull request

We’ll review and merge as soon as possible.

---

## 🙌 Credits & Maintainers

If you contribute a translation, feel free to add your name (or handle) to the top of the file as a comment, e.g.:

```jsonc
// Translated by @yourname
{
  ...
}
```

You can also optionally add yourself to the **Maintainers** section below, especially if you’d like to keep your language up to date over time.

---

## 🛠️ Maintainers

If you’d like to act as a contact for a specific language (e.g., to review new strings or update your translation in the future), feel free to open a PR to add your name here.

| Language  | Maintainer(s) |
|-----------|----------------|
| French    | _Available_    |
| Arabic    | _Available_    |
| Hindi     | _Available_    |
| Hebrew    | _Available_    |
| Japanese  | _Available_    |
| Portuguese| _Available_    |
| Spanish   | _Available_    |
| Chinese   | _Available_    |
| German    | _Available_    |
| English   | @napalu        |

---

Thanks for helping make `goopt` truly global! 🌐