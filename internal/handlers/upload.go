// upload.go — Image upload/delete/list handlers backed by Cloudflare R2.
package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/audit"
	"github.com/cjrdz/ishosBackEnd/internal/auth"
	"github.com/cjrdz/ishosBackEnd/internal/config"
	"github.com/cjrdz/ishosBackEnd/internal/services"
	"github.com/cjrdz/ishosBackEnd/pkg/response"
)

type UploadHandler struct {
	uploadSvc          *services.UploadService
	maxUploadMB        int
	maxMultipartBytes  int64
	maxRequestBodySize int64
	maxImageWidth      int
	maxImageHeight     int
	allowedFolders     []string
	auditSvc           audit.Service
}

func NewUploadHandler(cfg *config.Config, auditSvc audit.Service) *UploadHandler {
	maxUploadMB := cfg.AdminImageUploadMaxMB
	if maxUploadMB <= 0 {
		maxUploadMB = 15
	}

	maxMultipartBytes := int64(maxUploadMB) << 20
	maxRequestBodySize := maxMultipartBytes + (1 << 20)

	return &UploadHandler{
		uploadSvc:          services.NewUploadService(cfg),
		maxUploadMB:        maxUploadMB,
		maxMultipartBytes:  maxMultipartBytes,
		maxRequestBodySize: maxRequestBodySize,
		maxImageWidth:      cfg.AdminImageUploadMaxWidth,
		maxImageHeight:     cfg.AdminImageUploadMaxHeight,
		allowedFolders:     parseAllowedFolders(cfg.AllowedUploadFolders),
		auditSvc:           auditSvc,
	}
}

func (h *UploadHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxRequestBodySize)

	// #nosec G120 -- request body is hard-capped via MaxBytesReader above.
	if err := r.ParseMultipartForm(h.maxMultipartBytes); err != nil {
		response.Error(w, http.StatusBadRequest,
			fmt.Sprintf("File too large (max %dMB)", h.maxUploadMB))
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Missing 'image' form field")
		return
	}
	defer func() { _ = file.Close() }()

	path, err := h.uploadSvc.UploadImage(r.Context(), file, header, r.FormValue("folder"), h.maxImageWidth, h.maxImageHeight, h.allowedFolders)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidImageType):
			response.Error(w, http.StatusBadRequest,
				"Only JPEG, PNG and WebP images are allowed")
		case errors.Is(err, services.ErrExtensionMismatch):
			response.Error(w, http.StatusBadRequest,
				"File extension does not match detected image type")
		case errors.Is(err, services.ErrImageDecodeFailed):
			response.Error(w, http.StatusBadRequest,
				"Uploaded file could not be decoded as a valid image")
		case errors.Is(err, services.ErrImageDimensionsExceeded):
			response.Error(w, http.StatusBadRequest,
				fmt.Sprintf("Image dimensions exceed the maximum allowed size of %dx%d", h.maxImageWidth, h.maxImageHeight))
		case errors.Is(err, services.ErrInvalidFolder):
			response.Error(w, http.StatusBadRequest,
				"Invalid folder value")
		default:
			log.Error().Err(err).Msg("Failed to upload image")
			response.Error(w, http.StatusInternalServerError, "Failed to upload image")
		}
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

	actorID := getAdminID(r)
	ctx := audit.ContextFromRequest(r, actorID)
	ip := auth.ClientIP(r)
	_ = h.auditSvc.Log(ctx, audit.ActionImageDeleted, actorID, fmt.Sprintf("image:%s", body.Path), map[string]any{
		"path": body.Path,
	}, ip)

	response.JSON(w, http.StatusOK, map[string]string{
		"message": "Image deleted successfully",
	})
}

func (h *UploadHandler) ListImages(w http.ResponseWriter, r *http.Request) {
	continuationToken := r.URL.Query().Get("continuation_token")
	maxKeys := int32(100)

	if mk := r.URL.Query().Get("max_keys"); mk != "" {
		if parsed, err := strconv.ParseInt(mk, 10, 32); err == nil {
			maxKeys = int32(parsed)
		}
	}
	if maxKeys <= 0 {
		maxKeys = 100
	}
	if maxKeys > 1000 {
		maxKeys = 1000
	}

	images, nextToken, err := h.uploadSvc.ListImagesPaginated(r.Context(), continuationToken, maxKeys)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list images")
		response.Error(w, http.StatusInternalServerError, "Failed to list images")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"images":                  images,
		"truncated":               nextToken != "",
		"next_continuation_token": nextToken,
	})
}

func parseAllowedFolders(raw string) []string {
	parts := strings.Split(raw, ",")
	folders := make([]string, 0, len(parts))
	for _, p := range parts {
		if folder := strings.TrimSpace(p); folder != "" {
			folders = append(folders, folder)
		}
	}
	return folders
}
