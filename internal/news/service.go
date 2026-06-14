package news

import (
	"context"
	"strings"
	"time"
)

type Service interface {
	Create(ctx context.Context, request CreateNewsRequest) (*NewsResponse, error)
	GetAll(ctx context.Context) ([]*NewsResponse, error)
	GetCards(ctx context.Context, page int, limit int) (*PaginatedNewsCardsResponse, error)
	GetByID(ctx context.Context, id int) (*NewsResponse, error)
	GetBySlug(ctx context.Context, slug string) (*NewsResponse, error)
	GetByTitle(ctx context.Context, title string) ([]*NewsResponse, error)
	Delete(ctx context.Context, id int) error
}

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) Create(ctx context.Context, request CreateNewsRequest) (*NewsResponse, error) {
	request.Slug = strings.TrimSpace(request.Slug)
	request.Category = strings.TrimSpace(request.Category)
	request.Title = strings.TrimSpace(request.Title)
	request.Excerpt = strings.TrimSpace(request.Excerpt)
	request.Body = strings.TrimSpace(request.Body)
	request.Author = strings.TrimSpace(request.Author)
	request.CoverImage = strings.TrimSpace(request.CoverImage)
	request.Caption = strings.TrimSpace(request.Caption)
	request.PublishedAt = strings.TrimSpace(request.PublishedAt)

	sanitizedBody, err := sanitizeNewsHTML(request.Body)
	if err != nil {
		return nil, &InvalidInputError{Message: "body contains invalid HTML"}
	}
	request.Body = sanitizedBody

	if err := validateCreateRequest(request); err != nil {
		return nil, err
	}

	publishedAt, err := time.Parse(time.RFC3339, request.PublishedAt)
	if err != nil {
		return nil, &InvalidInputError{
			Message: "published_at must use RFC3339 format",
		}
	}

	createdNews, err := s.repository.Create(ctx, CreateNewsInput{
		Slug:        request.Slug,
		Category:    request.Category,
		Title:       request.Title,
		Excerpt:     request.Excerpt,
		Body:        request.Body,
		Author:      request.Author,
		ReadingTime: request.ReadingTime,
		CoverImage:  request.CoverImage,
		Caption:     request.Caption,
		Formats:     normalizeFormats(request.Formats),
		PublishedAt: publishedAt,
		IsPublished: request.IsPublished,
	})
	if err != nil {
		return nil, err
	}

	return ToNewsResponse(createdNews), nil
}

func (s *service) GetAll(ctx context.Context) ([]*NewsResponse, error) {
	newsList, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return ToNewsResponses(newsList), nil
}

func (s *service) GetCards(
	ctx context.Context,
	page int,
	limit int,
) (*PaginatedNewsCardsResponse, error) {
	if page <= 0 {
		return nil, &InvalidInputError{Message: "page must be greater than zero"}
	}
	if limit <= 0 || limit > 50 {
		return nil, &InvalidInputError{Message: "limit must be between 1 and 50"}
	}

	newsList, total, err := s.repository.FindPublishedCards(ctx, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return &PaginatedNewsCardsResponse{
		Data: ToNewsCardResponses(newsList),
		Pagination: PaginationResponse{
			Page:       page,
			Limit:      limit,
			TotalItems: total,
			TotalPages: totalPages,
		},
	}, nil
}

func (s *service) GetByID(ctx context.Context, id int) (*NewsResponse, error) {
	if id <= 0 {
		return nil, &InvalidInputError{Message: "id must be greater than zero"}
	}

	n, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return ToNewsResponse(n), nil
}

func (s *service) GetBySlug(ctx context.Context, slug string) (*NewsResponse, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, &InvalidInputError{Message: "slug is required"}
	}

	n, err := s.repository.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return ToNewsResponse(n), nil
}

func (s *service) GetByTitle(ctx context.Context, title string) ([]*NewsResponse, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, &InvalidInputError{Message: "title is required"}
	}

	newsList, err := s.repository.FindByTitle(ctx, title)
	if err != nil {
		return nil, err
	}

	return ToNewsResponses(newsList), nil
}

func (s *service) Delete(ctx context.Context, id int) error {
	if id <= 0 {
		return &InvalidInputError{Message: "id must be greater than zero"}
	}

	return s.repository.Delete(ctx, id)
}

func validateCreateRequest(request CreateNewsRequest) error {
	requiredFields := []struct {
		name  string
		value string
	}{
		{name: "slug", value: request.Slug},
		{name: "category", value: request.Category},
		{name: "title", value: request.Title},
		{name: "body", value: request.Body},
		{name: "author", value: request.Author},
		{name: "published_at", value: request.PublishedAt},
	}

	for _, field := range requiredFields {
		if field.value == "" {
			return &InvalidInputError{Message: field.name + " is required"}
		}
	}

	if request.ReadingTime < 0 {
		return &InvalidInputError{Message: "reading_time cannot be negative"}
	}

	return nil
}

func normalizeFormats(formats []string) []string {
	normalized := make([]string, 0, len(formats))
	for _, format := range formats {
		format = strings.TrimSpace(format)
		if format != "" {
			normalized = append(normalized, format)
		}
	}
	return normalized
}
