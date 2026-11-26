package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublishedCLIDistribution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cli/version.txt" {
			fmt.Fprintln(w, "1.2.3")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv("MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL", srv.URL+"/cli/")
	release, err := FetchLatestRelease()
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v1.2.3" || len(release.Assets) != 7 {
		t.Fatalf("bad release metadata: %+v", release)
	}
	for _, osName := range []string{"darwin", "linux", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			asset, err := findReleaseAsset(release.Assets, release.TagName, osName, arch)
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("meperstudio-%s-%s.%s", osName, arch, releaseArchiveExtension(osName))
			if asset.Name != want || asset.BrowserDownloadURL != srv.URL+"/cli/"+want {
				t.Fatalf("bad asset: %+v", asset)
			}
		}
	}
	if _, err := fetchReleaseByTag("v1.2.2"); err == nil {
		t.Fatal("expected mismatched version rejection")
	}
}

func TestVersionedCLIManifest(t *testing.T) {
	digest := strings.Repeat("a", 64)
	artifact := CLIReleaseArtifact{OS: "darwin", Arch: "arm64", Path: "releases/1.2.3/" + digest + "/bundle.tar.gz", ArchiveName: "bundle.tar.gz", Binary: "meperstudio", SHA256: digest, SizeBytes: 42}
	m := CLIReleaseManifest{SchemaVersion: CLIManifestVersion, Version: "1.2.3", Artifacts: []CLIReleaseArtifact{artifact}}
	raw, _ := json.Marshal(m)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cli/manifest.json" {
			_, _ = w.Write(raw)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv("MEPERSTUDIO_CLI_DOWNLOAD_BASE_URL", srv.URL+"/cli")
	release, err := FetchLatestRelease()
	if err != nil {
		t.Fatal(err)
	}
	asset, err := findReleaseAsset(release.Assets, release.TagName, "darwin", "arm64")
	if err != nil || asset.SHA256 != digest || asset.BrowserDownloadURL != srv.URL+"/cli/"+artifact.Path {
		t.Fatal(asset, err)
	}
	for _, bad := range []CLIReleaseArtifact{{OS: "darwin", Arch: "arm64", Path: "../bundle", ArchiveName: artifact.ArchiveName, Binary: artifact.Binary, SHA256: digest, SizeBytes: 42}, {OS: "darwin", Arch: "arm64", Path: artifact.Path, ArchiveName: artifact.ArchiveName, Binary: "../bin", SHA256: digest, SizeBytes: 42}} {
		m.Artifacts = []CLIReleaseArtifact{bad}
		encoded, _ := json.Marshal(m)
		if _, err := ParseCLIReleaseManifest(encoded); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}
