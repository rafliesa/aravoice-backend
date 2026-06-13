package news

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type serviceStub struct {
	createFn     func(context.Context, CreateNewsRequest) (*NewsResponse, error)
	getAllFn     func(context.Context) ([]*NewsResponse, error)
	getCardsFn   func(context.Context, int, int) (*PaginatedNewsCardsResponse, error)
	getByIDFn    func(context.Context, int) (*NewsResponse, error)
	getBySlugFn  func(context.Context, string) (*NewsResponse, error)
	getByTitleFn func(context.Context, string) ([]*NewsResponse, error)
	deleteFn     func(context.Context, int) error
}

func (s *serviceStub) Create(ctx context.Context, request CreateNewsRequest) (*NewsResponse, error) {
	return s.createFn(ctx, request)
}

func (s *serviceStub) GetAll(ctx context.Context) ([]*NewsResponse, error) {
	return s.getAllFn(ctx)
}

func (s *serviceStub) GetCards(
	ctx context.Context,
	page int,
	limit int,
) (*PaginatedNewsCardsResponse, error) {
	return s.getCardsFn(ctx, page, limit)
}

func (s *serviceStub) GetByID(ctx context.Context, id int) (*NewsResponse, error) {
	return s.getByIDFn(ctx, id)
}

func (s *serviceStub) GetBySlug(ctx context.Context, slug string) (*NewsResponse, error) {
	return s.getBySlugFn(ctx, slug)
}

func (s *serviceStub) GetByTitle(ctx context.Context, title string) ([]*NewsResponse, error) {
	return s.getByTitleFn(ctx, title)
}

func (s *serviceStub) Delete(ctx context.Context, id int) error {
	return s.deleteFn(ctx, id)
}

func TestHandlerCreate(t *testing.T) {
	service := &serviceStub{
		createFn: func(_ context.Context, request CreateNewsRequest) (*NewsResponse, error) {
			return &NewsResponse{ID: 1, Slug: request.Slug}, nil
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/news",
		strings.NewReader(`{
			"slug": "hello-world",
			"category": "Technology",
			"title": "Hello World",
			"body": "Content",
			"author": "Author",
			"published_at": "2026-06-13T10:00:00Z"
		}`),
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
}

func TestHandlerGetBySlugNotFound(t *testing.T) {
	service := &serviceStub{
		getBySlugFn: func(context.Context, string) (*NewsResponse, error) {
			return nil, ErrNewsNotFound
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/news/slug/missing", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestHandlerCreateReturnsConflictForDuplicateSlug(t *testing.T) {
	service := &serviceStub{
		createFn: func(context.Context, CreateNewsRequest) (*NewsResponse, error) {
			return nil, ErrNewsSlugExists
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/news",
		strings.NewReader(`{"slug":"duplicate"}`),
	)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), ErrNewsSlugExists.Error()) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestHandlerReturnsInternalServerError(t *testing.T) {
	service := &serviceStub{
		getAllFn: func(context.Context) ([]*NewsResponse, error) {
			return nil, errors.New("database unavailable")
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/news", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
}

func TestHandlerGetByTitle(t *testing.T) {
	service := &serviceStub{
		getByTitleFn: func(_ context.Context, title string) ([]*NewsResponse, error) {
			if title != "Ara Voice" {
				t.Fatalf("unexpected title: %q", title)
			}
			return []*NewsResponse{{ID: 1, Title: "Ara Voice Update"}}, nil
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/news/search?title=Ara+Voice", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Ara Voice Update") {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
}

func TestHandlerGetByTitleRequiresQuery(t *testing.T) {
	service := &serviceStub{
		getByTitleFn: func(context.Context, string) ([]*NewsResponse, error) {
			return nil, &InvalidInputError{Message: "title is required"}
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/news/search", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestHandlerGetCards(t *testing.T) {
	service := &serviceStub{
		getCardsFn: func(_ context.Context, page int, limit int) (*PaginatedNewsCardsResponse, error) {
			if page != 2 || limit != 4 {
				t.Fatalf("unexpected pagination: page=%d limit=%d", page, limit)
			}
			return &PaginatedNewsCardsResponse{
				Data: []*NewsCardResponse{
					{ID: 7, Title: "Paginated news"},
				},
				Pagination: PaginationResponse{
					Page:       2,
					Limit:      4,
					TotalItems: 10,
					TotalPages: 3,
				},
			}, nil
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/news/cards?page=2&limit=4", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"total_pages":3`) {
		t.Fatalf("unexpected response body: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"body"`) {
		t.Fatalf("card response must not include body: %s", recorder.Body.String())
	}
}

func TestHandlerGetCardsRejectsInvalidPagination(t *testing.T) {
	mux := http.NewServeMux()
	NewHandler(&serviceStub{}).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/news/cards?page=0", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerDelete(t *testing.T) {
	service := &serviceStub{
		deleteFn: func(_ context.Context, id int) error {
			if id != 42 {
				t.Fatalf("unexpected id: %d", id)
			}
			return nil
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/news/42", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerDeleteNotFound(t *testing.T) {
	service := &serviceStub{
		deleteFn: func(context.Context, int) error {
			return ErrNewsNotFound
		},
	}
	mux := http.NewServeMux()
	NewHandler(service).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodDelete, "/news/999", nil)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
