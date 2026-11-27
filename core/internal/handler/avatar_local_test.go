package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/auenger/MEPERECO/core/internal/storage"
	db "github.com/auenger/MEPERECO/core/pkg/db/generated"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestLocalAvatarURLResolution(t *testing.T) {
	for _, base := range []string{"", "https://example.test"} {
		t.Run(base, func(t *testing.T) {
			t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
			t.Setenv("LOCAL_UPLOAD_BASE_URL", base)
			store := storage.NewLocalStorageFromEnv()
			h := &Handler{Storage: store, cfg: Config{PublicURL: "https://example.test"}}
			workspaceKey := "workspaces/" + uuid.NewString() + "/" + uuid.NewString() + ".png"
			raw := store.ObjectURL(workspaceKey)
			resolved := h.resolveAvatarURL(raw)
			if !strings.HasPrefix(resolved, "https://example.test/api/avatars/") || h.normalizeStoredAvatarURL(resolved) != raw {
				t.Fatalf("workspace avatar must use its dedicated route and round-trip to the stored URL")
			}
			userRaw := store.ObjectURL("users/" + uuid.NewString() + "/" + uuid.NewString() + ".png")
			if h.resolveAvatarURL(userRaw) != userRaw {
				t.Fatal("per-user image should retain its existing public URL")
			}
			if h.resolveAvatarURL("emoji:🐱") != "emoji:🐱" {
				t.Fatal("emoji avatar changed")
			}
		})
	}
}

// A disposable PostgreSQL instance is used for the real attachment queries.
// All fixtures are temporary tables and are rolled back at the end.
func TestLocalPublishedAvatarPostgres(t *testing.T) {
	dsn := os.Getenv("MEPERSTUDIO_AVATAR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated avatar test PostgreSQL database required")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"workspace", "agent", "squad", `"user"`} {
		exec("CREATE TEMP TABLE " + table + " (avatar_url text)")
	}
	exec(`CREATE TEMP TABLE attachment (
		id uuid PRIMARY KEY, workspace_id uuid, task_id uuid, comment_id uuid,
		uploader_type text DEFAULT 'member', uploader_id uuid, filename text DEFAULT 'avatar.png',
		url text, content_type text DEFAULT 'image/png', size_bytes bigint, created_at timestamptz DEFAULT now(),
		chat_session_id uuid, chat_message_id uuid, run_id uuid
	)`)
	exec(`CREATE TEMP TABLE attachment_reference (workspace_id uuid, attachment_id uuid)`)

	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	store := storage.NewLocalStorageFromEnv()
	h := &Handler{Storage: store, Queries: db.New(tx), cfg: Config{PublicURL: "https://example.test"}}
	router := chi.NewRouter()
	router.Get("/uploads/*", h.ServeLocalUpload)
	router.Get("/api/avatars/{sig}/*", h.ServeAvatar)
	img, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jWioAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := uuid.NewString()
	makeUpload := func() (id, raw string) {
		t.Helper()
		id = uuid.NewString()
		key := "workspaces/" + workspaceID + "/" + id + ".png"
		raw, err = store.Upload(ctx, key, img, "image/png", "avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO attachment(id,workspace_id,url,size_bytes) VALUES($1,$2,$3,$4)`, id, workspaceID, raw, len(img))
		return
	}
	get := func(target string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		return w
	}
	assertImage := func(target string) {
		t.Helper()
		w := get(target)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), img) {
			t.Fatalf("published avatar did not render: status=%d type=%q", w.Code, w.Header().Get("Content-Type"))
		}
	}
	// Existing saved avatars must render both from fresh API data and from
	// cached raw URLs in installed clients. Re-uploads use fresh UUIDs.
	for _, table := range []string{"workspace", "agent"} {
		_, old := makeUpload()
		exec("INSERT INTO "+table+"(avatar_url) VALUES($1)", old)
		assertImage(old)
		assertImage(h.resolveAvatarURL(old))
		_, next := makeUpload()
		accepted, ok := h.acceptAvatarURL(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/", nil), next, old)
		if !ok || accepted != next {
			t.Fatal("standalone replacement avatar was rejected")
		}
		exec("UPDATE "+table+" SET avatar_url=$1", accepted)
		assertImage(next)
		assertImage(h.resolveAvatarURL(next))
		if got := get(old); got.Code != http.StatusFound {
			t.Fatalf("retired raw URL must use the existing attachment access path, got %d", got.Code)
		}
	}
	// Ordinary and project-referenced images retain the attachment access
	// policy; the avatar repair must not turn them into public resources.
	id, raw := makeUpload()
	if got := get(raw); got.Code != http.StatusFound || got.Header().Get("Location") != attachmentDownloadPath(id) {
		t.Fatal("ordinary upload did not retain its authenticated attachment route")
	}
	exec(`INSERT INTO attachment_reference(workspace_id,attachment_id) VALUES($1,$2)`, workspaceID, id)
	if h.avatarKeyIsPublishable(ctx, store.KeyFromURL(raw)) {
		t.Fatal("project-referenced attachment cannot be used as an avatar")
	}
}
