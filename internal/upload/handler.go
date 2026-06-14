package upload

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	imageLimit       int64 = 10 << 20
	audioLimit       int64 = 50 << 20
	videoLimit       int64 = 200 << 20
	requestOverhead  int64 = 1 << 20
	signatureMaxSize       = 512
)

var storedFilenamePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpe?g|png|webp|gif|mp3|wav|ogg|oga|m4a|mp4|webm|ogv)$`)

type Handler struct {
	uploadDir string
}

type Response struct {
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	MIMEType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

type mediaType struct {
	kind      string
	mimeType  string
	extension string
	limit     int64
}

func NewHandler(uploadDir string) (*Handler, error) {
	if strings.TrimSpace(uploadDir) == "" {
		return nil, errors.New("upload directory is required")
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}
	return &Handler{uploadDir: uploadDir}, nil
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /uploads", h.Upload)
	mux.HandleFunc("GET /uploads/{filename}", h.Serve)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, videoLimit+requestOverhead)

	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "request must use multipart/form-data")
		return
	}

	part, err := nextFilePart(reader)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer part.Close()

	head := make([]byte, signatureMaxSize)
	headSize, readErr := io.ReadFull(part, head)
	if readErr != nil && readErr != io.ErrUnexpectedEOF {
		if isRequestTooLarge(readErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the upload limit")
			return
		}
		writeError(w, http.StatusBadRequest, "could not read uploaded file")
		return
	}
	head = head[:headSize]
	if len(head) == 0 {
		writeError(w, http.StatusBadRequest, "uploaded file is empty")
		return
	}

	media, err := detectMedia(part.FileName(), part.Header.Get("Content-Type"), head)
	if err != nil {
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	temp, err := os.CreateTemp(h.uploadDir, ".upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store uploaded file")
		return
	}
	tempName := temp.Name()
	keepTemp := false
	defer func() {
		temp.Close()
		if !keepTemp {
			os.Remove(tempName)
		}
	}()

	size, copyErr := io.Copy(temp, io.LimitReader(io.MultiReader(bytes.NewReader(head), part), media.limit+1))
	if copyErr != nil {
		if isRequestTooLarge(copyErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the upload limit")
			return
		}
		writeError(w, http.StatusBadRequest, "could not read uploaded file")
		return
	}
	if size > media.limit {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("%s file exceeds the %d MB limit", media.kind, media.limit>>20))
		return
	}
	if err := ensureNoAdditionalFiles(reader); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := temp.Sync(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not store uploaded file")
		return
	}
	if err := temp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not store uploaded file")
		return
	}

	filename, err := randomFilename(media.extension)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not store uploaded file")
		return
	}
	destination := filepath.Join(h.uploadDir, filename)
	if err := os.Rename(tempName, destination); err != nil {
		writeError(w, http.StatusInternalServerError, "could not store uploaded file")
		return
	}
	if err := os.Chmod(destination, 0o644); err != nil {
		os.Remove(destination)
		writeError(w, http.StatusInternalServerError, "could not store uploaded file")
		return
	}
	keepTemp = true

	writeJSON(w, http.StatusCreated, Response{
		URL:      "/api/uploads/" + filename,
		Kind:     media.kind,
		MIMEType: media.mimeType,
		Size:     size,
	})
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if !storedFilenamePattern.MatchString(filename) || filepath.Base(filename) != filename {
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(filepath.Join(h.uploadDir, filename))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, "could not read uploaded file")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	contentType := storedContentType(filename)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func storedContentType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogv":
		return "video/ogg"
	default:
		return ""
	}
}

func nextFilePart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, errors.New(`multipart field "file" is required`)
		}
		if err != nil {
			return nil, errors.New("invalid multipart body")
		}
		if part.FormName() == "file" && part.FileName() != "" {
			return part, nil
		}
		part.Close()
	}
}

func ensureNoAdditionalFiles(reader *multipart.Reader) error {
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if isRequestTooLarge(err) {
				return errors.New("file exceeds the upload limit")
			}
			return errors.New("invalid multipart body")
		}
		isFile := part.FileName() != ""
		part.Close()
		if isFile {
			return errors.New("only one file may be uploaded at a time")
		}
	}
}

func detectMedia(filename string, declaredMIME string, head []byte) (mediaType, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	declaredMIME = normalizeMIME(declaredMIME)
	detectedMIME := normalizeMIME(http.DetectContentType(head))

	candidate, ok := mediaByExtension(extension, declaredMIME)
	if !ok {
		return mediaType{}, errors.New("unsupported file extension or media type")
	}
	if !signatureMatches(candidate, detectedMIME, head) {
		return mediaType{}, errors.New("file signature does not match its media type")
	}
	return candidate, nil
}

func mediaByExtension(extension string, declaredMIME string) (mediaType, bool) {
	switch extension {
	case ".jpg", ".jpeg":
		return exactMedia("image", "image/jpeg", ".jpg", imageLimit, declaredMIME, "image/jpeg")
	case ".png":
		return exactMedia("image", "image/png", ".png", imageLimit, declaredMIME, "image/png")
	case ".webp":
		return exactMedia("image", "image/webp", ".webp", imageLimit, declaredMIME, "image/webp")
	case ".gif":
		return exactMedia("image", "image/gif", ".gif", imageLimit, declaredMIME, "image/gif")
	case ".mp3":
		return exactMedia("audio", "audio/mpeg", ".mp3", audioLimit, declaredMIME, "audio/mpeg", "audio/mp3")
	case ".wav":
		return exactMedia("audio", "audio/wav", ".wav", audioLimit, declaredMIME, "audio/wav", "audio/wave", "audio/x-wav")
	case ".m4a":
		return exactMedia("audio", "audio/mp4", ".m4a", audioLimit, declaredMIME, "audio/mp4", "audio/x-m4a")
	case ".mp4":
		return exactMedia("video", "video/mp4", ".mp4", videoLimit, declaredMIME, "video/mp4")
	case ".webm":
		return exactMedia("video", "video/webm", ".webm", videoLimit, declaredMIME, "video/webm")
	case ".oga":
		return exactMedia("audio", "audio/ogg", ".oga", audioLimit, declaredMIME, "audio/ogg", "application/ogg")
	case ".ogv":
		return exactMedia("video", "video/ogg", ".ogv", videoLimit, declaredMIME, "video/ogg", "application/ogg")
	case ".ogg":
		if declaredMIME == "video/ogg" {
			return mediaType{kind: "video", mimeType: "video/ogg", extension: ".ogv", limit: videoLimit}, true
		}
		return exactMedia("audio", "audio/ogg", ".ogg", audioLimit, declaredMIME, "audio/ogg", "application/ogg")
	default:
		return mediaType{}, false
	}
}

func exactMedia(kind, canonicalMIME, extension string, limit int64, declared string, accepted ...string) (mediaType, bool) {
	for _, value := range accepted {
		if declared == value {
			return mediaType{kind: kind, mimeType: canonicalMIME, extension: extension, limit: limit}, true
		}
	}
	return mediaType{}, false
}

func signatureMatches(media mediaType, detectedMIME string, head []byte) bool {
	switch media.mimeType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return detectedMIME == media.mimeType
	case "audio/mpeg":
		return detectedMIME == "audio/mpeg" ||
			bytes.HasPrefix(head, []byte("ID3")) ||
			(len(head) >= 2 && head[0] == 0xff && head[1]&0xe0 == 0xe0)
	case "audio/wav":
		return len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE"
	case "audio/ogg", "video/ogg":
		return bytes.HasPrefix(head, []byte("OggS"))
	case "audio/mp4", "video/mp4":
		return len(head) >= 12 && string(head[4:8]) == "ftyp"
	case "video/webm":
		return len(head) >= 4 && bytes.Equal(head[:4], []byte{0x1a, 0x45, 0xdf, 0xa3})
	default:
		return false
	}
}

func normalizeMIME(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(mediaType)
}

func randomFilename(extension string) (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value) + extension, nil
}

func isRequestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError) || strings.Contains(strings.ToLower(err.Error()), "request body too large")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
