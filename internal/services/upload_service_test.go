package services

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestValidateExtensionMatchesContentType(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		contentType string
		wantErr     bool
	}{
		{"jpg matches jpeg", "photo.jpg", "image/jpeg", false},
		{"jpeg matches jpeg", "photo.jpeg", "image/jpeg", false},
		{"png matches png", "photo.png", "image/png", false},
		{"webp matches webp", "photo.webp", "image/webp", false},
		{"no extension allowed", "photo", "image/jpeg", false},
		{"jpg mismatched as png", "photo.jpg", "image/png", true},
		{"png mismatched as jpeg", "photo.png", "image/jpeg", true},
		{"txt extension rejected", "photo.txt", "image/jpeg", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateExtensionMatchesContentType(tc.filename, tc.contentType)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateImageDimensions(t *testing.T) {
	largeImg := image.NewRGBA(image.Rect(0, 0, 100, 100))

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, largeImg, nil); err != nil {
		t.Fatalf("failed to encode jpeg: %v", err)
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, largeImg); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	cases := []struct {
		name        string
		data        []byte
		contentType string
		maxW        int
		maxH        int
		wantErr     error
	}{
		{
			name:        "jpeg within limits",
			data:        jpegBuf.Bytes(),
			contentType: "image/jpeg",
			maxW:        200,
			maxH:        200,
			wantErr:     nil,
		},
		{
			name:        "jpeg exceeds width",
			data:        jpegBuf.Bytes(),
			contentType: "image/jpeg",
			maxW:        50,
			maxH:        200,
			wantErr:     ErrImageDimensionsExceeded,
		},
		{
			name:        "jpeg exceeds height",
			data:        jpegBuf.Bytes(),
			contentType: "image/jpeg",
			maxW:        200,
			maxH:        50,
			wantErr:     ErrImageDimensionsExceeded,
		},
		{
			name:        "png within limits",
			data:        pngBuf.Bytes(),
			contentType: "image/png",
			maxW:        200,
			maxH:        200,
			wantErr:     nil,
		},
		{
			name:        "webp skipped for stdlib",
			data:        []byte("RIFF\x00\x00\x00\x00WEBPVP8 "),
			contentType: "image/webp",
			maxW:        10,
			maxH:        10,
			wantErr:     nil,
		},
		{
			name:        "invalid jpeg decode fails",
			data:        []byte("not a jpeg"),
			contentType: "image/jpeg",
			maxW:        200,
			maxH:        200,
			wantErr:     ErrImageDecodeFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateImageDimensions(tc.data, tc.contentType, tc.maxW, tc.maxH)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %v, got nil", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error wrapping %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateUploadFolderAllowed(t *testing.T) {
	allowed := []string{"products", "categories", "flavors"}

	cases := []struct {
		name    string
		folder  string
		wantErr bool
	}{
		{"allowed exact", "products", false},
		{"allowed case insensitive", "Products", false},
		{"disallowed folder", "secret", true},
		{"empty folder allowed", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUploadFolderAllowed(tc.folder, allowed)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeUploadFolder(t *testing.T) {
	cases := []struct {
		name    string
		folder  string
		want    string
		wantErr bool
	}{
		{"simple folder", "products", "products", false},
		{"nested folder", "products/featured", "products/featured", false},
		{"whitespace trimmed", "  products  ", "products", false},
		{"empty folder", "", "", false},
		{"traversal rejected", "../etc", "", true},
		{"dotdot in middle rejected", "products/../etc", "", true},
		{"leading slash cleaned", "/products", "products", false},
		{"invalid chars rejected", "products@2", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeUploadFolder(tc.folder)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
