package config

import (
	"fmt"
	"regexp"
	"strings"
)

// scpLike captura o formato SSH "git@host:owner/repo(.git)".
var scpLike = regexp.MustCompile(`^(?:[^@]+@)?[^:/]+:(.+)$`)

// NormalizeRemote converte uma URL de remote em "owner/repo".
//
// Suporta:
//   - git@github.com:Owner/Repo.git
//   - ssh://git@github.com/Owner/Repo.git
//   - https://github.com/Owner/Repo.git
//   - https://github.com/Owner/Repo
//
// com ou sem ".git" e com barra final.
func NormalizeRemote(remote string) (string, error) {
	s := strings.TrimSpace(remote)
	if s == "" {
		return "", fmt.Errorf("remote vazio")
	}

	var path string
	switch {
	case strings.Contains(s, "://"):
		// URL com esquema (https://, ssh://, git://, http://).
		rest := s[strings.Index(s, "://")+3:]
		// Remove credenciais (user:pass@) e host; fica o path.
		if at := strings.LastIndex(rest, "@"); at != -1 {
			rest = rest[at+1:]
		}
		slash := strings.Index(rest, "/")
		if slash == -1 {
			return "", fmt.Errorf("remote sem path: %q", remote)
		}
		path = rest[slash+1:]
	case scpLike.MatchString(s):
		path = scpLike.FindStringSubmatch(s)[1]
	default:
		return "", fmt.Errorf("formato de remote não reconhecido: %q", remote)
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("remote não contém owner/repo: %q", remote)
	}
	// Usa os dois últimos segmentos (owner/repo), tolerando paths aninhados.
	owner := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", fmt.Errorf("remote não contém owner/repo: %q", remote)
	}
	return owner + "/" + repo, nil
}
