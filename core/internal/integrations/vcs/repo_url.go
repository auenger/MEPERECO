package vcs

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// CloneLocator is a transport locator, not an API endpoint or provider ID.
type CloneLocator struct {
	Scheme string
	Host   string
	Path   string
}

func (l CloneLocator) Key() string { return l.Scheme + "://" + l.Host + "/" + l.Path }

func NormalizeRepositoryPath(raw string) (string, error) {
	p := strings.Trim(strings.TrimSpace(raw), "/")
	p = strings.TrimSuffix(p, ".git")
	if p == "" || strings.ContainsAny(p, "\\?#\x00\r\n\t") {
		return "", errors.New("invalid repository path")
	}
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return "", errors.New("repository namespace and name are required")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part {
			return "", errors.New("invalid repository path segment")
		}
	}
	return p, nil
}

func ParseCloneURL(raw string) (CloneLocator, error) {
	value := strings.TrimSpace(raw)
	var loc CloneLocator
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" {
			return loc, errors.New("invalid clone URL")
		}
		loc.Scheme = strings.ToLower(u.Scheme)
		switch loc.Scheme {
		case "http", "https", "ssh", "git":
		default:
			return loc, errors.New("unsupported clone transport")
		}
		loc.Host = normalizedHost(u)
		loc.Path = u.Path
	} else {
		// scp form: [user@]hostname:path. Bracketed IPv6 is accepted.
		hostPath := value
		if index := strings.Index(hostPath, "@"); index >= 0 {
			hostPath = hostPath[index+1:]
		}
		sep := strings.Index(hostPath, ":")
		if strings.HasPrefix(hostPath, "[") {
			end := strings.Index(hostPath, "]")
			if end < 0 || end+1 >= len(hostPath) || hostPath[end+1] != ':' {
				return loc, errors.New("invalid SSH host")
			}
			sep = end + 1
		}
		if sep <= 0 || sep == len(hostPath)-1 || strings.ContainsAny(hostPath[:sep], " /@") {
			return loc, errors.New("invalid scp clone URL")
		}
		loc = CloneLocator{Scheme: "ssh", Host: strings.ToLower(hostPath[:sep]), Path: hostPath[sep+1:]}
	}
	p, err := NormalizeRepositoryPath(loc.Path)
	if err != nil {
		return CloneLocator{}, err
	}
	loc.Path = p
	return loc, nil
}

func normalizedHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") || (u.Scheme == "ssh" && port == "22") || (u.Scheme == "git" && port == "9418") {
		port = ""
	}
	if port != "" {
		return net.JoinHostPort(host, port)
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// MatchInstance only infers transport-compatible HTTP(S) mappings. SSH/API
// aliases and ports must be mapped explicitly by the project administrator.
func MatchInstance(loc CloneLocator, raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (loc.Scheme != "http" && loc.Scheme != "https") || loc.Scheme != u.Scheme || loc.Host != normalizedHost(u) {
		return "", false
	}
	prefix := strings.Trim(u.Path, "/")
	p := loc.Path
	if prefix != "" {
		if !strings.HasPrefix(p, prefix+"/") {
			return "", false
		}
		p = strings.TrimPrefix(p, prefix+"/")
	}
	p, err = NormalizeRepositoryPath(p)
	return p, err == nil
}
