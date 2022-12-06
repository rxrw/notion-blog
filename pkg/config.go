package notion_blog

type TransferConfig struct {
	DatabaseID string `usage:"ID of the Notion database."`

	// Storage config, required:
	ImagesLink    string `usage:"Directory in which the content will be generated."`
	ImagesFolder  string `usage:"Directory in which the static images will be stored. E.g.: ./web/static/images"`
	ContentFolder string `usage:"URL beggining to link the static images. E.g.: /images"`
	ArchetypeFile string `usage:"Route to the archetype file to generate the header."`

	// Properties mapping from notion to final markdown front matter, optional:
	PropertyDescription string `usage:"Description property name in Notion."`
	PropertyTags        string `usage:"Tags multi-select porperty name in Notion."`
	PropertyCategories  string `usage:"Categories multi-select porperty name in Notion."`

	// Categories mapping from notion to final markdown front matter, required:
	CategoryMap map[string]string `usage:"Map of categories in Notion to categories in Hugo."`

	// Filter config, optional:
	FilterProp     string   `usage:"Property of the filter to apply to a select value of the articles."`
	FilterValue    []string `usage:"Value of the filter to apply to the Notion articles database."`
	PublishedValue string   `usage:"Value to which the filter property will be set after generating the content."`

	// Other config, optional:
	UseDateForFilename bool `usage:"Use the creation date to generate the post filename."`
	UseShortcodes      bool `usage:"True if you want to generate shortcodes for unimplemented markdown blocks, such as callout or quote."`
}
