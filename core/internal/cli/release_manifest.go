package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const CLIManifestVersion = "meperstudio.cli/v1"

type CLIReleaseManifest struct {
	SchemaVersion string               `json:"schema_version"`
	Version       string               `json:"version"`
	Artifacts     []CLIReleaseArtifact `json:"artifacts"`
}
type CLIReleaseArtifact struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Path        string `json:"path"`
	ArchiveName string `json:"archive_name"`
	Binary      string `json:"binary"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
}

var safeArchiveName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func ParseCLIReleaseManifest(raw []byte) (CLIReleaseManifest, error) {
	var m CLIReleaseManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, err
	}
	if m.SchemaVersion != CLIManifestVersion || !IsReleaseVersion(m.Version) || strings.HasPrefix(m.Version, "v") || strings.TrimSpace(m.Version) != m.Version || len(m.Artifacts) == 0 {
		return m, fmt.Errorf("invalid CLI release manifest")
	}
	seen := map[string]bool{}
	for _, a := range m.Artifacts {
		k := a.OS + "/" + a.Arch
		if seen[k] || a.OS == "" || a.Arch == "" {
			return m, fmt.Errorf("duplicate or missing CLI platform %q", k)
		}
		seen[k] = true
		digest, err := hex.DecodeString(a.SHA256)
		if err != nil || len(digest) != 32 || a.SHA256 != strings.ToLower(a.SHA256) || a.SizeBytes <= 0 || !safeArchiveName.MatchString(a.ArchiveName) || a.ArchiveName == "." || a.ArchiveName == ".." {
			return m, fmt.Errorf("invalid CLI artifact for %s", k)
		}
		binary := "meperstudio"
		if a.OS == "windows" {
			binary += ".exe"
		}
		if a.Binary != binary || a.Path != "releases/"+m.Version+"/"+a.SHA256+"/"+a.ArchiveName {
			return m, fmt.Errorf("invalid CLI artifact path or executable for %s", k)
		}
	}
	return m, nil
}
func fetchCLIReleaseManifest(base string) (*CLIReleaseManifest, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(base + "/manifest.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CLI manifest HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	m, err := ParseCLIReleaseManifest(raw)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
