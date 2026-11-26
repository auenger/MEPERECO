package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/silieco-ai/silieco/core/internal/service"
)

func TestBuiltinSkillCatalogRead(t *testing.T) {
	h := &Handler{AgentRunService: &service.AgentRunService{}}
	r := chi.NewRouter()
	r.Get("/api/skills/{id}", h.GetSkill)
	request := func(id string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/skills/"+id, nil))
		return w
	}
	catalog := request("builtin:catalog")
	if catalog.Code != http.StatusOK {
		t.Fatalf("catalog status = %d", catalog.Code)
	}
	var meta struct {
		Skills []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(catalog.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.Skills) <= 3 {
		t.Fatalf("catalog only contains %d entries", len(meta.Skills))
	}
	for _, item := range meta.Skills {
		if item.Content != "" {
			t.Fatal("catalog included full skill content")
		}
	}
	optional := request("builtin:silieco-new-feature")
	if optional.Code != http.StatusOK {
		t.Fatalf("optional skill status = %d", optional.Code)
	}
	var body service.AgentSkillData
	if err := json.Unmarshal(optional.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != "builtin:silieco-new-feature" || body.Content == "" {
		t.Fatal("optional skill body missing")
	}
	if unknown := request("builtin:deleted-thermal-skill"); unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown builtin status = %d", unknown.Code)
	}
}
