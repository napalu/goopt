package templates

// NestedGroup represents a hierarchical group structure
type NestedGroup struct {
	Name      string
	Fields    []Field
	SubGroups map[string]*NestedGroup
}

// NestedTemplateData for the new template
type NestedTemplateData struct {
	Package string
	Root    *NestedGroup
}

// PathGroup - helper type for passing path to template
type PathGroup struct {
	Path  string
	Group *NestedGroup
}
