package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	translator "notion2md/pkg"

	"github.com/janeczku/go-spinner"
	"github.com/jomei/notionapi"
)

func filterFromConfig(config translator.TransferConfig) *notionapi.OrCompoundFilter {
	if config.FilterProp == "" || len(config.FilterValue) == 0 {
		return nil
	}

	properties := make(notionapi.OrCompoundFilter, len(config.FilterValue))

	for i, val := range config.FilterValue {
		properties[i] = notionapi.PropertyFilter{
			Property: config.FilterProp,
			Status: &notionapi.StatusFilterCondition{
				Equals: val,
			},
		}
	}

	return &properties
}

func generateArticleName(title string, date time.Time, config translator.TransferConfig) string {

	escapedTitle := strings.ReplaceAll(
		strings.ToValidUTF8(
			strings.ToLower(title),
			"",
		),
		" ", "_",
	)
	escapedFilename := escapedTitle + ".md"

	if config.UseDateForFilename {
		// Add date to the name to allow repeated titles
		return date.Format("2006-01-02") + escapedFilename
	}

	if strings.Contains(escapedFilename, "_index") {
		escapedFilename = "_index.md"
	}

	return escapedFilename
}

// chageStatus changes the Notion article status to the published value if set.
// It returns true if status changed.
func changeStatus(client *notionapi.Client, p notionapi.Page, config translator.TransferConfig) bool {
	// No published value or filter prop to change
	if config.FilterProp == "" || config.PublishedValue == "" {
		return false
	}

	if v, ok := p.Properties[config.FilterProp]; ok {
		if status, ok := v.(*notionapi.StatusProperty); ok {
			// Already has published value
			if status.Status.Name == config.PublishedValue {
				return false
			}
		} else { // Filter prop is not a select property
			return false
		}
	} else { // No filter prop in page, can't change it
		return false
	}

	updatedProps := make(notionapi.Properties)
	updatedProps[config.FilterProp] = notionapi.StatusProperty{
		Status: notionapi.Status{
			Name: config.PublishedValue,
			ID: nil,
		},
	}

	_, err := client.Page.Update(context.Background(), notionapi.PageID(p.ID),
		&notionapi.PageUpdateRequest{
			Properties: updatedProps,
		},
	)
	if err != nil {
		log.Println("error changing status:", err)
	}

	return err == nil
}

func recursiveGetChildren(client *notionapi.Client, blockID notionapi.BlockID) (blocks []notionapi.Block, err error) {
	res, err := client.Block.GetChildren(context.Background(), blockID, &notionapi.Pagination{
		PageSize: 100,
	})
	if err != nil {
		return nil, err
	}

	blocks = res.Results
	if len(blocks) == 0 {
		return
	}

	for _, block := range blocks {
		switch b := block.(type) {
		case *notionapi.ParagraphBlock:
			b.Paragraph.Children, err = recursiveGetChildren(client, b.ID)
		case *notionapi.CalloutBlock:
			b.Callout.Children, err = recursiveGetChildren(client, b.ID)
		case *notionapi.QuoteBlock:
			b.Quote.Children, err = recursiveGetChildren(client, b.ID)
		case *notionapi.BulletedListItemBlock:
			b.BulletedListItem.Children, err = recursiveGetChildren(client, b.ID)
		case *notionapi.NumberedListItemBlock:
			b.NumberedListItem.Children, err = recursiveGetChildren(client, b.ID)
		case *notionapi.TableBlock:
			b.Table.Children, err = recursiveGetChildren(client, b.ID)
		}

		if err != nil {
			return
		}
	}

	return
}

func ParseAndGenerate(config translator.TransferConfig) error {
	client := notionapi.NewClient(notionapi.Token(os.Getenv("NOTION_SECRET")))

	spin := spinner.StartNew("Querying Notion database")
	q, err := client.Database.Query(context.Background(), notionapi.DatabaseID(config.DatabaseID),
		&notionapi.DatabaseQueryRequest{
			Filter:   filterFromConfig(config),
			PageSize: 100,
		})
	spin.Stop()
	if err != nil {
		return fmt.Errorf("❌ Querying Notion database: %s", err)
	}
	fmt.Println("✔ Querying Notion database: Completed")

	err = os.MkdirAll(config.ContentFolder, 0777)
	if err != nil {
		return fmt.Errorf("couldn't create content folder: %s", err)
	}

	// number of article status changed
	changed := 0

	configMap := config.CategoryMap
	if err != nil {
		return fmt.Errorf("parsing config error: %s", err.Error())
	}

	for i, res := range q.Results {
		title := translator.ConvertRichText(res.Properties["Name"].(*notionapi.TitleProperty).Title)
		categoryName := res.Properties["Category"].(*notionapi.SelectProperty).Select.Name
		category := ""
		if categoryName != "首页" && categoryName != "_index" {
			category = strings.ToLower(configMap[categoryName])
		}

		fmt.Printf("-- Article [%d/%d] --\n", i+1, len(q.Results))
		spin = spinner.StartNew("Getting blocks tree")
		// Get page blocks tree
		blocks, err := recursiveGetChildren(client, notionapi.BlockID(res.ID))
		spin.Stop()
		if err != nil {
			log.Println("❌ Getting blocks tree:", err)
			continue
		}
		fmt.Println("✔ Getting blocks tree: Completed")

		folder := filepath.Join(config.ContentFolder, category)
		err = os.MkdirAll(folder, 0777)

		if err != nil {
			log.Println("❌ Creating category folder:", err)
			continue
		}

		filePath := filepath.Join(
			folder,
			generateArticleName(title, res.CreatedTime, config),
		)

		// Create file
		f, _ := os.Create(filePath)

		// Generate and dump content to file
		if err := translator.Generate(f, res, blocks, config); err != nil {
			fmt.Printf("❌ Generating blog %s: %s", res.URL, err)
			f.Close()
			continue
		}
		fmt.Printf("✔ Generating blog %s: Completed\n", title)

		// Change status of blog post if desired
		if changeStatus(client, res, config) {
			changed++
		}

		f.Close()
	}

	// Set GITHUB_ACTIONS info variables
	// https://docs.github.com/en/actions/learn-github-actions/workflow-commands-for-github-actions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Printf("::set-output name=articles_published::%d\n", changed)
	}

	return nil
}
