# Catálogo de ferramentas

Fase 4, depois da IA (ver [roadmap.md](roadmap.md)). O noct **não instala nada sozinho**. Ele sabe que ferramentas existem, detecta quais estão instaladas e monta o comando de instalação para o gerenciador que o usuário prefere, rodando-o num painel visível.

## Manifesto de ferramenta

```yaml
name: hurl
bin: hurl                      # binário usado para detectar se está instalado
description: Roda e testa requisições HTTP descritas em texto
tags: [http, testes, api]
homepage: https://hurl.dev
version_cmd: hurl --version    # opcional, para mostrar a versão
install:
  pacman: hurl
  apt: hurl
  brew: hurl
  cargo: hurl
  paru: hurl
```

- Manifestos embutidos no binário formam a lista curada de **recomendados** para testes (ex.: jq, yq, hurl, httpie, k6, ripgrep, fd, fzf, bat, lazygit).
- O usuário adiciona os próprios em `~/.local/share/noct/catalog/`; projetos em `.noct/catalog/`.

## Adaptadores

Cada gerenciador é um adaptador com a mesma interface:

| Método | Exemplo (pacman) |
|---|---|
| `Available()` | `pacman` existe no PATH |
| `InstallCmd(pkg)` | `sudo pacman -S --needed <pkg>` |
| `SearchCmd(q)` | `pacman -Ss <q>` |
| `NeedsRoot()` | sim |

Adaptadores da v1: `apt`, `dnf`, `pacman`, `paru`, `yay`, `brew`, `go install`, `cargo install`.

A ordem de preferência fica em `config.toml`:

```toml
[catalog]
managers = ["paru", "pacman", "cargo", "go"]
```

O primeiro gerenciador disponível que tem o pacote no manifesto é o sugerido; o usuário pode trocar antes de confirmar.

## Fluxo de instalação

1. O usuário escolhe instalar (no catálogo, ou porque um script ou fluxo usa um programa que não está instalado).
2. O noct mostra o comando exato que vai rodar.
3. O comando roda num **painel novo**, então `sudo`, prompts do paru e confirmações funcionam normalmente, porque é um terminal de verdade.
4. Ao terminar, o noct detecta de novo e atualiza o estado.

## Busca

- A busca local usa nome, descrição e tags dos manifestos (fuzzy).
- Para buscar além do catálogo, o noct roda o `SearchCmd` do gerenciador escolhido num painel e mostra o resultado. A v1 não interpreta a saída de cada gerenciador; isso pode vir depois, por adaptador.

## Detecção

A detecção usa `exec.LookPath(bin)`, com cache e reavaliada ao abrir o catálogo ou depois de instalar. Não consulta o banco de pacotes: interessa se o binário está disponível, venha de onde vier.
