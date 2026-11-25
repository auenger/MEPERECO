package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/silieco-ai/silieco/core/internal/storage"
	db "github.com/silieco-ai/silieco/core/pkg/db/generated"
)

func TestProjectIconCompatibility(t *testing.T) {
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	h := &Handler{Storage: storage.NewLocalStorageFromEnv()}
	for _, icon := range []string{"🌐", "/uploads/users/test/logo.webp"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPatch, "/api/projects/test", nil)
		accepted, ok := h.acceptAvatarURL(w, r, icon, "")
		if !ok || accepted != icon {
			t.Fatalf("icon %q rejected: status=%d", icon, w.Code)
		}
		response := h.projectToResponse(db.Project{Icon: pgtype.Text{String: accepted, Valid: true}})
		if response.Icon == nil || *response.Icon != icon {
			t.Fatalf("icon %q did not round trip", icon)
		}
	}
	if response := h.projectToResponse(db.Project{}); response.Icon != nil {
		t.Fatal("unset project icon must remain null")
	}
}
