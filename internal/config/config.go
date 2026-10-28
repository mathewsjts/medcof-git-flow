// Package config carrega a configuração embutida (go:embed) e resolve qual
// repositório gerenciado corresponde ao `origin` do repo local.
package config

import (
	_ "embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var embedded []byte

// Repo descreve um repositório gerenciado pelo flow.
type Repo struct {
	// Name é "owner/repo" (preenchido no Load a partir da chave do mapa).
	Name string `yaml:"-"`
	// Environments mapeia nome lógico (ex.: "dev", "hom") para a branch real (ex.: "develop").
	Environments map[string]string `yaml:"environments"`
	// Production é a branch de produção (deploy via merge do PR, fora do CLI).
	Production string `yaml:"production"`
}

// Branch devolve a branch real de um ambiente lógico e se ele existe.
func (r Repo) Branch(env string) (string, bool) {
	b, ok := r.Environments[env]
	return b, ok
}

// Envs devolve os nomes lógicos de ambiente em ordem estável (dev antes de hom).
func (r Repo) Envs() []string {
	envs := make([]string, 0, len(r.Environments))
	for e := range r.Environments {
		envs = append(envs, e)
	}
	sort.Slice(envs, func(i, j int) bool {
		// "dev" sempre antes de "hom"; demais em ordem alfabética.
		rank := func(s string) int {
			switch s {
			case "dev":
				return 0
			case "hom":
				return 1
			default:
				return 2
			}
		}
		if ri, rj := rank(envs[i]), rank(envs[j]); ri != rj {
			return ri < rj
		}
		return envs[i] < envs[j]
	})
	return envs
}

// Config é a configuração completa embutida.
type Config struct {
	Repos map[string]Repo `yaml:"repos"`
}

// Load decodifica a configuração embutida.
func Load() (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(embedded, &c); err != nil {
		return nil, fmt.Errorf("config embutida inválida: %w", err)
	}
	for name, r := range c.Repos {
		r.Name = name
		c.Repos[name] = r
	}
	return &c, nil
}

// Lookup resolve o repo gerenciado a partir de "owner/repo".
func (c *Config) Lookup(ownerRepo string) (Repo, bool) {
	r, ok := c.Repos[ownerRepo]
	return r, ok
}

// Names devolve os nomes dos repos gerenciados em ordem alfabética.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Repos))
	for n := range c.Repos {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
