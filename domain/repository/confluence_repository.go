package repository

import (
	"context"
	"fmt"

	goconfluence "github.com/virtomize/confluence-go-api"
)

type ConfluenceRepository struct {
	ansectorID string
	spaceKey   string
	client     *goconfluence.API
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
	}, nil
}

// ExportPostMortem(context.Context, string) error

func (c *ConfluenceRepository) ExportPostMortem(ctx context.Context, title, body string) error {
	data := &goconfluence.Content{
		Type:  "page",
		Title: title,
		Body: goconfluence.Body{
			Storage: goconfluence.Storage{
				Value:          body,
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

	_, err := c.client.CreateContent(data)
	if err != nil {
		return fmt.Errorf("failed to create confluence page: %w", err)
	}

	return err
}
