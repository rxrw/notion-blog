package notion_blog

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"net/url"

	"github.com/jomei/notionapi"
)

func emphFormat(a *notionapi.Annotations) (s string) {
	s = "%s"
	if a == nil {
		return
	}

	if a.Code {
		return "`%s`"
	}

	switch {
	case a.Bold && a.Italic:
		s = "***%s***"
	case a.Bold:
		s = "**%s**"
	case a.Italic:
		s = "*%s*"
	}

	if a.Underline {
		s = "__" + s + "__"
	} else if a.Strikethrough {
		s = "~~" + s + "~~"
	}

	// TODO: color

	return s
}

func ConvertRich(t notionapi.RichText) string {
	switch t.Type {
	case notionapi.ObjectTypeText:
		if t.Text.Link != nil {
			return fmt.Sprintf(
				emphFormat(t.Annotations),
				fmt.Sprintf("[%s](%s)", t.Text.Content, t.Text.Link),
			)
		}
		return fmt.Sprintf(emphFormat(t.Annotations), t.Text.Content)
	case notionapi.ObjectTypeList:
	}
	return ""
}

func ConvertRichText(t []notionapi.RichText) string {
	buf := &bytes.Buffer{}
	for _, word := range t {
		buf.WriteString(ConvertRich(word))
	}

	return buf.String()
}

