package config

import "testing"

func TestNormalizeRemote(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"git@github.com:MedCof-Tech/app-apollo-api.git", "MedCof-Tech/app-apollo-api", false},
		{"git@github.com:MedCof-Tech/app-apollo-api", "MedCof-Tech/app-apollo-api", false},
		{"https://github.com/MedCof-Tech/app-apollo-api.git", "MedCof-Tech/app-apollo-api", false},
		{"https://github.com/MedCof-Tech/app-apollo-api", "MedCof-Tech/app-apollo-api", false},
		{"https://github.com/MedCof-Tech/app-apollo-api/", "MedCof-Tech/app-apollo-api", false},
		{"ssh://git@github.com/MedCof-Tech/app-apollo-api.git", "MedCof-Tech/app-apollo-api", false},
		{"https://user:token@github.com/MedCof-Tech/app-apollo-api.git", "MedCof-Tech/app-apollo-api", false},
		{"  git@github.com:MedCof-Tech/app-apollo-api.git\n", "MedCof-Tech/app-apollo-api", false},
		{"", "", true},
		{"not-a-remote", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeRemote(c.in)
		if c.err {
			if err == nil {
				t.Errorf("NormalizeRemote(%q): esperava erro, veio %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeRemote(%q): erro inesperado: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeRemote(%q) = %q, queria %q", c.in, got, c.want)
		}
	}
}

func TestLoadEmbedded(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r, ok := c.Lookup("MedCof-Tech/app-apollo-api")
	if !ok {
		t.Fatal("app-apollo-api não encontrado na config embutida")
	}
	if r.Name != "MedCof-Tech/app-apollo-api" {
		t.Errorf("Name = %q", r.Name)
	}
	if b, _ := r.Branch("dev"); b != "dev" {
		t.Errorf("dev branch = %q, queria dev", b)
	}
	if !r.Shared {
		t.Error("apollo deveria ser shared")
	}
	if len(r.Verify) == 0 {
		t.Error("apollo deveria ter comandos verify")
	}
	// aulas-ui: dev lógico mapeia para develop.
	au, ok := c.Lookup("MedCof-Tech/app-aulas-ui")
	if !ok {
		t.Fatal("app-aulas-ui não encontrado")
	}
	if b, _ := au.Branch("dev"); b != "develop" {
		t.Errorf("aulas-ui dev branch = %q, queria develop", b)
	}
	if got := r.Envs(); len(got) != 2 || got[0] != "dev" || got[1] != "hom" {
		t.Errorf("Envs() = %v, queria [dev hom]", got)
	}
}
