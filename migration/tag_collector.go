package migration

import (
	"fmt"
	"strings"

	"github.com/napalu/goopt/types/orderedmap"
)

// TagCollector handles the conversion of struct tags
type TagCollector struct {
	originalTag string
	gooptTags   *orderedmap.OrderedMap[string, string]
	otherTags   *orderedmap.OrderedMap[string, string]
}

func NewTagCollector(tag string) *TagCollector {
	return &TagCollector{
		originalTag: tag,
		gooptTags:   orderedmap.NewOrderedMap[string, string](),
		otherTags:   orderedmap.NewOrderedMap[string, string](),
	}
}

// Parse separates legacy tags from other tags
func (c *TagCollector) Parse(isGoopt func(s string) bool) error {
	tagStr := strings.Trim(c.originalTag, "`")

	if isGoopt == nil {
		isGoopt = isLegacyTag
	}

	var tags []string
	var currentTag strings.Builder
	inQuote := false

	// Parse tags respecting quoted values
	for i := 0; i < len(tagStr); i++ {
		ch := tagStr[i]
		if ch == '"' {
			inQuote = !inQuote
		}

		if ch == ' ' && !inQuote {
			if currentTag.Len() > 0 {
				tags = append(tags, currentTag.String())
				currentTag.Reset()
			}
		} else {
			currentTag.WriteByte(ch)
		}
	}

	// Add the last tag if any
	if currentTag.Len() > 0 {
		tags = append(tags, currentTag.String())
	}

	for _, tag := range tags {
		key, value, err := parseTagKeyValue(tag)
		if err != nil {
			return fmt.Errorf("parsing tag %q: %w", tag, err)
		}

		if isGoopt(key) {
			c.gooptTags.Set(key, value)
		} else {
			c.otherTags.Set(key, value)
		}
	}

	return nil
}

// parseTagKeyValue splits a tag into key and value
func parseTagKeyValue(tag string) (key, value string, err error) {
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tag format: %s", tag)
	}

	// Remove quotes from value
	value = strings.Trim(parts[1], "\"")
	return parts[0], value, nil
}

// isLegacyTag checks if a tag is a legacy goopt tag
func isLegacyTag(key string) bool {
	legacyPrefixes := []string{
		"long",
		"short",
		"description",
		"type",
		"required",
		"secure",
		"prompt",
		"path",
		"accepted",
		"depends",
		"default",
	}

	for _, prefix := range legacyPrefixes {
		if key == prefix {
			return true
		}
	}
	return false
}

func isGooptTag(key string) bool {
	return strings.HasPrefix(key, "goopt:")
}
