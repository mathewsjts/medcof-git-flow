package flow

import (
	"fmt"
	"strings"
)

// reportConflict imprime o diagnóstico de um conflito e devolve erro.
// (A classificação A–D completa é adicionada no passo de diagnóstico.)
func (a *App) reportConflict(c *Conflict) error {
	a.UI.Fail("Conflito ao aplicar PR #%d em %s", c.PR, c.Env)
	if len(c.Files) > 0 {
		a.UI.Info("  Arquivos em conflito:")
		for _, f := range c.Files {
			a.UI.Info("    %s", f)
		}
	}
	return fmt.Errorf("conflito em %s (arquivos: %s)", c.Env.Logical, strings.Join(c.Files, ", "))
}
