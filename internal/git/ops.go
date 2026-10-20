package git

import (
	"fmt"
	"strings"
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

// MergeBase devolve o merge-base entre duas revisões.
func (g *Git) MergeBase(a, b string) (string, error) {
	return g.Run("merge-base", a, b)
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

// HasMerges informa se há merge commits no intervalo.
func (g *Git) HasMerges(rng string) (bool, error) {
	out, err := g.Run("rev-list", "--merges", rng)
	if err != nil {
		return false, err
	}
	return len(Lines(out)) > 0, nil
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
	SHA     string
	Parents []string
	Body    string // mensagem completa (subject + corpo + trailers)
}

// Subject devolve a primeira linha da mensagem.
func (c LogCommit) Subject() string {
	if i := strings.IndexByte(c.Body, '\n'); i >= 0 {
		return c.Body[:i]
	}
	return c.Body
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

// Log devolve os commits de um intervalo (ou revisão), do mais novo ao mais
// antigo por padrão. rng pode ser "base..head" ou um ref único.
func (g *Git) Log(rng string, reverse bool) ([]LogCommit, error) {
	args := []string{"log", "--format=%H" + fieldSep + "%P" + fieldSep + "%B" + recordSep}
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
		parts := strings.SplitN(rec, fieldSep, 3)
		if len(parts) < 3 {
			continue
		}
		var parents []string
		if p := strings.TrimSpace(parts[1]); p != "" {
			parents = strings.Fields(p)
		}
		commits = append(commits, LogCommit{
			SHA:     strings.TrimSpace(parts[0]),
			Parents: parents,
			Body:    strings.Trim(parts[2], "\n"),
		})
	}
	return commits, nil
}

// MergeTreeConflicts informa se o merge de branch sobre base conflita, sem tocar
// no working tree (git merge-tree --write-tree). Requer git >= 2.38.
func (g *Git) MergeTreeConflicts(base, branch string) (bool, error) {
	_, err := g.Run("merge-tree", "--write-tree", "--no-messages", base, branch)
	if err == nil {
		return false, nil
	}
	// merge-tree sai com código 1 em caso de conflito (não é erro de execução).
	if ce, ok := err.(*CmdError); ok {
		if ee, ok := ce.Err.(interface{ ExitCode() int }); ok && ee.ExitCode() == 1 {
			return true, nil
		}
	}
	return false, err
}

// UnmergedFiles devolve os arquivos em conflito no índice do worktree.
func (g *Git) UnmergedFiles() ([]string, error) {
	out, err := g.Run("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return Lines(out), nil
}

// CherryPick aplica um commit preservando a procedência (-x).
func (g *Git) CherryPick(sha string) error {
	return g.RunOK("cherry-pick", "-x", sha)
}

// CherryPickAbort aborta um cherry-pick em andamento (ignora erro se não houver).
func (g *Git) CherryPickAbort() {
	_ = g.RunOK("cherry-pick", "--abort")
}

// AmendTrailers adiciona trailers ao commit HEAD (via commit --amend --trailer).
// Cada trailer vem como "Chave: valor".
func (g *Git) AmendTrailers(trailers ...string) error {
	args := []string{"commit", "--amend", "--no-edit"}
	for _, t := range trailers {
		args = append(args, "--trailer", t)
	}
	return g.RunOK(args...)
}

// CommitAllowEmptyWithTrailers cria um commit (índice atual) com mensagem e trailers.
func (g *Git) Commit(message string, trailers ...string) error {
	args := []string{"commit", "-m", message}
	for _, t := range trailers {
		args = append(args, "--trailer", t)
	}
	return g.RunOK(args...)
}

// Revert reverte um commit sem abrir editor; pode gerar conflito (retorna erro).
func (g *Git) Revert(sha string) error {
	return g.RunOK("revert", "--no-edit", sha)
}

// RevertContinueAbort utilitários para revert em conflito.
func (g *Git) RevertAbort() { _ = g.RunOK("revert", "--abort") }

// ResetHard move a branch corrente para ref, descartando mudanças.
func (g *Git) ResetHard(ref string) error {
	return g.RunOK("reset", "--hard", ref)
}

// Push envia refspec para origin. forceWithLease usa --force-with-lease.
func (g *Git) Push(refspec string, forceWithLease bool) error {
	args := []string{"push"}
	if forceWithLease {
		args = append(args, "--force-with-lease")
	}
	args = append(args, "origin", refspec)
	return g.RunOK(args...)
}

// Rebase rebaseia a branch corrente sobre upstream (pode conflitar → erro).
func (g *Git) Rebase(upstream string) error {
	return g.RunOK("rebase", upstream)
}

// RebaseAbort aborta um rebase em andamento.
func (g *Git) RebaseAbort() { _ = g.RunOK("rebase", "--abort") }

// ShowDiff devolve o diff de um ref (ex.: para dar contexto no resolve).
func (g *Git) ShowDiff(ref string) (string, error) {
	return g.Run("show", "--no-color", ref)
}

// DiffRange devolve o diff de um intervalo de commits.
func (g *Git) DiffRange(rng string) (string, error) {
	return g.Run("diff", "--no-color", rng)
}

// CommitTouchesAny informa se um commit altera algum dos arquivos dados.
func (g *Git) CommitTouchesAny(sha string, files map[string]bool) (bool, error) {
	out, err := g.Run("show", "--name-only", "--format=", sha)
	if err != nil {
		return false, err
	}
	for _, f := range Lines(out) {
		if files[f] {
			return true, nil
		}
	}
	return false, nil
}

// EnsureClean falha se o worktree tiver mudanças pendentes (proteção extra).
func (g *Git) EnsureClean() error {
	out, err := g.Run("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("working tree não está limpo")
	}
	return nil
}
