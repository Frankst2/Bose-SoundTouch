package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootEndpoint(t *testing.T) {
	r, _ := setupRouter("http://localhost:8001", nil)

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Accept", "text/html")

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", res.Status)
	}

	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected text/html content type, got %s", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "AfterTouch") {
		t.Errorf("Expected body to contain 'AfterTouch', got %s", string(body))
	}
}

func TestRootEndpointJSON(t *testing.T) {
	r, _ := setupRouter("http://localhost:8001", nil)

	ts := httptest.NewServer(r)
	defer ts.Close()

	client := &http.Client{}
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", res.Status)
	}

	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected application/json content type, got %s", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	expected := `{"Bose": "AfterTouch", "service": "Go/Chi", "docs": "https://gesellix.github.io/Bose-SoundTouch/"}`
	if strings.TrimSpace(string(body)) != expected {
		t.Errorf("Expected body %s, got %s", expected, string(body))
	}
}

func TestStaticMedia(t *testing.T) {
	r, _ := setupRouter("http://localhost:8001", nil)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Use a known file from static/media
	res, err := http.Get(ts.URL + "/media/SiriusXM_Logo_Color.svg")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", res.Status)
	}

	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "image/svg+xml") {
		t.Errorf("Expected image/svg+xml content type, got %s", contentType)
	}
}

func TestStaticWeb(t *testing.T) {
	r, _ := setupRouter("http://localhost:8001", nil)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// 1. Test Migration UI CSS
	res, err := http.Get(ts.URL + "/web/migration/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Migration CSS: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/css") {
		t.Errorf("Migration CSS: Expected text/css content type, got %s", res.Header.Get("Content-Type"))
	}

	// 2. Test Migration UI JS
	res, err = http.Get(ts.URL + "/web/migration/script.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Migration JS: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "application/javascript") &&
		!strings.Contains(res.Header.Get("Content-Type"), "text/javascript") {
		t.Errorf("Migration JS: Expected javascript content type, got %s", res.Header.Get("Content-Type"))
	}

	// 3. Test Migration UI Index
	res, err = http.Get(ts.URL + "/web/migration/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Migration Index: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("Migration Index: Expected text/html content type, got %s", res.Header.Get("Content-Type"))
	}

	// 4. Test Stockholm Mini
	res, err = http.Get(ts.URL + "/web/stockholm-mini/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Stockholm Mini: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("Stockholm Mini: Expected text/html content type, got %s", res.Header.Get("Content-Type"))
	}

	// 5. Test Stockholm Mini CSS
	res, err = http.Get(ts.URL + "/web/stockholm-mini/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Stockholm Mini CSS: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/css") {
		t.Errorf("Stockholm Mini CSS: Expected text/css content type, got %s", res.Header.Get("Content-Type"))
	}

	// 6. Test Shared CSS
	res, err = http.Get(ts.URL + "/web/shared/common.css")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Shared CSS: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/css") {
		t.Errorf("Shared CSS: Expected text/css content type, got %s", res.Header.Get("Content-Type"))
	}

	// 7. Test Shared JS
	res, err = http.Get(ts.URL + "/web/shared/common.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Shared JS: Expected status OK, got %v", res.Status)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "application/javascript") &&
		!strings.Contains(res.Header.Get("Content-Type"), "text/javascript") {
		t.Errorf("Shared JS: Expected javascript content type, got %s", res.Header.Get("Content-Type"))
	}
}
