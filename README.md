# flow

CLI de bolso que leva os commits de uma branch de feature (com base em `main`)
para as branches de ambiente (`dev`/`hom`) por **cherry-pick** e faz push direto.
O merge do PR em `main` (pelo GitHub) continua sendo o deploy de produção.

É um MVP provisório, exclusivo do nosso time, até entrar o deploy por label. Toda
a configuração é **embutida no binário** (`go:embed`, em
[internal/config/config.yaml](internal/config/config.yaml)): não há arquivo de
config nos repositórios das apps.

## Pré-requisitos

- `git` ≥ 2.36
- `gh` autenticado (`gh auth login`)

## Instalação

Baixe o binário do seu sistema na página de *Releases* e coloque no `PATH`.
Para compilar localmente:

```sh
make build        # gera ./flow
make dist         # binários para windows/amd64, darwin/arm64, darwin/amd64, linux/amd64
```

## Comandos

`[pr|branch]` aceita o PR (`123`, `#123` ou a URL dele) ou o nome da branch. Sem
argumento, usa a branch atual.

| Comando | O que faz |
|---|---|
| `flow setup` | Checa `git` e `gh`. |
| `flow dev [pr\|branch]` | Aplica em **dev** os commits da branch que ainda não estão lá e faz push. A branch não precisa ter PR. |
| `flow hom [pr\|branch]` | Igual ao `dev`, em **hom**. Exige PR aberto para `main` com ao menos um **approve** e sem mudanças pedidas (não exige o mínimo de aprovações nem code owner do merge). |
| `flow status [pr\|branch]` | Para cada ambiente: aplicado, parcial (lista o que falta) ou ausente. |

As mensagens mostram o nome lógico e a branch real, ex.: `dev (develop)`.

### O que é aplicado

Os commits de `origin/main..origin/<branch>`, menos os que o ambiente já tem:

- commits que já estão na história do ambiente (ex.: mergeados lá pelo fluxo antigo);
- commits com o mesmo patch (ex.: depois de um rebase da feature em `main`);
- commits já aplicados pelo flow, reconhecidos pela linha
  `(cherry picked from commit <sha>)` que o `cherry-pick -x` grava, inclusive
  quando a aplicação passou por resolução de conflito.

Só entra o que está no `origin`: se a sua branch local tiver commits não
enviados, o flow avisa e eles ficam de fora.

### Tarefas dependentes

Uma feature pode ser criada em cima de outra que partiu de `main`. Os commits da
dependência vão junto quando ainda não estão no ambiente.

O flow **recusa** a branch que tem merge commits ou commits com
`cherry picked from` em relação a `main`: é sinal de que ela foi criada a partir
de `dev`/`hom` (ou fez merge deles), e levaria a história do ambiente para os
outros. A mensagem lista esses commits; a correção é refazer a base em
`origin/main`.

## Conflito

Sem conflito, tudo acontece num worktree temporário e o seu working tree não é
tocado. Quando um cherry-pick conflita, nada é enviado e o flow abre o conflito
**no seu working tree**: faz checkout da branch do ambiente alinhada a
`origin/<branch>`, reaplica os commits e para no que conflitou. Uma cópia local
antiga da branch do ambiente é descartada; se ela tiver commits que não estão em
nenhuma branch do origin, eles ficam guardados em `flow-backup/<branch>-<data>`.
A partir daí é git puro:

```sh
# edite os arquivos e remova os marcadores de conflito
git add <arquivos>
git cherry-pick --continue --no-edit   # commita e segue com os restantes (pode parar de novo)
flow dev 123                           # envia a dev e aplica o que faltar
```

Não rode `git commit`: o `--continue` já faz o commit, mantendo a mensagem
original e a linha `(cherry picked from commit X)` que o flow usa para saber o
que já foi aplicado. O `--no-edit` evita abrir o editor.

Ao re-rodar, o flow vê que a sua branch local do ambiente está à frente do
`origin` só com cherry-picks dessa feature, faz o push dela e aplica o que
faltar. Dá para re-rodar de dentro da branch do ambiente (informando o PR ou a
branch) ou depois de voltar para a feature (`git switch <feature>` e `flow dev`).

Para desistir: `git cherry-pick --abort`. O remoto não muda até o push.

Para abrir o conflito, o flow exige que:

- o working tree não tenha mudanças em arquivos rastreados (faça commit ou `git stash`);
- a branch local do ambiente, se existir, não tenha commits que não estão no remoto.

A resolução no ambiente é provisória. A que vai para `main` é sempre feita
**na branch de feature**.
