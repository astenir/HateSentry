package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConsoleRoutesServeIndexAndAssetsWithSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	writeConsoleTestFile(t, distDir, "index.html", "<!doctype html><title>HateSentry console</title>")
	writeConsoleTestFile(t, distDir, "assets/app.js", "console.log('console loaded')")

	engine := gin.New()
	registerConsoleRoutes(engine, gin.Dir(distDir, false))

	redirect := performConsoleRequest(engine, "/console")
	if redirect.Code != http.StatusTemporaryRedirect {
		t.Fatalf("GET /console status = %d, want %d", redirect.Code, http.StatusTemporaryRedirect)
	}
	if location := redirect.Header().Get("Location"); location != "/console/" {
		t.Fatalf("GET /console Location = %q, want /console/", location)
	}

	index := performConsoleRequest(engine, "/console/")
	if index.Code != http.StatusOK {
		t.Fatalf("GET /console/ status = %d, want 200", index.Code)
	}
	if !strings.Contains(index.Body.String(), "HateSentry console") {
		t.Fatalf("GET /console/ body = %q, want console index", index.Body.String())
	}
	assertConsoleSecurityHeaders(t, index)
	if cacheControl := index.Header().Get("Cache-Control"); cacheControl != "no-cache" {
		t.Fatalf("index Cache-Control = %q, want no-cache", cacheControl)
	}

	asset := performConsoleRequest(engine, "/console/assets/app.js")
	if asset.Code != http.StatusOK {
		t.Fatalf("GET console asset status = %d, want 200", asset.Code)
	}
	assertConsoleSecurityHeaders(t, asset)
	if cacheControl := asset.Header().Get("Cache-Control"); cacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control = %q, want immutable cache", cacheControl)
	}
}

func TestConsoleRoutesDoNotListDirectories(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	writeConsoleTestFile(t, distDir, "index.html", "console")
	writeConsoleTestFile(t, distDir, "assets/app.js", "app")

	engine := gin.New()
	registerConsoleRoutes(engine, gin.Dir(distDir, false))

	response := performConsoleRequest(engine, "/console/assets/")
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET asset directory status = %d, want 404", response.Code)
	}
}

func performConsoleRequest(engine http.Handler, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	engine.ServeHTTP(response, request)
	return response
}

func writeConsoleTestFile(t *testing.T, root string, relativePath string, content string) {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create console test directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write console test file: %v", err)
	}
}

func assertConsoleSecurityHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	headers := response.Header()
	if headers.Get("Content-Security-Policy") == "" {
		t.Fatal("Content-Security-Policy header is missing")
	}
	if value := headers.Get("X-Content-Type-Options"); value != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", value)
	}
	if value := headers.Get("X-Frame-Options"); value != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", value)
	}
}
