package vcs

import "testing"

func TestParseCloneURL(t *testing.T) {
	cases := []struct{ raw, key string }{
		{"https://GIT.example.com:443/group/sub/repo.git", "https://git.example.com/group/sub/repo"},
		{"http://git.example.com:8080/scm/group/repo.git", "http://git.example.com:8080/scm/group/repo"},
		{"git@git.example.com:group/sub/repo.git", "ssh://git.example.com/group/sub/repo"},
		{"ssh://git@git.example.com:2222/group/sub/repo", "ssh://git.example.com:2222/group/sub/repo"},
		{"git://git.example.com/group/Repo.git", "git://git.example.com/group/Repo"},
		{"ssh://git@[2001:db8::1]:22/group/repo.git", "ssh://[2001:db8::1]/group/repo"},
	}
	for _, c := range cases {
		loc, e := ParseCloneURL(c.raw)
		if e != nil || loc.Key() != c.key {
			t.Errorf("%s: got %v, %v; want %s", c.raw, loc, e, c.key)
		}
	}
}
func TestMatchInstance(t *testing.T) {
	loc, _ := ParseCloneURL("http://git.example.com:8080/scm/group/sub/repo.git")
	if path, ok := MatchInstance(loc, "http://Git.example.com:8080/scm/"); !ok || path != "group/sub/repo" {
		t.Fatalf("unexpected match: %s %v", path, ok)
	}
	ssh, _ := ParseCloneURL("git@git.example.com:group/sub/repo.git")
	if _, ok := MatchInstance(ssh, "https://git.example.com"); ok {
		t.Fatal("SSH and HTTP API locators must not be inferred")
	}
}
