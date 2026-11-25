package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestGitFileGitLab(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("PRIVATE-TOKEN") != "fixture-token" {
			t.Error("expected authenticated read")
		}
		w.Header().Set("X-Next-Page", "2")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"old_path": "old.go", "new_path": "renamed.go", "renamed_file": true, "diff": "@@ -1 +1 @@\n-old\n+new"},
			{"old_path": "gone.go", "new_path": "gone.go", "deleted_file": true, "too_large": true},
			{"new_path": "new.go", "new_file": true, "diff": strings.Repeat("+", gitFilePatchLimit+1)},
		})
	}))
	defer srv.Close()
	out, e := fetchGitLabFiles(context.Background(), srv.Client(), srv.URL, "fixture-token", 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Files) != 3 || out.NextPage == nil || *out.NextPage != 2 {
		t.Fatalf("unexpected page: count %d", len(out.Files))
	}
	if out.Files[0].Status != "renamed" || out.Files[0].PreviousPath != "old.go" || out.Files[1].Status != "removed" || !out.Files[1].PatchTruncated || out.Files[2].Status != "added" || !out.Files[2].PatchTruncated || len(out.Files[2].Patch) != gitFilePatchLimit {
		t.Fatal("incorrect change normalization")
	}
}

func TestGitFileGitHub(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})))
	revoked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/installations/42/access_tokens":
			var body struct {
				RepositoryIDs []int64           `json:"repository_ids"`
				Permissions   map[string]string `json:"permissions"`
			}
			if e := json.NewDecoder(r.Body).Decode(&body); e != nil || len(body.RepositoryIDs) != 1 || body.RepositoryIDs[0] != 99 || body.Permissions["pull_requests"] != "read" || r.Method != http.MethodPost {
				t.Error("token must be scoped to read-only repository")
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"token":"fixture-token"}`)
		case "/repos/org/repo/pulls/12/files":
			if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer fixture-token" || r.URL.Query().Get("page") != "1" {
				t.Error("incorrect read request")
			}
			w.Header().Set("Link", `<https://example.invalid/page2>; rel="next"`)
			fmt.Fprint(w, `[{"filename":"new.go","previous_filename":"old.go","status":"renamed","additions":3,"deletions":2,"patch":"@@ -1 +1 @@\n-x\n+y"}]`)
		case "/installation/token":
			if r.Method != http.MethodDelete {
				t.Error("expected token revocation")
			}
			revoked = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	previous := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = previous }()
	out, e := fetchGitHubFiles(context.Background(), srv.Client(), 42, "org", "repo", pgtype.Text{String: "99", Valid: true}, 12, 1)
	if e != nil {
		t.Fatal(e)
	}
	if !revoked || out.NextPage == nil || *out.NextPage != 2 || len(out.Files) != 1 || out.Files[0].PreviousPath != "old.go" || out.Files[0].Additions == nil || *out.Files[0].Additions != 3 {
		t.Fatal("incorrect GitHub page")
	}
}
