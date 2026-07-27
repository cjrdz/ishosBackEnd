// upload_service.go — Validates image content type and uploads files to Cloudflare R2 via the S3-compatible API.
package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
)

var (
	ErrInvalidImageType        = errors.New("invalid image type")
	ErrInvalidFolder           = errors.New("invalid upload folder")
	ErrImageDecodeFailed       = errors.New("failed to decode image")
	ErrImageDimensionsExceeded = errors.New("image dimensions exceed maximum allowed size")
	ErrExtensionMismatch       = errors.New("file extension does not match detected image type")
)

var uploadFolderRe = regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]{0,63}$`)
var fileNameSanitizeRe = regexp.MustCompile(`[^a-z0-9._-]`)
var repeatedDashRe = regexp.MustCompile(`-+`)

type UploadService struct {
	client *s3.Client
	bucket string
	cfg    *config.Config
}

func NewUploadService(cfg *config.Config) *UploadService {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.R2Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, ""),
		),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to configure Cloudflare R2 client")
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = &cfg.R2Endpoint
	})

	return &UploadService{
		client: client,
		bucket: cfg.R2BucketName,
		cfg:    cfg,
	}
}

func (s *UploadService) UploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string, maxWidth, maxHeight int, allowedFolders []string) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	contentType := detectImageContentType(data)
	if !isAllowedImageType(contentType) {
		return "", ErrInvalidImageType
	}

	if err := validateExtensionMatchesContentType(header.Filename, contentType); err != nil {
		return "", err
	}

	if err := validateImageDimensions(data, contentType, maxWidth, maxHeight); err != nil {
		return "", err
	}

	folder, err = normalizeUploadFolder(folder)
	if err != nil {
		return "", err
	}

	if err := validateUploadFolderAllowed(folder, allowedFolders); err != nil {
		return "", err
	}

	// Guard against files landing at the bucket root when the folder matches the bucket name.
	if folder == strings.ToLower(strings.TrimSpace(s.bucket)) {
		folder = ""
	}

	ext := extensionFromContentType(contentType)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(header.Filename))
	}
	if ext == "" {
		ext = ".bin"
	}

	base := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	filename := sanitize(base)
	if filename == "" {
		filename = "image"
	}

	objectPath := filename + ext
	if folder != "" {
		objectPath = fmt.Sprintf("%s/%s", folder, objectPath)
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &objectPath,
		Body:        bytes.NewReader(data),
		ContentType: &contentType,
	})
	if err != nil {
		log.Error().Err(err).Str("path", objectPath).Msg("R2 upload failed")
		return "", err
	}

	return objectPath, nil
}

func (s *UploadService) DeleteImage(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &path,
	})
	return err
}

func (s *UploadService) ListImagesPaginated(ctx context.Context, continuationToken string, maxKeys int32) ([]map[string]any, string, error) {
	if maxKeys <= 0 {
		maxKeys = 100
	}
	if maxKeys > 1000 {
		maxKeys = 1000
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  &s.bucket,
		MaxKeys: int32Ptr(maxKeys),
	}
	if continuationToken != "" {
		input.ContinuationToken = &continuationToken
	}

	resp, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, "", err
	}

	images := make([]map[string]any, 0, len(resp.Contents))
	for _, object := range resp.Contents {
		if object.Key == nil || strings.TrimSpace(*object.Key) == "" {
			continue
		}

		createdAt := ""
		if object.LastModified != nil {
			createdAt = object.LastModified.UTC().Format(time.RFC3339)
		}

		mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(*object.Key)))
		publicURL := s.cfg.PublicObjectURL(*object.Key)
		size := int64(0)
		if object.Size != nil {
			size = *object.Size
		}

		images = append(images, map[string]any{
			"name":       *object.Key,
			"size":       size,
			"mime_type":  mimeType,
			"created_at": createdAt,
			"url":        publicURL,
		})
	}

	nextToken := ""
	if resp.IsTruncated != nil && *resp.IsTruncated && resp.NextContinuationToken != nil {
		nextToken = *resp.NextContinuationToken
	}

	return images, nextToken, nil
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

func contentTypeFromExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func validateExtensionMatchesContentType(filename, contentType string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil
	}

	expected := contentTypeFromExtension(ext)
	if expected == "" || expected != contentType {
		return ErrExtensionMismatch
	}
	return nil
}

func validateImageDimensions(data []byte, contentType string, maxWidth, maxHeight int) error {
	// The standard library image package does not include a WebP decoder.
	// For WebP uploads we rely on magic-byte content-type detection and skip
	// dimension validation rather than pulling in a non-stdlib dependency.
	if contentType == "image/webp" {
		return nil
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrImageDecodeFailed, err)
	}

	if cfg.Width > maxWidth || cfg.Height > maxHeight {
		return fmt.Errorf("%w: %dx%d exceeds maximum %dx%d", ErrImageDimensionsExceeded, cfg.Width, cfg.Height, maxWidth, maxHeight)
	}
	return nil
}

func normalizeUploadFolder(folder string) (string, error) {
	folder = strings.TrimSpace(strings.ToLower(folder))
	if folder == "" {
		return "", nil
	}

	// Reject path traversal before cleaning resolves it away.
	if strings.Contains(folder, "..") {
		return "", ErrInvalidFolder
	}

	cleaned := strings.TrimPrefix(path.Clean("/"+folder), "/")
	if cleaned == "." || cleaned == "" {
		return "", ErrInvalidFolder
	}

	if !uploadFolderRe.MatchString(cleaned) {
		return "", ErrInvalidFolder
	}

	return cleaned, nil
}

func validateUploadFolderAllowed(folder string, allowed []string) error {
	if folder == "" {
		return nil
	}
	folder = strings.ToLower(strings.TrimSpace(folder))
	for _, a := range allowed {
		if folder == strings.ToLower(strings.TrimSpace(a)) {
			return nil
		}
	}
	return ErrInvalidFolder
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ToLower(s)
	s = strings.Trim(s, "-._")
	s = fileNameSanitizeRe.ReplaceAllString(s, "-")
	s = repeatedDashRe.ReplaceAllString(s, "-")
	return s
}

func int32Ptr(v int32) *int32 { return &v }
