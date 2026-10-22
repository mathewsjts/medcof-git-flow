package flow

import (
	"regexp"
	"strings"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
)

var reCherryFrom = regexp.MustCompile(`\(cherry picked from commit ([0-9a-fA-F]{7,40})\)`)

// cherryFrom devolve o SHA de origem registrado por `cherry-pick -x`, ou "".
func cherryFrom(c git.LogCommit) string {
	if mm := reCherryFrom.FindStringSubmatch(c.Body); mm != nil {
		return strings.ToLower(mm[1])
	}
	return ""
}

// matchesSHA compara dois SHAs tolerando abreviação em qualquer um dos lados.
func matchesSHA(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == "" || b == "" {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// containsSHA informa se algum dos commits tem o SHA dado.
func containsSHA(commits []git.LogCommit, sha string) bool {
	for _, c := range commits {
		if matchesSHA(c.SHA, sha) {
			return true
		}
	}
	return false
}
