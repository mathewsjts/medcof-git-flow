package git

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Fetch atualiza o remote (por padrão origin) sem tocar no working tree.
func (g *Git) Fetch(refs ...string) error {
	args := append([]string{"fetch", "--prune", "origin"}, refs...)
	return g.RunOK(args...)
}

// RevParse resolve uma revisão para o SHA completo.
func (g *Git) RevParse(rev string) (string, error) {
	return g.Run("rev-parse", "--verify", rev)
}

// BranchExists informa se existe a branch local.
func (g *Git) BranchExists(branch string) bool {
	return g.RunOK("rev-parse", "--verify", "-q", "refs/heads/"+branch) == nil
}

// MergeBase devolve o merge-base entre duas revisões.
func (g *Git) MergeBase(a, b string) (string, error) {
	return g.Run("merge-base", a, b)
}

// IsAncestor informa se ancestor é ancestral de (ou igual a) rev.
func (g *Git) IsAncestor(ancestor, rev string) bool {
	return g.RunOK("merge-base", "--is-ancestor", ancestor, rev) == nil
}

// RevListRange devolve os SHAs do intervalo (ex.: "base..head"), do mais antigo
// ao mais novo quando reverse=true. Exclui merges se noMerges=true.
func (g *Git) RevListRange(rng string, reverse, noMerges bool) ([]string, error) {
	args := []string{"rev-list"}
	if reverse {
		args = append(args, "--reverse")
	}
	if noMerges {
		args = append(args, "--no-merges")
	}
	args = append(args, rng)
	out, err := g.Run(args...)
	if err != nil {
		return nil, err
	}
	return Lines(out), nil
}

