package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	storage "github.com/supabase-community/storage-go"

	"github.com/cjrdz/ishosBackEnd/internal/config"
)

var ErrInvalidImageType = errors.New("invalid image type")
var ErrInvalidFolder = errors.New("invalid upload folder")

var uploadFolderRe = regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]{0,63}$`)

type UploadService struct {
	client      *storage.Client
	bucket      string
	supabaseURL string
	apiKey      string
}

func NewUploadService(cfg *config.Config) *UploadService {
	client := storage.NewClient(cfg.SupabaseURL+"/storage/v1", cfg.SupabaseKey, nil)
	return &UploadService{
		client:      client,
		bucket:      cfg.SupabaseBucket,
		supabaseURL: cfg.SupabaseURL,
		apiKey:      cfg.SupabaseKey,
	}
}

func (s *UploadService) UploadImage(_ context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	contentType := detectImageContentType(data)
	if !isAllowedImageType(contentType) {
		return "", ErrInvalidImageType
	}

	folder, err = normalizeUploadFolder(folder)
	if err != nil {
		return "", err
	}

	ext := extensionFromContentType(contentType)
	if ext == "" {
		ext = filepath.Ext(header.Filename)
	}
	if ext == "" {
		ext = ".bin"
	}

	base := strings.TrimSuffix(header.Filename, ext)
	ts := time.Now().Format("20060102T150405")
	objectPath := fmt.Sprintf("%s/%s_%s%s", folder, sanitize(base), ts, ext)

	_, err = s.client.UploadFile(s.bucket, objectPath, bytes.NewReader(data), storage.FileOptions{
		ContentType: &contentType,
		Upsert:      boolPtr(true),
	})
	if err != nil {
		log.Error().Err(err).Str("path", objectPath).Msg("Supabase upload failed")
		return "", err
	}

	return objectPath, nil
}

func (s *UploadService) DeleteImage(_ context.Context, path string) error {
	_, err := s.client.RemoveFile(s.bucket, []string{path})
	return err
}

func (s *UploadService) ListImages(_ context.Context) ([]map[string]interface{}, error) {
	listURL := fmt.Sprintf("%s/storage/v1/object/list/%s", s.supabaseURL, s.bucket)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"prefix": "",
		"limit":  100,
		"offset": 0,
		"sortBy": map[string]string{
			"column": "created_at",
			"order":  "desc",
		},
	})

	req, err := http.NewRequest("POST", listURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supabase list failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var files []struct {
		Name      string `json:"name"`
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
		Metadata  struct {
			Size     int    `json:"size"`
			MimeType string `json:"mimetype"`
		} `json:"metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, err
	}

	images := make([]map[string]interface{}, 0, len(files))
	for _, f := range files {
		if f.ID == "" {
			continue
		}
		publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.supabaseURL, s.bucket, f.Name)
		images = append(images, map[string]interface{}{
			"name":       f.Name,
			"size":       f.Metadata.Size,
			"mime_type":  f.Metadata.MimeType,
			"created_at": f.CreatedAt,
			"url":        publicURL,
		})
	}

	return images, nil
}

func isAllowedImageType(ct string) bool {
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	return allowed[ct]
}

func detectImageContentType(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	limit := 512
	if len(data) < limit {
		limit = len(data)
	}
	return http.DetectContentType(data[:limit])
}

func extensionFromContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

func normalizeUploadFolder(folder string) (string, error) {
	folder = strings.TrimSpace(strings.ToLower(folder))
	if folder == "" {
		return "products", nil
	}

	if !uploadFolderRe.MatchString(folder) {
		return "", ErrInvalidFolder
	}

	cleaned := strings.TrimPrefix(path.Clean("/"+folder), "/")
	if cleaned == "." || cleaned == "" || strings.Contains(cleaned, "..") {
		return "", ErrInvalidFolder
	}

	return cleaned, nil
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ToLower(s)
	return s
}

func boolPtr(b bool) *bool { return &b }
