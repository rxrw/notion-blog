package notion_blog

import (
	"log"
	"strings"

	"github.com/jomei/notionapi"
)

type ArchetypeFields struct {
	Title        string
	Description  string
	Banner       string
	CreationDate string
	LastModified string
	Author       string
	Tags         []string
	Category     string
	Content      string
	Toc          bool
	Gallery      bool
	Properties   notionapi.Properties
}

func MakeArchetypeFields(p notionapi.Page, config BlogConfig) ArchetypeFields {
	title := ConvertRichText(p.Properties["Name"].(*notionapi.TitleProperty).Title)
	title = strings.Replace(title, "_index", "", 1)
	// Initialize first default Notion page fields
	a := ArchetypeFields{
		Title:        title,
		CreationDate: p.CreatedTime.Format("2006-01-02T15:04:05+08:00"),
		LastModified: p.LastEditedTime.Format("2006-01-02T15:04:05+08:00"),
		Author:       p.Properties["Created by"].(*notionapi.CreatedByProperty).CreatedBy.Name,
	}

	a.Banner = ""
	if p.Cover != nil && p.Cover.GetURL() != "" {
		coverSrc, _ := getImage(p.Cover.GetURL(), config)
		a.Banner = coverSrc
	}

	if v, ok := p.Properties["Toc"]; ok {
		checked, ok := v.(*notionapi.CheckboxProperty)
		if ok {
			a.Toc = checked.Checkbox
		} else {
			log.Println("warning: given property categories is not a select property")
		}
	}

	if v, ok := p.Properties["Gallery"]; ok {
		checked, ok := v.(*notionapi.CheckboxProperty)
		if ok {
			a.Gallery = checked.Checkbox
		} else {
			log.Println("warning: given property categories is not a select property")
		}
	}

	// Custom fields
	if v, ok := p.Properties[config.PropertyDescription]; ok {
		text, ok := v.(*notionapi.RichTextProperty)
		if ok {
			a.Description = ConvertRichText(text.RichText)
		} else {
			log.Println("warning: given property description is not a text property")
		}
	}

	if v, ok := p.Properties[config.PropertyCategories]; ok {
		selects, ok := v.(*notionapi.SelectProperty)
		if ok {
			a.Category = selects.Select.Name
		} else {
			log.Println("warning: given property categories is not a select property")
		}
	}

	tags := make([]string, 0)
	if v, ok := p.Properties[config.PropertyTags]; ok {
		multiSelect, ok := v.(*notionapi.MultiSelectProperty)
		if ok {
			for _, tag := range multiSelect.MultiSelect {
				tags = append(tags, tag.Name)
			}
		} else {
			log.Println("warning: given property tags is not a multi-select property")
		}
	}
	a.Tags = tags

	a.Properties = p.Properties

	return a
}
