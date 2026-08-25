package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestBrowserHandlerEnforcesSameOriginAndServesUI(t *testing.T) {
	assets := fstest.MapFS{"dist/index.html": {Data: []byte("<html>Nemeton</html>")}}
	apiHandler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	handler, err := BrowserHandler(apiHandler, assets, "127.0.0.1:7373")
	if err != nil {
		t.Fatalf("create Browser handler: %v", err)
	}

	index := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7373/", nil)
	handler.ServeHTTP(index, request)
	data, _ := io.ReadAll(index.Result().Body)
	if index.Code != http.StatusOK || string(data) != "<html>Nemeton</html>" {
		t.Fatalf("index response = %d %q", index.Code, data)
	}
	if index.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("index response lacks Content-Security-Policy")
	}

	for name, mutate := range map[string]func(*http.Request){
		"Host":   func(request *http.Request) { request.Host = "attacker.invalid" },
		"Origin": func(request *http.Request) { request.Header.Set("Origin", "http://attacker.invalid") },
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7373/v1/meetings", nil)
			mutate(request)
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("cross-origin response = %d", response.Code)
			}
		})
	}
}
