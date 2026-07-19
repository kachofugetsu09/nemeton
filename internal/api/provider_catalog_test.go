package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kachofugetsu09/nemeton/internal/runner"
	"github.com/kachofugetsu09/nemeton/internal/store"
)

type catalogFixture struct {
	catalog runner.Catalog
}

func (fixture catalogFixture) Catalog(_ context.Context, provider string) (runner.Catalog, error) {
	if provider != fixture.catalog.Provider {
		panic("unexpected Provider " + provider)
	}
	return fixture.catalog, nil
}

func TestProviderCatalogProjectsDiscoveredCodexModels(t *testing.T) {
	discovered := runner.Catalog{Provider: "codex", Version: "codex-test 1", Models: []runner.Model{
		{ID: "gpt-frontier", Label: "GPT Frontier", Description: "Frontier", Default: true,
			OptionValues: []string{"low", "high", "ultra"}, DefaultOption: "low"},
		{ID: "gpt-fast", Label: "GPT Fast", Description: "Fast",
			OptionValues: []string{"low", "medium", "high"}, DefaultOption: "medium"},
	}}
	state := NewRuntimeState(store.ReconcileResult{})
	handler := NewServer(nil, nil, catalogFixture{catalog: discovered}, nil, state, "/data", "/worktrees").Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/providers", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/providers status = %d: %s", response.Code, response.Body.String())
	}
	var actual ProviderCatalog
	if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil {
		t.Fatalf("decode Provider catalog: %v", err)
	}
	expected := ProviderCatalog{Providers: []ProviderDescriptor{
		{ID: "codex", Label: "Codex", Version: "codex-test 1", ModelMode: "select", Option: "reasoning_effort", Models: []ProviderModel{
			{ID: "gpt-frontier", Label: "GPT Frontier", Description: "Frontier", Default: true,
				OptionValues: []string{"low", "high", "ultra"}, DefaultOption: "low"},
			{ID: "gpt-fast", Label: "GPT Fast", Description: "Fast",
				OptionValues: []string{"low", "medium", "high"}, DefaultOption: "medium"},
		}},
		{ID: "opencode", Label: "OpenCode", ModelMode: "editable", Option: "variant", Models: []ProviderModel{
			{ID: "opencode-go/deepseek-v4-pro", Label: "DeepSeek V4 Pro", Default: true,
				OptionValues: []string{"low", "medium", "high"}, DefaultOption: "high"},
		}},
	}}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Provider catalog = %#v, want %#v", actual, expected)
	}
}
