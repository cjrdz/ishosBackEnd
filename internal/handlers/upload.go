// upload.go — Image upload/delete/list handlers backed by Cloudflare R2.
// UploadImage enforces a 6 MiB hard limit at the HTTP layer and a 5 MiB multipart limit;
// the 1 MiB gap ensures oversized requests are rejected before multipart parsing begins.
package handlers

import (
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type UploadHandler struct {
	uploadSvc *services.UploadService
}

func NewUploadHandler(cfg *config.Config) *UploadHandler {
	return &UploadHandler{
		uploadSvc: services.NewUploadService(cfg),
	}
}

func (h *UploadHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)

	// #nosec G120 -- request body is hard-capped via MaxBytesReader above.
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		response.Error(w, http.StatusBadRequest, "File too large (max 5MB)")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Missing 'image' form field")
		return
	}
	defer func() { _ = file.Close() }()

	path, err := h.uploadSvc.UploadImage(r.Context(), file, header, r.FormValue("folder"))
	if err != nil {
		if errors.Is(err, services.ErrInvalidImageType) {
			response.Error(w, http.StatusBadRequest,
				"Only JPEG, PNG and WebP images are allowed")
			return
		}
		if errors.Is(err, services.ErrInvalidFolder) {
			response.Error(w, http.StatusBadRequest,
				"Invalid folder value")
			return
		}
		log.Error().Err(err).Msg("Failed to upload image")
		response.Error(w, http.StatusInternalServerError, "Failed to upload image")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"path":    path,
		"message": "Image uploaded successfully",
	})
}

func (h *UploadHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Path == "" {
		response.Error(w, http.StatusBadRequest, "path is required")
		return
	}

	if err := h.uploadSvc.DeleteImage(r.Context(), body.Path); err != nil {
		log.Error().Err(err).Str("path", body.Path).Msg("R2 delete failed")
		response.Error(w, http.StatusInternalServerError, "Failed to delete image")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Image deleted successfully",
	})
}

func (h *UploadHandler) ListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.uploadSvc.ListImages(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list images")
		response.Error(w, http.StatusInternalServerError, "Failed to list images")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"images": images,
	})
}
