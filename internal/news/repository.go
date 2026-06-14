package news

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, input CreateNewsInput) (*News, error)
	FindAll(ctx context.Context) ([]*News, error)
	FindPublishedCards(ctx context.Context, category string, limit int, offset int) ([]*NewsCard, int, error)
	FindByID(ctx context.Context, id int) (*News, error)
	FindBySlug(ctx context.Context, slug string) (*News, error)
	FindByTitle(ctx context.Context, title string) ([]*News, error)
	Delete(ctx context.Context, id int) error
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

const newsColumns = `
	id,
	slug,
	category,
	title,
	excerpt,
	body,
	author,
	reading_time,
	cover_image,
	caption,
	formats,
	published_at,
	is_published,
	created_at,
	updated_at
`

func (r *repository) Create(ctx context.Context, input CreateNewsInput) (*News, error) {
	const query = `
		INSERT INTO news (
			slug,
			category,
			title,
			excerpt,
			body,
			author,
			reading_time,
			cover_image,
			caption,
			formats,
			published_at,
			is_published
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12
		)
		RETURNING ` + newsColumns

	n, err := scanNews(r.db.QueryRow(
		ctx,
		query,
		input.Slug,
		input.Category,
		input.Title,
		input.Excerpt,
		input.Body,
		input.Author,
		input.ReadingTime,
		input.CoverImage,
		input.Caption,
		input.Formats,
		input.PublishedAt,
		input.IsPublished,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrNewsSlugExists
		}
		return nil, fmt.Errorf("create news: %w", err)
	}

	return n, nil
}

func (r *repository) FindAll(ctx context.Context) ([]*News, error) {
	const query = `
		SELECT ` + newsColumns + `
		FROM news
		ORDER BY published_at DESC, id DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find all news: %w", err)
	}
	defer rows.Close()

	newsList := make([]*News, 0)
	for rows.Next() {
		n, err := scanNews(rows)
		if err != nil {
			return nil, fmt.Errorf("scan news: %w", err)
		}
		newsList = append(newsList, n)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news: %w", err)
	}

	return newsList, nil
}

func (r *repository) FindPublishedCards(
	ctx context.Context,
	category string,
	limit int,
	offset int,
) ([]*NewsCard, int, error) {
	// An empty category means "all rubriks"; otherwise filter case-insensitively.
	countQuery := `SELECT COUNT(*) FROM news WHERE is_published = TRUE`
	countArgs := []any{}
	if category != "" {
		countQuery += ` AND LOWER(category) = LOWER($1)`
		countArgs = append(countArgs, category)
	}

	var total int
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count published news cards: %w", err)
	}

	query := `
		SELECT
			id,
			slug,
			category,
			title,
			excerpt,
			author,
			reading_time,
			cover_image,
			caption,
			formats,
			published_at
		FROM news
		WHERE is_published = TRUE
	`
	args := []any{}
	if category != "" {
		args = append(args, category)
		query += fmt.Sprintf(" AND LOWER(category) = LOWER($%d)", len(args))
	}
	args = append(args, limit)
	limitPos := len(args)
	args = append(args, offset)
	offsetPos := len(args)
	query += fmt.Sprintf(
		" ORDER BY published_at DESC, id DESC LIMIT $%d OFFSET $%d",
		limitPos,
		offsetPos,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("find published news cards: %w", err)
	}
	defer rows.Close()

	newsList := make([]*NewsCard, 0)
	for rows.Next() {
		var n NewsCard
		if err := rows.Scan(
			&n.Id,
			&n.Slug,
			&n.Category,
			&n.Title,
			&n.Excerpt,
			&n.Author,
			&n.ReadingTime,
			&n.CoverImage,
			&n.Caption,
			&n.Formats,
			&n.PublishedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan published news card: %w", err)
		}
		newsList = append(newsList, &n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate published news cards: %w", err)
	}

	return newsList, total, nil
}

func (r *repository) FindByID(ctx context.Context, id int) (*News, error) {
	const query = `
		SELECT ` + newsColumns + `
		FROM news
		WHERE id = $1
	`

	n, err := scanNews(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNewsNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find news by id: %w", err)
	}

	return n, nil
}

func (r *repository) FindBySlug(ctx context.Context, slug string) (*News, error) {
	const query = `
		SELECT ` + newsColumns + `
		FROM news
		WHERE slug = $1
	`

	n, err := scanNews(r.db.QueryRow(ctx, query, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNewsNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find news by slug: %w", err)
	}

	return n, nil
}

func (r *repository) FindByTitle(ctx context.Context, title string) ([]*News, error) {
	const query = `
		SELECT ` + newsColumns + `
		FROM news
		WHERE title ILIKE '%' || $1 || '%'
		ORDER BY published_at DESC, id DESC
	`

	rows, err := r.db.Query(ctx, query, title)
	if err != nil {
		return nil, fmt.Errorf("find news by title: %w", err)
	}
	defer rows.Close()

	newsList := make([]*News, 0)
	for rows.Next() {
		n, err := scanNews(rows)
		if err != nil {
			return nil, fmt.Errorf("scan news by title: %w", err)
		}
		newsList = append(newsList, n)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news by title: %w", err)
	}

	return newsList, nil
}

func (r *repository) Delete(ctx context.Context, id int) error {
	commandTag, err := r.db.Exec(ctx, `DELETE FROM news WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete news: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNewsNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNews(row rowScanner) (*News, error) {
	var n News
	if err := row.Scan(
		&n.Id,
		&n.Slug,
		&n.Category,
		&n.Title,
		&n.Excerpt,
		&n.Body,
		&n.Author,
		&n.ReadingTime,
		&n.CoverImage,
		&n.Caption,
		&n.Formats,
		&n.PublishedAt,
		&n.IsPublished,
		&n.CreatedAt,
		&n.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &n, nil
}
