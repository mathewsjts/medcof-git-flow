package git

import "time"

// WorktreeAddDetached cria um worktree em HEAD destacado no commit dado.
// O flow trabalha destacado e só publica via push de refspec, sem deixar
// branches temporárias no repo do usuário.
func (g *Git) WorktreeAddDetached(path, commit string) error {
	return g.RunOK("worktree", "add", "-f", "--detach", path, commit)
}

// WorktreePrune remove registros de worktrees ausentes.
func (g *Git) WorktreePrune() { _ = g.RunOK("worktree", "prune") }

// WorktreeRemove remove um worktree de forma tolerante a locks (antivírus/editor
// no Windows): tenta com --force, repete e por fim faz prune. Nunca propaga erro,
// pois o worktree é descartável.
func (g *Git) WorktreeRemove(path string) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := g.RunOK("worktree", "remove", "--force", path); err == nil {
			g.WorktreePrune()
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	// Última tentativa: prune limpa o registro mesmo que o diretório persista.
	g.WorktreePrune()
}
