package news

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run repository integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(adminPool.Close)

	schema := fmt.Sprintf("news_test_%d", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := adminPool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database config: %v", err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+quotedSchema)
		return err
	}

	testPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("create test connection pool: %v", err)
	}
	t.Cleanup(testPool.Close)

	migration, err := os.ReadFile("../../migrations/000001_create_news_table.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := testPool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	repository := NewRepository(testPool)
	first := createTestNews(t, ctx, repository, CreateNewsInput{
		Slug:        "ara-voice-update",
		Category:    "Technology",
		Title:       "Ara Voice Update",
		Excerpt:     "Latest update",
		Body:        "Content",
		Author:      "Ara Voice",
		ReadingTime: 5,
		CoverImage:  "cover.jpg",
		Caption:     "Cover",
		Formats:     []string{"article", "audio"},
		PublishedAt: time.Date(2026, time.June, 13, 10, 0, 0, 0, time.UTC),
		IsPublished: true,
	})
	second := createTestNews(t, ctx, repository, CreateNewsInput{
		Slug:        "inside-aravoice",
		Category:    "Company",
		Title:       "Inside Aravoice",
		Excerpt:     "Behind the scenes",
		Body:        "Content",
		Author:      "Ara Voice",
		ReadingTime: 3,
		CoverImage:  "inside.jpg",
		Caption:     "Office",
		Formats:     []string{},
		PublishedAt: time.Date(2026, time.June, 12, 10, 0, 0, 0, time.UTC),
		IsPublished: false,
	})

	byID, err := repository.FindByID(ctx, first.Id)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if byID.Slug != first.Slug || byID.Title != first.Title {
		t.Fatalf("unexpected news from FindByID: %#v", byID)
	}

	bySlug, err := repository.FindBySlug(ctx, second.Slug)
	if err != nil {
		t.Fatalf("FindBySlug returned error: %v", err)
	}
	if bySlug.Id != second.Id {
		t.Fatalf("unexpected news from FindBySlug: %#v", bySlug)
	}

	byTitle, err := repository.FindByTitle(ctx, "VOICE")
	if err != nil {
		t.Fatalf("FindByTitle returned error: %v", err)
	}
	if len(byTitle) != 2 {
		t.Fatalf("expected 2 title matches, got %d", len(byTitle))
	}

	all, err := repository.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 2 || all[0].Id != first.Id || all[1].Id != second.Id {
		t.Fatalf("unexpected FindAll order: %#v", all)
	}

	cards, total, err := repository.FindPublishedCards(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("FindPublishedCards returned error: %v", err)
	}
	if total != 1 || len(cards) != 1 || cards[0].Id != first.Id {
		t.Fatalf("unexpected published cards: total=%d cards=%#v", total, cards)
	}

	filteredCards, filteredTotal, err := repository.FindPublishedCards(
		ctx,
		"technology",
		10,
		0,
	)
	if err != nil {
		t.Fatalf("FindPublishedCards with category returned error: %v", err)
	}
	if filteredTotal != 1 || len(filteredCards) != 1 || filteredCards[0].Id != first.Id {
		t.Fatalf(
			"unexpected category-filtered cards: total=%d cards=%#v",
			filteredTotal,
			filteredCards,
		)
	}

	filteredCards, filteredTotal, err = repository.FindPublishedCards(
		ctx,
		"missing",
		10,
		0,
	)
	if err != nil {
		t.Fatalf("FindPublishedCards with missing category returned error: %v", err)
	}
	if filteredTotal != 0 || len(filteredCards) != 0 {
		t.Fatalf(
			"expected no cards for missing category: total=%d cards=%#v",
			filteredTotal,
			filteredCards,
		)
	}

	_, err = repository.Create(ctx, CreateNewsInput{
		Slug:        first.Slug,
		Category:    "Technology",
		Title:       "Duplicate",
		Excerpt:     "",
		Body:        "Content",
		Author:      "Ara Voice",
		ReadingTime: 1,
		CoverImage:  "",
		Caption:     "",
		Formats:     []string{},
		PublishedAt: time.Now(),
	})
	if !errors.Is(err, ErrNewsSlugExists) {
		t.Fatalf("expected ErrNewsSlugExists, got %v", err)
	}

	_, err = repository.FindByID(ctx, first.Id+second.Id+1000)
	if !errors.Is(err, ErrNewsNotFound) {
		t.Fatalf("expected ErrNewsNotFound, got %v", err)
	}

	if err := repository.Delete(ctx, second.Id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	_, err = repository.FindByID(ctx, second.Id)
	if !errors.Is(err, ErrNewsNotFound) {
		t.Fatalf("expected deleted news to be missing, got %v", err)
	}
	if err := repository.Delete(ctx, second.Id); !errors.Is(err, ErrNewsNotFound) {
		t.Fatalf("expected ErrNewsNotFound when deleting twice, got %v", err)
	}
}

func createTestNews(
	t *testing.T,
	ctx context.Context,
	repository Repository,
	input CreateNewsInput,
) *News {
	t.Helper()

	n, err := repository.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	return n
}
