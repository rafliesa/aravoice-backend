package news

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
)

const maxRequestBodySize = 1 << 20

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /news", h.Create)
	mux.HandleFunc("GET /news", h.GetAll)
	mux.HandleFunc("GET /news/cards", h.GetCards)
	mux.HandleFunc("GET /news/search", h.GetByTitle)
	mux.HandleFunc("GET /news/slug/{slug}", h.GetBySlug)
	mux.HandleFunc("GET /news/{id}", h.GetByID)
	mux.HandleFunc("DELETE /news/{id}", h.Delete)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var request CreateNewsRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	response, err := h.service.Create(r.Context(), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.GetAll(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetCards(w http.ResponseWriter, r *http.Request) {
	page, err := parsePositiveQueryInt(r, "page", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := parsePositiveQueryInt(r, "limit", 6)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := h.service.GetCards(r.Context(), page, limit)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	response, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetByTitle(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.GetByTitle(r.Context(), r.URL.Query().Get("title"))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePositiveQueryInt(r *http.Request, name string, fallback int) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, &InvalidInputError{Message: name + " must be a positive integer"}
	}
	return parsed, nil
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidNewsInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNewsNotFound):
		writeError(w, http.StatusNotFound, ErrNewsNotFound.Error())
	case errors.Is(err, ErrNewsSlugExists):
		writeError(w, http.StatusConflict, ErrNewsSlugExists.Error())
	default:
		log.Printf("news handler: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("encode JSON response: %v", err)
	}
}