// Count devolve quantos commits as revisões selecionam (ex.: "a", "^b").
func (g *Git) Count(revs ...string) (int, error) {
	out, err := g.Run(append([]string{"rev-list", "--count"}, revs...)...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// MergeHasOwnChanges informa se o merge commit muda algo além do merge
// automático dos parents (resolução de conflito ou alteração manual).
func (g *Git) MergeHasOwnChanges(sha string) (bool, error) {
	out, err := g.Run("show", "--remerge-diff", "--format=", "--name-only", sha)
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// CherryEquivalent devolve o conjunto de SHAs de head (limitado por limit) cujo
// patch-id já existe em upstream (linhas "-" de `git cherry`).
func (g *Git) CherryEquivalent(upstream, head, limit string) (map[string]bool, error) {
	out, err := g.Run("cherry", upstream, head, limit)
	if err != nil {
		return nil, err
	}
	applied := map[string]bool{}
	for _, ln := range Lines(out) {
		f := strings.Fields(ln)
		if len(f) == 2 && f[0] == "-" {
			applied[f[1]] = true
		}
	}
	return applied, nil
}

// LogCommit é um commit cru lido do log (mensagem completa inclusa).
type LogCommit struct {
	SHA        string
	Parents    []string
	AuthorDate time.Time // preservada por cherry-pick e rebase
	Body       string    // mensagem completa (subject + corpo + trailers)
}

// Subject devolve a primeira linha da mensagem.
func (c LogCommit) Subject() string {
	if i := strings.IndexByte(c.Body, '\n'); i >= 0 {
		return c.Body[:i]
	}
	return c.Body
}

// Short devolve o SHA abreviado para mensagens.
func (c LogCommit) Short() string {
	if len(c.SHA) > 7 {
		return c.SHA[:7]
	}
	return c.SHA
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

// Log devolve os commits de um intervalo (ou revisão), do mais novo ao mais
// antigo por padrão. rng pode ser "base..head" ou um ref único.
func (g *Git) Log(rng string, reverse bool) ([]LogCommit, error) {
	args := []string{"log", "--format=%H" + fieldSep + "%P" + fieldSep + "%aI" + fieldSep + "%B" + recordSep}
	if reverse {
		args = append(args, "--reverse")
	}
	args = append(args, rng)
	out, err := g.Run(args...)
	if err != nil {
		return nil, err
	}
	var commits []LogCommit
	for _, rec := range strings.Split(out, recordSep) {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, fieldSep, 4)
		if len(parts) < 4 {
			continue
		}
		var parents []string
		if p := strings.TrimSpace(parts[1]); p != "" {
			parents = strings.Fields(p)
		}
		date, _ := time.Parse(time.RFC3339, strings.TrimSpace(parts[2]))
		commits = append(commits, LogCommit{
			SHA:        strings.TrimSpace(parts[0]),
			Parents:    parents,
			AuthorDate: date,
			Body:       strings.Trim(parts[3], "\n"),
		})
	}
	return commits, nil
}

// UnmergedFiles devolve os arquivos em conflito no índice do worktree.
func (g *Git) UnmergedFiles() ([]string, error) {
	out, err := g.Run("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return Lines(out), nil
}

// CherryPick aplica os commits em sequência preservando a procedência (-x).
// Commits que ficam vazios são mantidos, para que a procedência fique registrada.
// Num conflito, para com o cherry-pick em andamento (`--continue` segue a sequência).
func (g *Git) CherryPick(shas ...string) error {
	args := append([]string{"cherry-pick", "-x", "--keep-redundant-commits"}, shas...)
	return g.RunOK(args...)
}

// CherryPickAbort aborta um cherry-pick em andamento (ignora erro se não houver).
func (g *Git) CherryPickAbort() {
	_ = g.RunOK("cherry-pick", "--abort")
}

// CherryPickInProgress informa se há um cherry-pick parado (conflito ou
// sequência interrompida) neste working tree.
func (g *Git) CherryPickInProgress() bool {
	if g.RunOK("rev-parse", "-q", "--verify", "CHERRY_PICK_HEAD") == nil {
		return true
	}
	seq, err := g.Run("rev-parse", "--git-path", "sequencer")
	if err != nil {
		return false
	}
	if !filepath.IsAbs(seq) {
		seq = filepath.Join(g.dir, seq)
	}
	_, err = os.Stat(seq)
	return err == nil
}

// LocalOnly devolve os commits de rev que não estão em nenhuma branch do origin.
func (g *Git) LocalOnly(rev string) ([]string, error) {
	out, err := g.Run("rev-list", rev, "--not", "--remotes=origin")
	if err != nil {
		return nil, err
	}
	return Lines(out), nil
}

// CreateBranch cria branch em startPoint sem fazer checkout.
func (g *Git) CreateBranch(branch, startPoint string) error {
	return g.RunOK("branch", branch, startPoint)
}

// Commit cria um commit com o índice atual.
func (g *Git) Commit(message string) error {
	return g.RunOK("commit", "-m", message)
}

// SwitchReset faz checkout de branch no working tree, criando-a ou movendo-a
// para startPoint.
func (g *Git) SwitchReset(branch, startPoint string) error {
	return g.RunOK("switch", "-C", branch, startPoint)
}

// Push envia refspec para origin sem rodar hooks: os pre-push dos repos validam
// a feature com as dependências instaladas, o que não vale para a branch do
// ambiente (nem existe no worktree temporário); o ambiente valida no CI.
func (g *Git) Push(refspec string) error {
	return g.RunOK("push", "--no-verify", "origin", refspec)
}

// Rebase rebaseia a branch corrente sobre upstream (pode conflitar → erro).
func (g *Git) Rebase(upstream string) error {
	return g.RunOK("rebase", upstream)
}

// RebaseAbort aborta um rebase em andamento.
func (g *Git) RebaseAbort() { _ = g.RunOK("rebase", "--abort") }

// TrackedChanges informa se há mudanças em arquivos rastreados (untracked não contam).
func (g *Git) TrackedChanges() (bool, error) {
	out, err := g.Run("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}
