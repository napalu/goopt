# Makefile for goopt-i18n-gen 360° workflow

# Variables
I18N_GEN := ../../cmd/goopt-i18n-gen/goopt-i18n-gen
LOCALE_FILE := locales/en.json
OUTPUT_FILE := messages/messages.go
PACKAGE := messages
KEY_PREFIX := app
SOURCE_FILES := main.go

# Default target
.PHONY: help
help:
	@echo "goopt-i18n-gen 360° Workflow Targets:"
	@echo "  make init        - Initialize empty translation file"
	@echo "  make analyze     - Find fields without descKeys and generate suggestions (Step 1)"
	@echo "  make analyze-update - Same as analyze but auto-updates source files! (Step 1b)"
	@echo "  make generate    - Generate constants file from existing translations (Step 3)"
	@echo "  make validate    - Validate all descKeys have translations (Step 2)"
	@echo "  make strict      - CI/CD validation with strict mode (Step 4)"
	@echo "  make clean       - Remove generated files"
	@echo ""
	@echo "Typical workflow:"
	@echo "  1. make init analyze-update  - Fully automated 360° workflow!"
	@echo "  2. make generate             - Generate the constants file"
	@echo "  3. make validate             - Ensure everything is properly set up"
	@echo ""
	@echo "Or manually:"
	@echo "  1. make init analyze  - Start fresh and see suggestions"
	@echo "  2. Add descKey tags to your struct manually"
	@echo "  3. make generate      - Generate the constants file"

# Initialize empty translation file
.PHONY: init
init:
	@mkdir -p locales
	@echo '{}' > $(LOCALE_FILE)
	@echo "✓ Created empty translation file: $(LOCALE_FILE)"

# Step 1: Analyze code and generate descKey suggestions
.PHONY: analyze
analyze:
	@echo "🔍 Analyzing structs for missing descKeys..."
	@$(I18N_GEN) -i $(LOCALE_FILE) audit \
		-d -g --key-prefix $(KEY_PREFIX) --files $(SOURCE_FILES)

# Step 1b: Analyze AND auto-update source files (360° automation!)
.PHONY: analyze-update
analyze-update:
	@echo "🔍 Analyzing structs and auto-updating source files..."
	@$(I18N_GEN) -i $(LOCALE_FILE) audit \
		-d -g -u --key-prefix $(KEY_PREFIX) --files $(SOURCE_FILES)

# Step 2: Validate (development mode - non-strict)
.PHONY: validate
validate:
	@echo "✅ Validating descKey references..."
	@$(I18N_GEN) -i $(LOCALE_FILE) validate \
		-s $(SOURCE_FILES)

# Step 3: Generate constants file
.PHONY: generate
generate:
	@echo "🔨 Generating constants file..."
	@$(I18N_GEN) -i $(LOCALE_FILE) generate -o $(OUTPUT_FILE) -p $(PACKAGE)
	@echo "✓ Generated $(OUTPUT_FILE)"

# Step 4: Strict validation for CI/CD
.PHONY: strict
strict:
	@echo "🚨 Running strict validation (CI/CD mode)..."
	@$(I18N_GEN) -i $(LOCALE_FILE) validate \
		-s $(SOURCE_FILES) --strict

# Clean generated files
.PHONY: clean
clean:
	@rm -f $(OUTPUT_FILE)
	@rm -rf messages/
	@echo "✓ Cleaned generated files"

# Build and run the example
.PHONY: build
build: generate
	go build -o i18n-codegen-demo .

.PHONY: run
run: build
	./i18n-codegen-demo -h

# Complete 360° workflow demonstration
.PHONY: demo-360
demo-360: clean init
	@echo "🎯 Starting 360° Workflow Demo"
	@echo "================================"
	@echo ""
	@echo "Step 1: Analyzing your structs..."
	@$(MAKE) -s analyze
	@echo ""
	@echo "Step 2: Add the suggested descKey tags to your struct"
	@echo "        (This would be manual in real usage)"
	@echo ""
	@echo "Step 3: Generate constants..."
	@$(MAKE) -s generate
	@echo ""
	@echo "Step 4: Validate everything..."
	@$(MAKE) -s validate
	@echo ""
	@echo "✨ 360° Workflow Complete!"