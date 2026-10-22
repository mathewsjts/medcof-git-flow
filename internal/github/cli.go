package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CLI é a implementação real de Client sobre o executável `gh`.
type CLI struct {
	dir string // repositório de contexto (gh usa o remote do cwd)
}

// NewCLI cria um cliente gh que opera no diretório informado.
func NewCLI(dir string) *CLI { return &CLI{dir: dir} }

func (c *CLI) run(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GH_PROMPT_DISABLED=1", "NO_COLOR=1")
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return bytes.TrimRight(stdout.Bytes(), "\r\n"), nil
}

// AuthStatus falha se o gh não estiver autenticado.
func (c *CLI) AuthStatus() error {
	if _, err := c.run("auth", "status"); err != nil {
		return fmt.Errorf("gh não autenticado: rode `gh auth login` (%w)", err)
	}
	return nil
}

const prJSONFields = "number,title,state,baseRefName,headRefName,reviewDecision,url"

// prJSON espelha a saída de `gh pr view/list --json`.
type prJSON struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	State          string `json:"state"`
	BaseRefName    string `json:"baseRefName"`
	HeadRefName    string `json:"headRefName"`
	ReviewDecision string `json:"reviewDecision"`
	URL            string `json:"url"`
}

func (p prJSON) toPR() PR {
	return PR{
		Number:         p.Number,
		Title:          p.Title,
		State:          p.State,
		BaseRefName:    p.BaseRefName,
		HeadRefName:    p.HeadRefName,
		ReviewDecision: p.ReviewDecision,
		URL:            p.URL,
	}
}

// PR devolve um PR pelo número.
func (c *CLI) PR(number int) (PR, error) {
	out, err := c.run("pr", "view", strconv.Itoa(number), "--json", prJSONFields)
	if err != nil {
		return PR{}, err
	}
	var p prJSON
	if err := json.Unmarshal(out, &p); err != nil {
		return PR{}, fmt.Errorf("JSON inesperado de `gh pr view %d`: %w", number, err)
	}
	return p.toPR(), nil
}

// PRForBranch devolve o PR aberto com head em branch e base em base.
func (c *CLI) PRForBranch(branch, base string) (PR, bool, error) {
	out, err := c.run("pr", "list", "--head", branch, "--base", base, "--state", "open", "--limit", "1", "--json", prJSONFields)
	if err != nil {
		return PR{}, false, err
	}
	var ps []prJSON
	if err := json.Unmarshal(out, &ps); err != nil {
		return PR{}, false, fmt.Errorf("JSON inesperado de `gh pr list --head %s`: %w", branch, err)
	}
	if len(ps) == 0 {
		return PR{}, false, nil
	}
	return ps[0].toPR(), true, nil
}
