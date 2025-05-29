#!/bin/bash
# Demonstrates the complete 360° i18n workflow

set -e  # Exit on error

echo "🎯 goopt-i18n-gen 360° Workflow Demonstration"
echo "=============================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Clean and initialize
echo -e "${BLUE}Step 1: Initialize${NC}"
echo "Creating empty translation file..."
make clean init
echo ""

# Step 2: Analyze
echo -e "${BLUE}Step 2: Analyze structs for missing descKeys${NC}"
echo "Running analysis..."
make analyze
echo ""

# Step 3: Auto-update option
echo -e "${YELLOW}Step 3: Auto-update available!${NC}"
echo "The tool can automatically add descKey tags to your source files."
echo "To enable auto-update, use the -u flag:"
echo ""
echo "  make analyze-update"
echo "  # or"
echo "  goopt-i18n-gen ... -u"
echo ""
echo "This will:"
echo "- Create backups of your files"
echo "- Add all the suggested descKey tags automatically"
echo "- Preserve your code formatting"
echo ""
echo "For this demo, we'll continue with the manual workflow..."
echo ""

# Step 4: Generate constants
echo -e "${BLUE}Step 4: Generate constants file${NC}"
make generate
echo ""

# Step 5: Show the generated file
echo -e "${BLUE}Step 5: View generated constants${NC}"
echo "Generated messages/messages.go:"
echo "--------------------------------"
head -30 messages/messages.go
echo "..."
echo ""

# Step 6: Validate
echo -e "${BLUE}Step 6: Validate all descKeys have translations${NC}"
make validate
echo ""

# Step 7: Build and test
echo -e "${BLUE}Step 7: Build and test the application${NC}"
make build
./i18n-codegen-demo -h
echo ""

echo -e "${GREEN}✨ 360° Workflow Complete!${NC}"
echo ""
echo "Next steps:"
echo "- Add more languages by copying locales/en.json to locales/[lang].json"
echo "- Use 'make strict' in your CI/CD pipeline"
echo "- Import messages package and use Keys constants in your code"