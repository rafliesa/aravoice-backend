package upload

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadAndServeSupportedMedia(t *testing.T) {
	testCases := []struct {
		name        string
		filename    string
		contentType string
		content     []byte
		kind        string
	}{
		{name: "jpeg", filename: "photo.jpeg", contentType: "image/jpeg", content: append([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}, []byte("JFIF\x00payload")...), kind: "image"},
		{name: "png", filename: "photo.png", contentType: "image/png", content: append([]byte("\x89PNG\r\n\x1a\n"), []byte("payload")...), kind: "image"},
		{name: "webp", filename: "photo.webp", contentType: "image/webp", content: []byte("RIFF\x10\x00\x00\x00WEBPVP8 payload"), kind: "image"},
		{name: "gif", filename: "photo.gif", contentType: "image/gif", content: []byte("GIF89a\x01\x00\x01\x00payload"), kind: "image"},
		{name: "mp3", filename: "voice.mp3", contentType: "audio/mpeg", content: []byte("ID3\x04\x00\x00payload"), kind: "audio"},
		{name: "wav", filename: "voice.wav", contentType: "audio/wav", content: []byte("RIFF\x10\x00\x00\x00WAVEfmt payload"), kind: "audio"},
		{name: "audio ogg", filename: "voice.ogg", contentType: "audio/ogg", content: []byte("OggS\x00\x02payload"), kind: "audio"},
		{name: "m4a", filename: "voice.m4a", contentType: "audio/mp4", content: []byte("\x00\x00\x00\x18ftypM4A payload"), kind: "audio"},
		{name: "mp4", filename: "clip.mp4", contentType: "video/mp4", content: []byte("\x00\x00\x00\x18ftypisompayload"), kind: "video"},
		{name: "webm", filename: "clip.webm", contentType: "video/webm", content: append([]byte{0x1a, 0x45, 0xdf, 0xa3}, []byte("payload")...), kind: "video"},
		{name: "video ogg", filename: "clip.ogv", contentType: "video/ogg", content: []byte("OggS\x00\x02payload"), kind: "video"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			handler, err := NewHandler(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			recorder := uploadRequest(t, mux, testCase.filename, testCase.contentType, testCase.content)
			if recorder.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
			}

			var response Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Kind != testCase.kind || response.Size != int64(len(testCase.content)) {
				t.Fatalf("unexpected response: %#v", response)
			}
			if !strings.HasPrefix(response.URL, "/api/uploads/") {
				t.Fatalf("unexpected URL: %q", response.URL)
			}

			getRecorder := httptest.NewRecorder()
			path := strings.TrimPrefix(response.URL, "/api")
			mux.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, path, nil))
			if getRecorder.Code != http.StatusOK {
				t.Fatalf("expected served status 200, got %d: %s", getRecorder.Code, getRecorder.Body.String())
			}
			if !bytes.Equal(getRecorder.Body.Bytes(), testCase.content) {
				t.Fatal("served content does not match upload")
			}
			if getRecorder.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing nosniff header")
			}
			if getRecorder.Header().Get("Content-Type") != response.MIMEType {
				t.Fatalf(
					"expected served content type %q, got %q",
					response.MIMEType,
					getRecorder.Header().Get("Content-Type"),
				)
			}
		})
	}
}

func TestUploadRejectsInvalidMediaAndCleansTemporaryFiles(t *testing.T) {
	testCases := []struct {
		name        string
		filename    string
		contentType string
		content     []byte
	}{
		{name: "svg", filename: "image.svg", contentType: "image/svg+xml", content: []byte("<svg></svg>")},
		{name: "fake jpeg", filename: "image.jpg", contentType: "image/jpeg", content: []byte("plain text")},
		{name: "mismatched mime", filename: "image.png", contentType: "image/jpeg", content: []byte("\x89PNG\r\n\x1a\npayload")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			directory := t.TempDir()
			handler, err := NewHandler(directory)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			recorder := uploadRequest(t, mux, testCase.filename, testCase.contentType, testCase.content)
			if recorder.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("expected 415, got %d: %s", recorder.Code, recorder.Body.String())
			}
			assertDirectoryEmpty(t, directory)
		})
	}
}

func TestUploadRejectsOversizedImageAndCleansTemporaryFile(t *testing.T) {
	directory := t.TempDir()
	handler, err := NewHandler(directory)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	content := make([]byte, imageLimit+1)
	copy(content, []byte("\x89PNG\r\n\x1a\n"))
	recorder := uploadRequest(t, mux, "large.png", "image/png", content)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", recorder.Code, recorder.Body.String())
	}
	assertDirectoryEmpty(t, directory)
}

func TestUploadRequiresFileFieldAndOnlyAcceptsOneFile(t *testing.T) {
	handler, err := NewHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", "missing")
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing file, got %d", recorder.Code)
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	for _, name := range []string{"first.png", "second.png"} {
		headers := make(textproto.MIMEHeader)
		headers.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
		headers.Set("Content-Type", "image/png")
		part, createErr := writer.CreatePart(headers)
		if createErr != nil {
			t.Fatal(createErr)
		}
		_, _ = part.Write([]byte("\x89PNG\r\n\x1a\npayload"))
	}
	_ = writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for multiple files, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestServeRejectsTraversalAndUnknownFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "secret"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(directory)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/uploads/invalid", nil)
	request.SetPathValue("filename", "../secret")
	recorder := httptest.NewRecorder()
	handler.Serve(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected traversal to return 404, got %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/uploads/invalid", nil)
	request.SetPathValue("filename", strings.Repeat("a", 32)+".png")
	recorder = httptest.NewRecorder()
	handler.Serve(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected unknown file to return 404, got %d", recorder.Code)
	}
}

func TestNewHandlerUsesConfiguredDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "nested", "uploads")
	if _, err := NewHandler(directory); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		t.Fatalf("configured directory was not created: %v", err)
	}
}

func uploadRequest(t *testing.T, handler http.Handler, filename, contentType string, content []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	headers := make(textproto.MIMEHeader)
	headers["Content-Disposition"] = []string{`form-data; name="file"; filename="` + filename + `"`}
	headers["Content-Type"] = []string{contentType}
	part, err := writer.CreatePart(headers)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/uploads", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func assertDirectoryEmpty(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no stored files, found %v", entries)
	}
}