func getImage(imgURL string, config BlogConfig) (string, error) {
	// Split image url to get host and file name
	splittedURL, err := url.Parse(imgURL)
	if err != nil {
		return "", fmt.Errorf("malformed url: %s", err)
	}

	// Get file name
	filePath := splittedURL.Path
	filePath = filePath[strings.LastIndex(filePath, "/")+1:]

	name := fmt.Sprintf("%s_%s", splittedURL.Hostname(), filePath)

	log.Println("getting image", name, "...")

	resp, err := http.Get(imgURL)
	if err != nil {
		return "", fmt.Errorf("couldn't download image: %s", err)
	}
	defer resp.Body.Close()

	err = os.MkdirAll(config.ImagesFolder, 0777)
	if err != nil {
		return "", fmt.Errorf("couldn't create images folder: %s", err)
	}

	// Create the file
	out, err := os.Create(filepath.Join(config.ImagesFolder, name))
	if err != nil {
		return name, fmt.Errorf("couldn't create image file: %s", err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return filepath.Join(config.ImagesLink, name), err
}

func MakeArchetypeFields(p notionapi.Page, config BlogConfig) ArchetypeFields {
	// Initialize first default Notion page fields
	a := ArchetypeFields{
		Title:        ConvertRichText(p.Properties["Name"].(*notionapi.TitleProperty).Title),
		CreationDate: p.CreatedTime,
		LastModified: p.LastEditedTime,
		Author:       p.Properties["Created By"].(*notionapi.CreatedByProperty).CreatedBy.Name,
	}

	a.Banner = ""
	if p.Cover != nil && p.Cover.GetURL() != "" {
		coverSrc, err := getImage(p.Cover.GetURL(), config)
		if err != nil {
			log.Println("couldn't download cover:", err)
		}
		a.Banner = coverSrc
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
		multiSelect, ok := v.(*notionapi.MultiSelectProperty)
		if ok {
			a.Categories = multiSelect.MultiSelect
		} else {
			log.Println("warning: given property categories is not a multi-select property")
		}
	}

	if v, ok := p.Properties[config.PropertyTags]; ok {
		multiSelect, ok := v.(*notionapi.MultiSelectProperty)
		if ok {
			a.Tags = multiSelect.MultiSelect
		} else {
			log.Println("warning: given property tags is not a multi-select property")
		}
	}

	return a
}

func Generate(w io.Writer, page notionapi.Page, blocks []notionapi.Block, config BlogConfig) error {
	// Parse template file
	t, err := template.ParseFiles(config.ArchetypeFile)
	if err != nil {
		return fmt.Errorf("error parsing archetype file: %s", err)
	}

	// Generate markdown content
	buffer := &bytes.Buffer{}
	GenerateContent(buffer, blocks, config)

	// Dump markdown content into output according to archetype file
	fileArchetype := MakeArchetypeFields(page, config)
	fileArchetype.Content = buffer.String()
	err = t.Execute(w, fileArchetype)
	if err != nil {
		return fmt.Errorf("error filling archetype file: %s", err)
	}

	return nil
}

func GenerateContent(w io.Writer, blocks []notionapi.Block, config BlogConfig, prefixes ...string) {
	if len(blocks) == 0 {
		return
	}

	numberedList := false
	bulletedList := false

	for _, block := range blocks {
		// Add line break after list is finished
		if bulletedList && block.GetType() != notionapi.BlockTypeBulletedListItem {
			bulletedList = false
			fmt.Fprintln(w)
		}
		if numberedList && block.GetType() != notionapi.BlockTypeNumberedListItem {
			numberedList = false
			fmt.Fprintln(w)
		}

		switch b := block.(type) {
		case *notionapi.ParagraphBlock:
			fprintln(w, prefixes, ConvertRichText(b.Paragraph.Text)+"\n")
			GenerateContent(w, b.Paragraph.Children, config)
		case *notionapi.Heading1Block:
			fprintf(w, prefixes, "# %s", ConvertRichText(b.Heading1.Text))
		case *notionapi.Heading2Block:
			fprintf(w, prefixes, "## %s", ConvertRichText(b.Heading2.Text))
		case *notionapi.Heading3Block:
			fprintf(w, prefixes, "### %s", ConvertRichText(b.Heading3.Text))
		case *notionapi.CalloutBlock:
			if !config.UseShortcodes {
				continue
			}
			if b.Callout.Icon != nil {
				if b.Callout.Icon.Emoji != nil {
					fprintf(w, prefixes, `{{%% callout emoji="%s" %%}}`, *b.Callout.Icon.Emoji)
				} else {
					fprintf(w, prefixes, `{{%% callout image="%s" %%}}`, b.Callout.Icon.GetURL())
				}
			}
			fprintln(w, prefixes, ConvertRichText(b.Callout.Text))
			GenerateContent(w, b.Callout.Children, config, prefixes...)
			fprintln(w, prefixes, "{{% /callout %}}")

		case *notionapi.BookmarkBlock:
			if !config.UseShortcodes {
				// Simply generate the url link
				fprintf(w, prefixes, "[%s](%s)", b.Bookmark.URL, b.Bookmark.URL)
				continue
			}
			// Parse external page metadata
			og, err := parseMetadata(b.Bookmark.URL)
			if err != nil {
				log.Println("error getting bookmark metadata:", err)
			}

			// GenerateContent shortcode with given metadata
			fprintf(w, prefixes,
				`{{< bookmark url="%s" title="%s" description="%s" img="%s" >}}`,
				og.URL,
				og.Title,
				og.Description,
				og.Image,
			)

		case *notionapi.QuoteBlock:
			fprintf(w, prefixes, "> %s", ConvertRichText(b.Quote.Text))
			GenerateContent(w, b.Quote.Children, config,
				append([]string{"> "}, prefixes...)...)

		case *notionapi.BulletedListItemBlock:
			bulletedList = true
			fprintf(w, prefixes, "- %s", ConvertRichText(b.BulletedListItem.Text))
			GenerateContent(w, b.BulletedListItem.Children, config,
				append([]string{"    "}, prefixes...)...)

		case *notionapi.NumberedListItemBlock:
			numberedList = true
			fprintf(w, prefixes, "1. %s", ConvertRichText(b.NumberedListItem.Text))
			GenerateContent(w, b.NumberedListItem.Children, config,
				append([]string{"    "}, prefixes...)...)

		case *notionapi.ImageBlock:
			src, err := getImage(b.Image.File.URL, config)
			if err != nil {
				log.Println("error getting image:", err)
			}
			fprintf(w, prefixes, "![%s](%s)\n", ConvertRichText(b.Image.Caption), src)

		case *notionapi.CodeBlock:
			if b.Code.Language == "plain text" {
				fprintln(w, prefixes, "```")
			} else {
				fprintf(w, prefixes, "```%s", b.Code.Language)
			}
			fprintln(w, prefixes, ConvertRichText(b.Code.Text))
			fprintln(w, prefixes, "```")

		case *notionapi.UnsupportedBlock:
			if b.GetType() != "unsupported" {
				log.Println("unimplemented", block.GetType())
			} else {
				log.Println("unsupported block type")
			}
		default:
			log.Println("unimplemented", block.GetType())
		}
	}
}
