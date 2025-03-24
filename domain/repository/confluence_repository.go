package repository

import (
	"context"
	"fmt"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	goconfluence "github.com/virtomize/confluence-go-api"
)

type ConfluenceRepository struct {
	ansectorID string
	spaceKey   string
	client     *goconfluence.API
	domain     string
}

func NewConfluenceRepository(domain, user, password, spaceKey, ancestorID string) (*ConfluenceRepository, error) {
	api, err := goconfluence.NewAPI(
		fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api", domain),
		user,
		password)
	if err != nil {
		return nil, fmt.Errorf("failed to create confluence api: %w", err)
	}

	return &ConfluenceRepository{
		ansectorID: ancestorID,
		spaceKey:   spaceKey,
		client:     api,
		domain:     domain,
	}, nil
}

func (c *ConfluenceRepository) ExportPostMortem(ctx context.Context, title, body string) (string, error) {
	renderer := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
		Flags: blackfriday.HrefTargetBlank,
	})
	output := blackfriday.Run([]byte(body), blackfriday.WithExtensions(blackfriday.HardLineBreak+blackfriday.Autolink), blackfriday.WithRenderer(renderer))
	html := bluemonday.UGCPolicy().SanitizeBytes(output)

	data := &goconfluence.Content{
		Type:  "page",
		Title: title,
		Body: goconfluence.Body{
			Storage: goconfluence.Storage{
				Value:          string(html),
				Representation: "storage",
			},
		},
		Version: &goconfluence.Version{ // mandatory
			Number: 1,
		},
	}
	if c.ansectorID != "" {
		data.Ancestors = append(data.Ancestors, goconfluence.Ancestor{
			ID: c.ansectorID,
		})
	}

	if c.spaceKey != "" {
		data.Space = &goconfluence.Space{
			Key: c.spaceKey,
		}
	}

	page, err := c.client.CreateContent(data)
	if err != nil {
		return "", fmt.Errorf("failed to create confluence page: %w", err)
	}

	return fmt.Sprintf("https://%s.atlassian.net/wiki%s", c.domain, page.Links.WebUI), nil
}
