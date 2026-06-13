package news

import (
	"testing"
	"time"
)

func TestToNewsResponse(t *testing.T) {
	publishedAt := time.Date(2026, time.June, 13, 10, 30, 0, 0, time.FixedZone("WIB", 7*60*60))
	n := &News{
		Id:          1,
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
		PublishedAt: publishedAt,
		IsPublished: true,
	}

	response := ToNewsResponse(n)

	if response.ID != n.Id ||
		response.Slug != n.Slug ||
		response.Category != n.Category ||
		response.Title != n.Title ||
		response.Excerpt != n.Excerpt ||
		response.Body != n.Body ||
		response.Author != n.Author ||
		response.ReadingTime != n.ReadingTime ||
		response.CoverImage != n.CoverImage ||
		response.Caption != n.Caption ||
		response.IsPublished != n.IsPublished {
		t.Fatalf("response does not match news: %#v", response)
	}
	if response.PublishedAt != "2026-06-13T10:30:00+07:00" {
		t.Fatalf("unexpected published_at: %q", response.PublishedAt)
	}
	if len(response.Formats) != 2 || response.Formats[0] != "article" || response.Formats[1] != "audio" {
		t.Fatalf("unexpected formats: %#v", response.Formats)
	}
}

func TestToNewsResponseHandlesNil(t *testing.T) {
	if response := ToNewsResponse(nil); response != nil {
		t.Fatalf("expected nil response, got %#v", response)
	}
}

func TestToNewsResponses(t *testing.T) {
	response := ToNewsResponses([]*News{
		{Id: 1, Title: "First"},
		{Id: 2, Title: "Second"},
	})

	if len(response) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(response))
	}
	if response[0].ID != 1 || response[1].ID != 2 {
		t.Fatalf("unexpected responses: %#v", response)
	}

	if nilResponse := ToNewsResponses(nil); nilResponse != nil {
		t.Fatalf("expected nil response, got %#v", nilResponse)
	}
}
