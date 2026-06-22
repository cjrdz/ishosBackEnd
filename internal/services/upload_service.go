// upload_service.go — Validates image content type and uploads files to Cloudflare R2 via the S3-compatible API.
package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

var ErrInvalidImageType = errors.New("invalid image type")
var ErrInvalidFolder = errors.New("invalid upload folder")

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

func (s *UploadService) UploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
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

func (s *UploadService) ListImages(ctx context.Context) ([]map[string]any, error) {
	resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &s.bucket,
		MaxKeys: int32Ptr(1000),
	})
	if err != nil {
		return nil, err
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
		return "", nil
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
	s = strings.Trim(s, "-._")
	s = fileNameSanitizeRe.ReplaceAllString(s, "-")
	s = repeatedDashRe.ReplaceAllString(s, "-")
	return s
}

func int32Ptr(v int32) *int32 { return &v }
