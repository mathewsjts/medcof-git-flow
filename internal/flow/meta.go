package flow

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mathewsjts/medcof-git-flow/internal/git"
)

var (
	reCherryFrom   = regexp.MustCompile(`\(cherry picked from commit ([0-9a-fA-F]{7,40})\)`)
	reFlowPR       = regexp.MustCompile(`(?mi)^Flow-PR:\s*#?(\d+)\s*$`)
	reFlowDrop     = regexp.MustCompile(`(?mi)^Flow-Drop:\s*#?(\d+)\s*$`)
	reFlowResolves = regexp.MustCompile(`(?mi)^Flow-Resolves:\s*(.+)$`)
	rePRNum        = regexp.MustCompile(`#?(\d+)`)
)

// CommitMeta é a metainformação extraída da mensagem de um commit de ambiente.
type CommitMeta struct {
	SHA          string
	Subject      string
	FlowPR       int    // 0 se ausente
	FlowDrop     int    // 0 se ausente
	FlowResolves []int  // PRs resolvidos
	CherryFrom   string // SHA de origem da linha "(cherry picked from commit X)"
}

// HasTrailer informa se o commit foi aplicado pelo flow (tem Flow-PR ou Flow-Drop).
func (m CommitMeta) HasTrailer() bool { return m.FlowPR != 0 || m.FlowDrop != 0 }

// parseMeta extrai a metainformação de um commit cru.
func parseMeta(c git.LogCommit) CommitMeta {
	m := CommitMeta{SHA: c.SHA, Subject: c.Subject()}
	body := c.Body
	if mm := reCherryFrom.FindStringSubmatch(body); mm != nil {
		m.CherryFrom = strings.ToLower(mm[1])
	}
	if mm := reFlowPR.FindStringSubmatch(body); mm != nil {
		m.FlowPR, _ = strconv.Atoi(mm[1])
	}
	if mm := reFlowDrop.FindStringSubmatch(body); mm != nil {
		m.FlowDrop, _ = strconv.Atoi(mm[1])
	}
	if mm := reFlowResolves.FindStringSubmatch(body); mm != nil {
		for _, g := range rePRNum.FindAllStringSubmatch(mm[1], -1) {
			if n, err := strconv.Atoi(g[1]); err == nil {
				m.FlowResolves = append(m.FlowResolves, n)
			}
		}
	}
	return m
}

// matchesSHA compara dois SHAs tolerando abreviação em qualquer um dos lados.
func matchesSHA(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == "" || b == "" {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
