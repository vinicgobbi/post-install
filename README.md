# POST — Pós-instalação com TUI

Binário em Go, com interface interativa no terminal (TUI), para configurar uma
workstation Linux recém-instalada: mirrors, atualização do sistema, Docker,
VSCode, PHP + SQL Server, Flatpaks (incluindo jogos — Steam, Heroic,
ProtonPlus, PrismLauncher), Tailscale + Trayscale, Chrome, Bitwarden,
temas/ícones GNOME, o ambiente do usuário (zsh, Oh My Zsh, fnm/Node, dotfiles
pessoais), Rust (rustup, eza, topgrade), Claude Code, importação de perfis
OpenVPN e as extensões "Abrir com o VSCode"/"Copiar caminho" no Nautilus.

## Sistemas suportados

| Família  | Distribuições                                  | Gerenciador |
|----------|------------------------------------------------|-------------|
| Fedora   | Fedora Linux                                   | `dnf`       |
| RHEL     | AlmaLinux e derivados (`ID_LIKE=rhel/centos`)  | `dnf`       |
| Arch     | Arch Linux puro (`ID=arch`)                    | `pacman`    |
| Debian   | Ubuntu (bionic, focal, jammy, noble) e derivados | `apt`     |
| Debian   | Debian (bullseye, bookworm)                    | `apt`       |

Se a distribuição/versão não estiver nessa lista, o programa para com uma
mensagem clara em vez de continuar em um estado incerto. No Arch isso inclui
derivados como Manjaro/EndeavourOS/CachyOS: cada um diverge de um jeito
diferente do Arch puro, então não são reconhecidos por `ID_LIKE` — rodar lá é
por conta e risco, sem suporte. Ubuntus mais novos que a última LTS mapeada
usam o repositório da Microsoft do 24.04 (compatível), com um aviso.

### Arch Linux: Chaotic-AUR e yay

