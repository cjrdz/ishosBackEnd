package openapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateYAML(t *testing.T) {
	yaml, err := GenerateYAML()
	if err != nil {
		t.Fatalf("GenerateYAML failed: %v", err)
	}
	if len(yaml) == 0 {
		t.Fatal("GenerateYAML returned empty output")
	}
	if !strings.Contains(string(yaml), "openapi:") {
		t.Fatal("GenerateYAML output missing openapi version field")
	}
	if !strings.Contains(string(yaml), "paths:") {
		t.Fatal("GenerateYAML output missing paths section")
	}
	if !strings.Contains(string(yaml), "/orders/internal") {
		t.Fatal("GenerateYAML output missing internal order creation endpoint")
	}
}

func TestGenerateJSON(t *testing.T) {
	json, err := GenerateJSON()
	if err != nil {
		t.Fatalf("GenerateJSON failed: %v", err)
	}
	if len(json) == 0 {
		t.Fatal("GenerateJSON returned empty output")
	}
	if !strings.Contains(string(json), `"openapi"`) {
		t.Fatal("GenerateJSON output missing openapi version field")
	}
}

func TestDocsHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/docs", nil)
	rec := httptest.NewRecorder()
	DocsHandler(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected HTML content type, got %q", ct)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "@scalar/api-reference") {
		t.Fatalf("expected CSP allowing the Scalar CDN, got %q", csp)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-url="/openapi.json"`) {
		t.Fatal("docs page missing spec URL reference")
	}
	if !strings.Contains(body, "@scalar/api-reference") {
		t.Fatal("docs page missing Scalar script tag")
	}
}
