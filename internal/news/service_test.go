package news

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repositoryStub struct {
	createFn      func(context.Context, CreateNewsInput) (*News, error)
	findAllFn     func(context.Context) ([]*News, error)
	findByIDFn    func(context.Context, int) (*News, error)
	findBySlugFn  func(context.Context, string) (*News, error)
	findByTitleFn func(context.Context, string) ([]*News, error)
	deleteFn      func(context.Context, int) error
}

func (r *repositoryStub) Create(ctx context.Context, input CreateNewsInput) (*News, error) {
	return r.createFn(ctx, input)
}

func (r *repositoryStub) FindAll(ctx context.Context) ([]*News, error) {
	return r.findAllFn(ctx)
}

func (r *repositoryStub) FindByID(ctx context.Context, id int) (*News, error) {
	return r.findByIDFn(ctx, id)
}

func (r *repositoryStub) FindBySlug(ctx context.Context, slug string) (*News, error) {
	return r.findBySlugFn(ctx, slug)
}

func (r *repositoryStub) FindByTitle(ctx context.Context, title string) ([]*News, error) {
	return r.findByTitleFn(ctx, title)
}

func (r *repositoryStub) Delete(ctx context.Context, id int) error {
	return r.deleteFn(ctx, id)
}

func TestServiceCreate(t *testing.T) {
	repository := &repositoryStub{
		createFn: func(_ context.Context, input CreateNewsInput) (*News, error) {
			if input.Slug != "hello-world" {
				t.Fatalf("unexpected slug: %q", input.Slug)
			}
			if input.PublishedAt.Location() != time.UTC {
				t.Fatalf("unexpected timezone: %v", input.PublishedAt.Location())
			}
			if len(input.Formats) != 1 || input.Formats[0] != "article" {
				t.Fatalf("unexpected formats: %#v", input.Formats)
			}

			return &News{
				Id:          1,
				Slug:        input.Slug,
				Category:    input.Category,
				Title:       input.Title,
				Body:        input.Body,
				Author:      input.Author,
				Formats:     input.Formats,
				PublishedAt: input.PublishedAt,
			}, nil
		},
	}

	service := NewService(repository)
	response, err := service.Create(context.Background(), CreateNewsRequest{
		Slug:        " hello-world ",
		Category:    "Technology",
		Title:       "Hello World",
		Body:        "Content",
		Author:      "Author",
		Formats:     []string{" article ", ""},
		PublishedAt: "2026-06-13T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if response.ID != 1 {
		t.Fatalf("unexpected response ID: %d", response.ID)
	}
}

func TestServiceCreateRejectsInvalidPublishedAt(t *testing.T) {
	service := NewService(&repositoryStub{})

	_, err := service.Create(context.Background(), CreateNewsRequest{
		Slug:        "hello-world",
		Category:    "Technology",
		Title:       "Hello World",
		Body:        "Content",
		Author:      "Author",
		PublishedAt: "13-06-2026",
	})
	if !errors.Is(err, ErrInvalidNewsInput) {
		t.Fatalf("expected ErrInvalidNewsInput, got %v", err)
	}
}

func TestServiceGetByTitle(t *testing.T) {
	repository := &repositoryStub{
		findByTitleFn: func(_ context.Context, title string) ([]*News, error) {
			if title != "Ara Voice" {
				t.Fatalf("unexpected title: %q", title)
			}
			return []*News{
				{Id: 1, Title: "Ara Voice Update"},
				{Id: 2, Title: "Inside Ara Voice"},
			}, nil
		},
	}

	service := NewService(repository)
	response, err := service.GetByTitle(context.Background(), " Ara Voice ")
	if err != nil {
		t.Fatalf("GetByTitle returned error: %v", err)
	}
	if len(response) != 2 {
		t.Fatalf("expected 2 results, got %d", len(response))
	}
}

func TestServiceGetByTitleRejectsEmptyTitle(t *testing.T) {
	service := NewService(&repositoryStub{})

	_, err := service.GetByTitle(context.Background(), " ")
	if !errors.Is(err, ErrInvalidNewsInput) {
		t.Fatalf("expected ErrInvalidNewsInput, got %v", err)
	}
}

func TestServiceDelete(t *testing.T) {
	repository := &repositoryStub{
		deleteFn: func(_ context.Context, id int) error {
			if id != 7 {
				t.Fatalf("unexpected id: %d", id)
			}
			return nil
		},
	}

	if err := NewService(repository).Delete(context.Background(), 7); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestServiceDeleteRejectsInvalidID(t *testing.T) {
	err := NewService(&repositoryStub{}).Delete(context.Background(), 0)
	if !errors.Is(err, ErrInvalidNewsInput) {
		t.Fatalf("expected ErrInvalidNewsInput, got %v", err)
	}
}
