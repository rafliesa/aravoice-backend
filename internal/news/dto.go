package news

type CreateNewsRequest struct {
	Slug        string   `json:"slug"`
	Category    string   `json:"category"`
	Title       string   `json:"title"`
	Excerpt     string   `json:"excerpt"`
	Body        string   `json:"body"`
	Author      string   `json:"author"`
	ReadingTime int      `json:"reading_time"`
	CoverImage  string   `json:"cover_image"`
	Caption     string   `json:"caption"`
	Formats     []string `json:"formats"`
	PublishedAt string   `json:"published_at"`
	IsPublished bool     `json:"is_published"`
}

type NewsResponse struct {
	ID          int      `json:"id"`
	Slug        string   `json:"slug"`
	Category    string   `json:"category"`
	Title       string   `json:"title"`
	Excerpt     string   `json:"excerpt"`
	Body        string   `json:"body"`
	Author      string   `json:"author"`
	ReadingTime int      `json:"reading_time"`
	CoverImage  string   `json:"cover_image"`
	Caption     string   `json:"caption"`
	Formats     []string `json:"formats"`
	PublishedAt string   `json:"published_at"`
	IsPublished bool     `json:"is_published"`
}

func ToNewsResponse(n *News) *NewsResponse {
	if n == nil {
		return nil
	}

	return &NewsResponse{
		ID:          n.Id,
		Slug:        n.Slug,
		Category:    n.Category,
		Title:       n.Title,
		Excerpt:     n.Excerpt,
		Body:        n.Body,
		Author:      n.Author,
		ReadingTime: n.ReadingTime,
		CoverImage:  n.CoverImage,
		Caption:     n.Caption,
		Formats:     n.Formats,
		PublishedAt: n.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
		IsPublished: n.IsPublished,
	}
}

func ToNewsResponses(newsList []*News) []*NewsResponse {
	if newsList == nil {
		return nil
	}

	responses := make([]*NewsResponse, len(newsList))
	for i, n := range newsList {
		responses[i] = ToNewsResponse(n)
	}
	return responses
}
