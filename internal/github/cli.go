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

const prJSONFields = "number,title,state,baseRefName,headRefName,reviewDecision,author,mergedAt,url,commits"

// prJSON espelha a saída de `gh pr view --json`.
type prJSON struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	State          string `json:"state"`
	BaseRefName    string `json:"baseRefName"`
	HeadRefName    string `json:"headRefName"`
	ReviewDecision string `json:"reviewDecision"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	MergedAt string `json:"mergedAt"`
	URL      string `json:"url"`
	Commits  []struct {
		OID             string `json:"oid"`
		MessageHeadline string `json:"messageHeadline"`
	} `json:"commits"`
}

func (p prJSON) toPR() PR {
	pr := PR{
		Number:         p.Number,
		Title:          p.Title,
		State:          p.State,
		BaseRefName:    p.BaseRefName,
		HeadRefName:    p.HeadRefName,
		ReviewDecision: p.ReviewDecision,
		Author:         p.Author.Login,
		Merged:         p.MergedAt != "" && p.MergedAt != "null",
		URL:            p.URL,
	}
	for _, cm := range p.Commits {
		pr.Commits = append(pr.Commits, Commit{SHA: cm.OID, Headline: cm.MessageHeadline})
	}
	return pr
}

// CurrentPR devolve o PR da branch corrente.
func (c *CLI) CurrentPR() (PR, error) {
	out, err := c.run("pr", "view", "--json", prJSONFields)
	if err != nil {
		return PR{}, err
	}
	var p prJSON
	if err := json.Unmarshal(out, &p); err != nil {
		return PR{}, fmt.Errorf("JSON inesperado de `gh pr view`: %w", err)
	}
	return p.toPR(), nil
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

// CommitPRs devolve os PRs associados a um commit via API REST.
func (c *CLI) CommitPRs(repo, sha string) ([]CommitPR, error) {
	out, err := c.run("api", fmt.Sprintf("repos/%s/commits/%s/pulls", repo, sha))
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("JSON inesperado de commits/%s/pulls: %w", sha, err)
	}
	prs := make([]CommitPR, 0, len(raw))
	for _, r := range raw {
		prs = append(prs, CommitPR{Number: r.Number, State: r.State, Merged: r.Merged, Author: r.User.Login})
	}
	return prs, nil
}

// CommitAuthor devolve o login do autor de um commit (ou o nome, se sem login).
func (c *CLI) CommitAuthor(repo, sha string) (string, error) {
	out, err := c.run("api", fmt.Sprintf("repos/%s/commits/%s", repo, sha))
	if err != nil {
		return "", err
	}
	var raw struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return "", fmt.Errorf("JSON inesperado de commits/%s: %w", sha, err)
	}
	if raw.Author.Login != "" {
		return raw.Author.Login, nil
	}
	return raw.Commit.Author.Name, nil
}
