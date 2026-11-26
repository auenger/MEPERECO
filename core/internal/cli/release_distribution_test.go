package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
