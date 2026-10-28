// Package ui centraliza a formatação das mensagens ao usuário (em pt-BR).
package ui

import (
	"fmt"
	"io"
)

// Printer escreve mensagens formatadas num writer.
type Printer struct{ W io.Writer }

// New cria um Printer.
func New(w io.Writer) *Printer { return &Printer{W: w} }

// Info imprime uma linha informativa.
func (p *Printer) Info(format string, a ...any) {
	fmt.Fprintf(p.W, format+"\n", a...)
}

// Step imprime um passo em andamento ("… ").
func (p *Printer) Step(format string, a ...any) {
	fmt.Fprintf(p.W, "→ "+format+"\n", a...)
}

// OK imprime sucesso.
func (p *Printer) OK(format string, a ...any) {
	fmt.Fprintf(p.W, "✓ "+format+"\n", a...)
}

// Fail imprime falha.
func (p *Printer) Fail(format string, a ...any) {
	fmt.Fprintf(p.W, "✗ "+format+"\n", a...)
}

// Warn imprime um aviso.
func (p *Printer) Warn(format string, a ...any) {
	fmt.Fprintf(p.W, "! "+format+"\n", a...)
}