Como o Arch não tem repositório empacotado próprio para VSCode, Google Chrome,
Bitwarden nativo ou os pacotes MS SQL, o módulo `repositorios` configura o
[Chaotic-AUR](https://aur.chaotic.cx/) — repositório binário com pacotes da
AUR pré-compilados — e instala o `yay` (de lá, se disponível; senão compila da
AUR). Os demais módulos usam o `yay` para esses pacotes.

Como o `makepkg` (usado pelo `yay`) recusa rodar como root, o módulo
`repositorios` libera temporariamente sudo sem senha **só para o binário do
`pacman`** (`/etc/sudoers.d/99-post-install-aur`) para o usuário escolhido.
Essa regra é removida pelo módulo `limpeza` ao final; se você pular a
limpeza, ela fica até a próxima execução completa (ou até remover o arquivo
manualmente).

### Ubuntu: Firefox e Thunderbird sem snap

No Ubuntu, os pacotes `firefox` e `thunderbird` do repositório oficial são só
pacotes de transição que instalam a versão snap. Por isso, no Ubuntu (e
sabores com `ID=ubuntu`, como o Kubuntu):

- o módulo `repositorios` segue a instrução oficial da Mozilla para o
  [repositório APT](https://packages.mozilla.org/apt) dela: chave em
  `/etc/apt/keyrings/packages.mozilla.org.asc`, conferência do fingerprint
  (`35BAA0B33E9EB396F59CA838C0BA5CE6DC6315A3`), repositório com as suítes
  `mozilla` (Firefox) e `thunderbird-deb` (Thunderbird) — em
  `mozilla.sources` (deb822) do Ubuntu 26.04 em diante, ou `mozilla.list` no
  24.04 e anteriores — e `/etc/apt/preferences.d/mozilla` com prioridade 1000
  para `packages.mozilla.org`;
- o módulo `pacotes_base` instala os `.deb` da Mozilla (com
  `--allow-downgrades`, já que o pacote de transição tem epoch maior) e, depois,
  remove os snaps de ambos, se instalados. Antes de remover, copia o perfil do
  snap para `~/.mozilla/firefox` / `~/.thunderbird` quando o usuário ainda não
  tem um. A remoção é sem `--purge`, então o snapd guarda um snapshot por
  alguns dias.

Nas outras distros, os dois vêm do repositório nativo (no Debian puro,
`firefox-esr`).

## Como usar

```bash
git clone https://github.com/vinicgobbi/post-install.git
cd post-install
./setup.sh
```

O `setup.sh` se reexecuta com `sudo`, baixa o binário da última release para
`amd64` (única arquitetura suportada), confere o SHA-256 e executa
apontando `OVPN/`, `Wallpapers/` e `logs/` para a pasta do repositório. Para fixar uma
versão: `POST_INSTALL_VERSION=v1.2.0 ./setup.sh`. Qualquer flag é repassada ao
binário (ex.: `./setup.sh --dry-run`).

### Pacote da release (sem clonar o repositório)

Cada release publica um pacote com o binário e a estrutura de pastas pronta:

```
post-install-linux-amd64.tar.gz
└── post-install/
    ├── post-install      # binário
    ├── OVPN/README.md    # coloque aqui os .ovpn a importar
    ├── Wallpapers/       # papéis de parede oferecidos na TUI
    ├── logs/             # logs de cada execução
    └── README.md
```

```bash
tar -xzf post-install-linux-amd64.tar.gz
cp ~/meus-perfis/*.ovpn post-install/OVPN/   # opcional
sudo ./post-install/post-install
```

Por padrão o binário procura `OVPN/` e `Wallpapers/` e grava em `logs/`
**ao lado dele mesmo**, então funciona a partir de qualquer diretório. A release também
traz o binário solto (`post-install-linux-amd64`, usado pelo `setup.sh`) e um
`.sha256` para cada arquivo.

### Fluxo da TUI

1. Mostra o sistema detectado (distro, base, desktop GNOME/GTK ou KDE).
2. Pergunta para qual usuário do sistema configurar o ambiente (sem sugestão
   de nome; valida que o usuário existe, inclusive contas de domínio).
3. Pergunta qual papel de parede aplicar, entre as imagens de `Wallpapers/`
   (com resolução e tamanho), ou "Não alterar", que desmarca o módulo
   `papel_parede`. A tela é pulada se a pasta estiver vazia ou se vier
   `--wallpaper`.
4. Checklist com todos os módulos, já marcados:
   - `↑`/`↓` (ou `j`/`k`) — mover · `espaço` — marcar/desmarcar · `a` — todos
   - o painel abaixo mostra a descrição do módulo e as dependências
   - marcar um módulo marca as dependências dele; desmarcar uma dependência
     desmarca quem depende dela (com aviso na tela)
5. Confirmação com a lista do que vai rodar (e o papel de parede escolhido).
6. Execução: barra de progresso, status de cada módulo (✓ ok, ! com avisos,
   ✗ falhou, ⤼ pulado) e a saída ao vivo dos comandos (`↑`/`↓`/`pgup`/`pgdn`
   para rolar, `G` para voltar a seguir). `Ctrl+C` pede confirmação e então
   interrompe o comando em andamento.
7. Resumo final com tempo de cada módulo e o caminho dos logs (o resumo
   também fica impresso no terminal depois que a TUI fecha).

Se um módulo falha, os seguintes continuam rodando — exceto os que dependem
dele, que são pulados. Use `--fail-fast` para parar tudo no primeiro erro.

### Flags

| Flag | Descrição |
|------|-----------|
| `--user NOME` | usuário alvo (pula a pergunta) |
| `--modules a,b` | seleciona só esses módulos (IDs de `--list`) |
| `--skip a,b` | desmarca esses módulos |
| `--dry-run` | simula: lista comandos/arquivos sem alterar nada (não exige root) |
| `--no-tui` | saída em texto simples |
| `--yes` | não pede confirmação (modo texto) |
| `--fail-fast` | para no primeiro módulo que falhar |
| `--list` | lista os módulos e sai |
| `--wallpaper NOME` | papel de parede a aplicar (nome do arquivo, ou `none` para não alterar); sem ela, a TUI pergunta |
| `--ovpn-dir` / `--wallpapers-dir` / `--log-dir` | pastas de `.ovpn`, papéis de parede e logs (padrão: `OVPN/`, `Wallpapers/` e `logs/` ao lado do binário) |
| `--version` | mostra a versão |

Sem terminal interativo (pipe, CI), o programa usa o modo texto
automaticamente e roda todos os módulos (menos os de `--skip`); nesse caso
`--user` é obrigatório.

## Log em arquivo

Cada execução grava dois arquivos em `logs/`:

- `setup_<timestamp>.log` — só as mensagens (info/aviso/erro/sucesso/passo),
  com hora e nível. Bom para uma leitura rápida.
- `setup_<timestamp>.raw.log` — transcrição completa, incluindo cada comando
  executado e a saída crua de `apt`/`dnf`/`flatpak`/`curl`. Bom para
  investigar uma falha.

## Estrutura do projeto

```
setup.sh                    # bootstrap: baixa o binário da release e executa
cmd/post-install/main.go    # flags, verificação de root, TUI x modo texto
internal/
  sysinfo/                  # detecção de distro/versão/gerenciador/desktop
  runner/                   # execução de comandos, downloads, escrita de arquivos, dry-run
  runlog/                   # logs .log/.raw.log e repasse das mensagens à interface
  engine/                   # dependências entre módulos e execução em ordem
  modules/                  # os módulos (um arquivo por grupo)
    module.go               #   tipo Module, ordem de execução (All) e helpers
    mirrors.go              #   mirrors (dnf, reflector no Arch, teste de velocidade no Ubuntu)
    repositorios.go         #   atualização, repositórios/Chaotic-AUR+yay, pacotes base
    apps.go                 #   Solaar, LibreOffice, Flatpaks, Tailscale, PHP, Chrome+GCM, Bitwarden
    desktop.go              #   temas/ícones, ambiente do usuário, Rust, Claude Code, Nautilus/Dolphin
    wallpaper.go            #   papel de parede (cópia para ~/.local/share/backgrounds + gsettings)
    sistema.go              #   OpenVPN, virt-manager, limpeza
  config/flatpaks.go        # listas de Flatpaks (GTK, KDE, jogos)
  tui/                      # interface Bubble Tea
  plain/                    # saída em texto simples
.github/workflows/release.yml
.cz.toml                    # configuração do commitizen
```

## Adicionando ou editando um módulo

1. Declare o módulo em um dos arquivos de `internal/modules/` (ou num novo):
   ```go
   var meuModulo = Module{
       ID:    "meu_modulo",
       Title: "Título no menu",
       Desc:  "Descrição de uma linha",
       Deps:  []string{"pacotes_base"}, // opcional
       Run: func(c *Ctx) error {
           c.Log.Infof("Fazendo algo...")
           if err := c.install("pacote"); err != nil {
               return err
           }
           c.Log.Successf("Pronto.")
           return nil
       },
   }
   ```
2. Inclua-o em `All()` (`internal/modules/module.go`) na posição de execução.

Dentro de `Run`, use os helpers do `Ctx` em vez de `os/exec` direto, para a
saída ir para a TUI/logs e o `--dry-run` funcionar:

- `c.Run(...)`/`c.RunEnv(...)` — comando que altera o sistema (falha = erro)
- `c.Try(...)` — comando que altera o sistema, silencioso, devolve `bool`
- `c.Probe(...)`/`c.Output(...)` — consultas somente-leitura (rodam mesmo no dry-run)
- `c.AsUser(script)` — shell como o usuário alvo (`su - usuario -c`)
- `c.install(...)`, `c.installAUR(...)`, `c.flatpakInstall(...)`
- `c.Download(url, destino)`, `c.Temp(nome)`, `c.WriteFile(...)`, `c.AppendFile(...)`
- `c.Log.Infof/Successf/Warnf(...)` — avisos marcam o módulo como "concluído com avisos"

## Detecção de sistema (`internal/sysinfo`)

`sysinfo.Detect` lê `/etc/os-release` (e `/etc/upstream-release/lsb-release`
no Mint) e preenche:

- `Family` — `fedora`, `rhel`, `arch` ou `debian`; `PkgMgr` — `dnf`, `pacman` ou `apt`
- `RHELVersion` — só família `rhel` (ex.: `9`)
- `Arch`, `BaseCodename`, `DistroVersion`, `DockerDistro` (`ubuntu`/`debian`)
  — só família `debian`
- `MSRepoSupported` — se a Microsoft publica repositório (msodbcsql/mssql-tools)
  para essa distro+versão; se não, os módulos pulam essa parte com um aviso
- `Desktop` — `kde` se `XDG_CURRENT_DESKTOP` contém KDE ou o `plasmashell`
  está instalado (o sudo costuma limpar a variável), senão `gtk`

## Papel de parede

Os papéis de parede ficam em `Wallpapers/` (`.jpg`, `.jpeg`, `.png` ou
`.webp`) — para oferecer outra imagem, basta colocá-la na pasta, sem
recompilar. O módulo `papel_parede`, como o usuário alvo:

1. copia a imagem escolhida para `~/.local/share/backgrounds/` (o mesmo lugar
   onde o GNOME guarda as imagens escolhidas em Configurações → Plano de
   fundo);
2. aplica com `gsettings` (`picture-uri`, `picture-uri-dark` e
   `picture-options 'zoom'`) — no Plasma, com `plasma-apply-wallpaperimage`.

Os scripts que rodam como o usuário (`su - usuario`) usam a sessão D-Bus dele
quando há uma sessão gráfica aberta; sem isso o `gsettings` descarta as
alterações sem erro.

## Importação de perfis OpenVPN

Antes de rodar, copie os arquivos `.ovpn` para a pasta `OVPN/` (veja
`OVPN/README.md`). O módulo `ovpn`:

1. Garante que o plugin OpenVPN do NetworkManager esteja instalado.
2. Importa cada `OVPN/*.ovpn` como conexão do NetworkManager (reimportando do
   zero em reexecuções, para não duplicar).
3. Se o arquivo declarar `dhcp-option DNS` e/ou `dhcp-option DOMAIN`
   (`DOMAIN-SEARCH`), aplica esses valores na conexão via `nmcli`.

Se `OVPN/` estiver vazia, o módulo é pulado com um aviso.

## Desenvolvimento

Requer Go (versão em `go.mod`).

```bash
go test ./...
go build -o post-install ./cmd/post-install   # o setup.sh usa ./post-install se existir
./post-install --dry-run                       # testa a TUI sem root e sem alterar nada
```

## Versionamento e releases

Os commits seguem [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `refactor:`, `feat!:` ...). A cada push na `main`, a action
`.github/workflows/release.yml`:

1. roda `go vet` e `go test`;
2. usa o [commitizen](https://commitizen-tools.github.io/commitizen/) para
   calcular a próxima versão (`fix` → patch, `feat` → minor, `!`/`BREAKING
   CHANGE` → major), atualizar o `CHANGELOG.md`, criar o commit `bump: ...` e a
   tag `vX.Y.Z` — sem commit `feat`/`fix`/breaking desde a última tag, para aqui;
3. compila o binário estático (`CGO_ENABLED=0`) para `amd64`, com a
   versão embutida, empacota o `.tar.gz` com o binário + `OVPN/` + `Wallpapers/` + `logs/` e
   sobe tudo como artefato;
4. publica a release da tag com os binários soltos, os pacotes `.tar.gz` e os
   `.sha256`, e apaga os artefatos da action.

## Requisitos

- Acesso root (`sudo`) — exceto em `--dry-run`.
- `curl` ou `wget` para o `setup.sh` baixar o binário.
- Terminal interativo para a TUI (sem ele, o modo texto é usado).
