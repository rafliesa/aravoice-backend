package news

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSanitizeNewsHTMLPreservesEditorFormatting(t *testing.T) {
	input := `<h2 style="text-align: center">Judul</h2>` +
		`<p style="text-align: justify"><span style="font-family: var(--font-libre-caslon-text); font-size: 24px; color: #AABBCC; font-weight: 700">Teks</span> <strong>tebal</strong> <em>miring</em> <u>garis</u></p>` +
		`<img src="/api/uploads/0123456789abcdef0123456789abcdef.png" alt="Foto" data-align="left" data-width="45%" style="width: 45%">` +
		`<audio src="/api/uploads/0123456789abcdef0123456789abcdef.mp3" controls autoplay></audio>` +
		`<video src="/api/uploads/0123456789abcdef0123456789abcdef.mp4" controls autoplay></video>`

	output, err := sanitizeNewsHTML(input)
	if err != nil {
		t.Fatal(err)
	}

	expectedParts := []string{
		`<h2 style="text-align: center">Judul</h2>`,
		`font-family: var(--font-libre-caslon-text)`,
		`font-size: 24px`,
		`color: #AABBCC`,
		`font-weight: 700`,
		`data-align="left"`,
		`data-width="45%"`,
		`<audio src="/api/uploads/0123456789abcdef0123456789abcdef.mp3" controls="" preload="metadata"></audio>`,
		`<video src="/api/uploads/0123456789abcdef0123456789abcdef.mp4" controls="" preload="metadata" playsinline=""></video>`,
	}
	for _, expected := range expectedParts {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %s", expected, output)
		}
	}
	if strings.Contains(output, "autoplay") {
		t.Fatalf("autoplay must be removed: %s", output)
	}
}

func TestSanitizeNewsHTMLRemovesUnsafeContent(t *testing.T) {
	input := `<script>alert(1)</script>` +
		`<p onclick="alert(1)" style="text-align:center;position:fixed;background:url(javascript:alert(1))">Safe</p>` +
		`<a href="javascript:alert(1)"><b>bad link</b></a>` +
		`<img src="data:image/svg+xml;base64,AAAA" onerror="alert(1)">` +
		`<iframe src="https://evil.example/embed/1"></iframe>` +
		`<span style="font-size:999px;color:red;font-weight:900">Styled</span>`

	output, err := sanitizeNewsHTML(input)
	if err != nil {
		t.Fatal(err)
	}

	for _, forbidden := range []string{"script", "onclick", "javascript:", "data:image", "iframe", "999px", "color:red", "900"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("unsafe value %q remained in %s", forbidden, output)
		}
	}
	if !strings.Contains(output, `<p style="text-align: center">Safe</p>`) {
		t.Fatalf("safe paragraph formatting was not preserved: %s", output)
	}
	if !strings.Contains(output, `<b>bad link</b>`) {
		t.Fatalf("invalid link should be unwrapped: %s", output)
	}
}

func TestSanitizeNewsHTMLPreservesInternalLinks(t *testing.T) {
	output, err := sanitizeNewsHTML(
		`<p><a href="/tentang-kami">Tentang</a> <a href="#bagian">Bagian</a></p>`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `href="/tentang-kami"`) ||
		!strings.Contains(output, `href="#bagian"`) {
		t.Fatalf("internal links were not preserved: %s", output)
	}
}

func TestSanitizeNewsHTMLNormalizesAllowedEmbeds(t *testing.T) {
	testCases := []struct {
		name     string
		src      string
		provider string
		expected string
	}{
		{name: "youtube", src: "https://www.youtube.com/embed/abcDEF123_0", provider: "youtube", expected: "https://www.youtube-nocookie.com/embed/abcDEF123_0"},
		{name: "vimeo", src: "https://player.vimeo.com/video/123456", provider: "vimeo", expected: "https://player.vimeo.com/video/123456"},
		{name: "spotify", src: "https://open.spotify.com/embed/track/4cOdK2wGLETKBW3PvgPWqT", provider: "spotify", expected: "https://open.spotify.com/embed/track/4cOdK2wGLETKBW3PvgPWqT"},
		{name: "soundcloud", src: "https://w.soundcloud.com/player/?url=https%3A%2F%2Fsoundcloud.com%2Fartist%2Ftrack", provider: "soundcloud", expected: "https://w.soundcloud.com/player/?url=https%3A%2F%2Fsoundcloud.com%2Fartist%2Ftrack"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := sanitizeNewsHTML(`<iframe src="` + testCase.src + `" allow="*" onload="bad()"></iframe>`)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, `data-provider="`+testCase.provider+`"`) ||
				!strings.Contains(output, `src="`+testCase.expected+`"`) {
				t.Fatalf("unexpected embed output: %s", output)
			}
			if !strings.Contains(output, `loading="lazy"`) ||
				!strings.Contains(output, `referrerpolicy="strict-origin-when-cross-origin"`) {
				t.Fatalf("missing safe iframe attributes: %s", output)
			}
			if strings.Contains(output, `allow="*"`) || strings.Contains(output, "onload") {
				t.Fatalf("unsafe iframe attributes remained: %s", output)
			}
		})
	}
}

func TestServiceCreateSanitizesBodyBeforePersistence(t *testing.T) {
	repository := &repositoryStub{
		createFn: func(_ context.Context, input CreateNewsInput) (*News, error) {
			if strings.Contains(input.Body, "script") || strings.Contains(input.Body, "onclick") {
				t.Fatalf("unsafe body reached repository: %s", input.Body)
			}
			return &News{
				Id:          1,
				Slug:        input.Slug,
				Category:    input.Category,
				Title:       input.Title,
				Body:        input.Body,
				Author:      input.Author,
				PublishedAt: input.PublishedAt,
			}, nil
		},
	}

	_, err := NewService(repository).Create(context.Background(), CreateNewsRequest{
		Slug:        "safe",
		Category:    "News",
		Title:       "Safe",
		Body:        `<p onclick="bad()">Hello</p><script>bad()</script>`,
		Author:      "Author",
		PublishedAt: "2026-06-13T10:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestServiceCreateRejectsBodyEmptyAfterSanitization(t *testing.T) {
	_, err := NewService(&repositoryStub{}).Create(context.Background(), CreateNewsRequest{
		Slug:        "empty",
		Category:    "News",
		Title:       "Empty",
		Body:        `<script>bad()</script>`,
		Author:      "Author",
		PublishedAt: "2026-06-13T10:00:00Z",
	})
	if !errors.Is(err, ErrInvalidNewsInput) {
		t.Fatalf("expected invalid body error, got %v", err)
	}
}

func TestToNewsResponseSanitizesStoredBody(t *testing.T) {
	response := ToNewsResponse(&News{
		Body: `<p onclick="bad()">Stored</p><script>bad()</script>`,
	})
	if response == nil {
		t.Fatal("expected response")
	}
	if response.Body != "<p>Stored</p>" {
		t.Fatalf("unexpected sanitized response body: %s", response.Body)
	}
}
