# POST — Pós-instalação com TUI

Binário em Go, com interface interativa no terminal (TUI), para configurar uma
workstation Linux recém-instalada: mirrors, atualização do sistema, Docker,
VSCode, PHP + SQL Server, ferramentas de terminal (gh, nmap, pipx, ffmpeg,
bashtop, FUSE 2 para AppImages), Flatpaks (incluindo jogos — Steam, Heroic,
ProtonPlus, PrismLauncher), Tailscale + Trayscale, Chrome, Bitwarden,
temas/ícones GNOME, extensões e atalhos do GNOME, o ambiente do usuário (zsh,
Oh My Zsh, fnm/Node, dotfiles pessoais), Rust (rustup, eza, topgrade), Claude
Code, deploy-dev, importação de perfis OpenVPN e as extensões "Abrir com o
VSCode"/"Copiar caminho" no Nautilus.

## Sistemas suportados

| Família  | Distribuições                                  | Gerenciador |
|----------|------------------------------------------------|-------------|
| Fedora   | Fedora Linux                                   | `dnf`       |
| RHEL     | AlmaLinux e derivados (`ID_LIKE=rhel/centos`)  | `dnf`       |
| Arch     | Arch Linux puro (`ID=arch`)                    | `pacman`    |
| Debian   | Ubuntu (bionic, focal, jammy, noble, resolute) e derivados | `apt` |
| Debian   | Debian (bullseye, bookworm)                    | `apt`       |

Se a distribuição/versão não estiver nessa lista, o programa para com uma
mensagem clara em vez de continuar em um estado incerto. No Arch isso inclui
derivados como Manjaro/EndeavourOS/CachyOS: cada um diverge de um jeito
diferente do Arch puro, então não são reconhecidos por `ID_LIKE` — rodar lá é
por conta e risco, sem suporte. Ubuntus mais novos que a última LTS mapeada
usam o repositório da Microsoft do 26.04 (compatível), com um aviso.

### Arch Linux: Chaotic-AUR e yay

Como o Arch não tem repositório empacotado próprio para VSCode, Google Chrome,
Bitwarden nativo ou os pacotes MS SQL, o módulo `repositorios` configura o
[Chaotic-AUR](https://aur.chaotic.cx/) — repositório binário com pacotes da
AUR pré-compilados — e instala o `yay` (de lá, se disponível; senão compila da
AUR). Os demais módulos usam o `yay` para esses pacotes.

Como o `makepkg` (usado pelo `yay`) recusa rodar como root, o módulo
`repositorios` libera temporariamente sudo sem senha **só para o binário do
`pacman`** (`/etc/sudoers.d/99-post-install-aur`) para o usuário escolhido.
Como `pacman` com argumentos livres equivale a root completo, a regra é
removida no fim de **toda** execução — com qualquer resultado (falha,
`--fail-fast`, Ctrl+C, limpeza desmarcada) — e também no início, caso tenha
sobrado de uma execução morta à força.

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

### Pacote da release

Cada release publica um pacote com o binário e a estrutura de pastas pronta:

```
post-install-linux-amd64.tar.gz
└── post-install/
    ├── post-install      # binário
    ├── OVPN/README.md    # coloque aqui os .ovpn a importar
    ├── Wallpapers/       # papéis de parede oferecidos na TUI
    ├── GNOME/            # extensões (lista e configurações) e atalhos do GNOME
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
traz o binário solto (`post-install-linux-amd64`) e um `.sha256` para cada
arquivo. Só `amd64` é suportado.

### Verificando o download

O `.sha256` só prova que o arquivo chegou inteiro (ele vem da mesma release).
Para provar que o binário foi compilado pela action deste repositório, a
partir do código da tag, cada release tem um **atestado de proveniência**
(Sigstore), verificável com o [GitHub CLI](https://cli.github.com/):

```bash
sha256sum -c post-install-linux-amd64.tar.gz.sha256
gh attestation verify post-install-linux-amd64.tar.gz --repo vinicgobbi/post-install
```

Releases anteriores a essa mudança não têm atestado.

### Fluxo da TUI

1. Mostra o sistema detectado (distro, base, desktop GNOME/GTK ou KDE).
2. Pergunta para qual usuário do sistema configurar o ambiente. O usuário
   que executou o `sudo` é detectado (`SUDO_USER`, `PKEXEC_UID` ou `logname`,
   nunca `root`) e vem como primeira opção; a segunda, "Outro usuário…", abre
   um campo para digitar o nome. Sem usuário detectado, o campo aparece direto.
   O nome é validado (inclusive contas de domínio). No modo texto, a pergunta
   é `Configurar o ambiente para o usuário 'fulano'? [S/n]`.
3. Pergunta qual papel de parede aplicar, entre as imagens de `Wallpapers/`
   (com resolução e tamanho), ou "Não alterar", que desmarca o módulo
   `papel_parede`. A tela é pulada se a pasta estiver vazia ou se vier
   `--wallpaper`.
4. Se alguma extensão do GNOME de `GNOME/extensoes.txt` não tiver versão
   compatível com o GNOME Shell da máquina, lista essas extensões
   (desmarcadas) para instalar mesmo assim, se quiser — veja
   [Extensões do GNOME](#extensões-do-gnome).
5. Checklist com todos os módulos, já marcados:
   - `↑`/`↓` (ou `j`/`k`) — mover · `espaço` — marcar/desmarcar · `a` — todos
   - o painel abaixo mostra a descrição do módulo e as dependências
   - marcar um módulo marca as dependências dele; desmarcar uma dependência
     desmarca quem depende dela (com aviso na tela)
6. Confirmação com a lista do que vai rodar (e o papel de parede e as extensões forçadas).
7. Execução: barra de progresso, status de cada módulo (✓ ok, ! com avisos,
   ✗ falhou, ⤼ pulado) e a saída ao vivo dos comandos (`↑`/`↓`/`pgup`/`pgdn`
   para rolar, `G` para voltar a seguir). `Ctrl+C` pede confirmação e então
   interrompe o comando em andamento.
8. Resumo final com tempo de cada módulo e o caminho dos logs (o resumo
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
| `--force-extensions a,b` | extensões do GNOME sem versão compatível a instalar mesmo assim (ou `none`); sem ela, pergunta |
| `--wallpaper NOME` | papel de parede a aplicar (nome do arquivo, ou `none` para não alterar); sem ela, a TUI pergunta |
| `--ovpn-dir` / `--wallpapers-dir` / `--gnome-dir` / `--log-dir` | pastas de `.ovpn`, papéis de parede, extensões do GNOME e logs (padrão: `OVPN/`, `Wallpapers/`, `GNOME/` e `logs/` ao lado do binário) |
| `--version` | mostra a versão |

Sem terminal interativo (pipe, CI), o programa usa o modo texto
automaticamente e roda todos os módulos (menos os de `--skip`); nesse caso
`--user` é obrigatório.

## Log em arquivo

Cada execução grava dois arquivos em `logs/` (permissão `0600`, com dono
transferido para o usuário alvo — o programa roda como root):

- `setup_<timestamp>.log` — só as mensagens (info/aviso/erro/sucesso/passo),
  com hora e nível. Bom para uma leitura rápida.
- `setup_<timestamp>.raw.log` — transcrição completa, incluindo cada comando
  executado e a saída crua de `apt`/`dnf`/`flatpak`/`curl`. Bom para
  investigar uma falha.

## Estrutura do projeto

```
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
    desktop.go              #   temas/ícones, ambiente do usuário, Rust, Claude Code, deploy-dev, Nautilus/Dolphin
    wallpaper.go            #   papel de parede (cópia para ~/.local/share/backgrounds + gsettings)
    gnome_extensoes.go      #   extensões do GNOME (extensions.gnome.org ou git + gsettings + dconf)
    gnome_atalhos.go        #   atalhos do GNOME (dconf)
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

## Extensões do GNOME

A pasta `GNOME/` define as extensões (editável sem recompilar):

- `extensoes.txt` — uma extensão (UUID) por linha; `#` comenta, o prefixo
  `-` mantém a extensão desativada (ex.: `-ding@rastersoft.com`) e uma URL
  git depois do UUID instala a extensão do repositório em vez do
  extensions.gnome.org (ex.: a Now Bar). Depois da URL, opcionalmente, vem o
  commit (ou tag) a instalar — recomendado, para uma mudança na branch
  principal não chegar sozinha às instalações (sem ele, o módulo avisa);
- `extensoes.dconf` — as preferências delas, no formato do `dconf dump`;
- `atalhos.dconf` — os atalhos de teclado, aplicados pelo módulo
  `atalhos_gnome` (veja [Atalhos do GNOME](#atalhos-do-gnome)).

O módulo `extensoes_gnome` (pulado no Plasma), como o usuário alvo:

1. para cada extensão que não vem com o sistema (`/usr/share/gnome-shell/extensions`),
   consulta o [extensions.gnome.org](https://extensions.gnome.org) pela versão
   compatível com o GNOME Shell da máquina, baixa e instala com
   `gnome-extensions install --force`; as que têm URL git são clonadas e
   instaladas com o `make install` do próprio repositório;
2. ativa via `gsettings` (`enabled-extensions`/`disabled-extensions`,
   mesclando com o que já estava ativo) — no Wayland, extensões novas só
   carregam no próximo login, por isso não usa `gnome-extensions enable`;
3. aplica as preferências com `dconf load /org/gnome/shell/extensions/`.

**Extensões sem versão compatível.** Antes de abrir a interface, o
post-install confere a compatibilidade de cada extensão pelo
`shell_version_map` do extensions.gnome.org (com `shell_version`, a API não
dá erro: devolve a versão mais recente, que o GNOME não carregaria). Se
alguma não tiver versão para o GNOME Shell da máquina, uma tela avisa que ela
**não será instalada automaticamente** e permite marcar as que devem ser
instaladas mesmo assim — com o aviso de que podem não funcionar
corretamente. As marcadas recebem a versão mais recente publicada e o
post-install liga `disable-extension-version-validation` (sem isso o GNOME
recusa carregá-las; vale para todas as extensões do usuário, inclusive as
instaladas depois — para reverter:
`gsettings reset org.gnome.shell disable-extension-version-validation`). No modo texto
a pergunta é feita por extensão (`[s/N]`); sem terminal, use
`--force-extensions uuid1,uuid2` (ou `none`).

Para atualizar as preferências a partir de uma máquina já configurada:
`dconf dump /org/gnome/shell/extensions/ > GNOME/extensoes.dconf` — e tire
o que for estado da máquina (dispositivos pareados, monitor, tamanho de
janela).

## Atalhos do GNOME

O módulo `atalhos_gnome` (pulado no Plasma) aplica `GNOME/atalhos.dconf` com
`dconf load /org/gnome/`:

- `Print` e `Shift+Super+S` abrem a captura interativa do Gradia (Flatpak,
  instalado pelo módulo se ainda não estiver);
- a captura nativa do GNOME sai do `Print` e a gravação de tela vai para
  `Shift+Super+R`;
- o volume sobe e desce de 2 em 2% (`volume-step`).

Para atualizar a partir de uma máquina já configurada, use
`dconf dump /org/gnome/settings-daemon/plugins/media-keys/` e
`dconf dump /org/gnome/shell/keybindings/`, e ajuste os nomes das seções
para ficarem relativos a `/org/gnome/`.

## Importação de perfis OpenVPN

Antes de rodar, copie os arquivos `.ovpn` para a pasta `OVPN/` (veja
`OVPN/README.md`). O módulo `ovpn`:

1. Garante que o plugin OpenVPN do NetworkManager esteja instalado.
2. Importa cada `OVPN/*.ovpn` como conexão do NetworkManager (reimportando do
   zero em reexecuções, para não duplicar). Só apaga uma conexão existente com
   o mesmo nome se ela for do tipo VPN; se houver uma cabeada/Wi-Fi homônima,
   o arquivo é pulado com um aviso. Também avisa quando um `.ovpn` (que
   costuma ter chave privada embutida) é legível por qualquer usuário.
3. Se o arquivo declarar `dhcp-option DNS` e/ou `dhcp-option DOMAIN`
   (`DOMAIN-SEARCH`), aplica esses valores na conexão via `nmcli`.

Se `OVPN/` estiver vazia, o módulo é pulado com um aviso.

## Desenvolvimento

Requer Go (versão em `go.mod`).

```bash
go test ./...
go build -o post-install ./cmd/post-install
./post-install --dry-run                       # testa a TUI sem root e sem alterar nada
sudo ./post-install                            # usa OVPN/, Wallpapers/, GNOME/ e logs/ do repositório
```

## Versionamento e releases

Os commits seguem [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `refactor:`, `feat!:` ...). A cada push na `main`, a action
`.github/workflows/release.yml`:

1. roda `go vet`, `go test`, `staticcheck`, `govulncheck` e `shellcheck`;
2. usa o [commitizen](https://commitizen-tools.github.io/commitizen/) para
   calcular a próxima versão (`fix` → patch, `feat` → minor, `!`/`BREAKING
   CHANGE` → major), atualizar o `CHANGELOG.md`, criar o commit `bump: ...` e a
   tag `vX.Y.Z` — sem commit `feat`/`fix`/breaking desde a última tag, para aqui;
3. compila o binário estático (`CGO_ENABLED=0`) para `amd64`, com a
   versão embutida, empacota o `.tar.gz` com o binário + `OVPN/` + `Wallpapers/` + `GNOME/` + `logs/`,
   gera o atestado de proveniência dos dois e sobe tudo como artefato;
4. publica a release da tag com os binários soltos, os pacotes `.tar.gz` e os
   `.sha256`, e apaga os artefatos da action.

### Atualização das versões fixadas

Tudo o que é fixado tem quem avise quando sai versão nova — com um PR na
`development`, que chega como notificação do GitHub:

- **Actions e módulos Go** — o Dependabot (`.github/dependabot.yml`), toda
  semana. As actions ficam fixadas pelo SHA do commit, com a tag num
  comentário.
- **Commits dos repositórios pessoais, Now Bar, Yaru e regra UDEV do
  Solaar** — o workflow `.github/workflows/pins.yml`, todo dia (ou à mão
  em Actions → Pins → Run workflow). Ele roda `scripts/atualizar-pins.sh`,
  que confere o último commit/tag de cada um, atualiza o pin (recalculando o
  SHA-256 do Yaru) e abre o PR `fix: atualiza versões fixadas` com links de
  comparação para revisar o que mudou. Se a regra do Solaar mudar, o PR
  destaca isso (ela roda como root). Precisa de *Settings → Actions → General
  → Allow GitHub Actions to create and approve pull requests*.

O script também roda localmente: `bash scripts/atualizar-pins.sh`.

## Segurança

- **Grupos com poder de root.** O usuário alvo entra nos grupos `docker`
  (módulo `ambiente_usuario`) e `libvirt`/`kvm` (módulo `virt_manager`).
  Quem está no `docker` ou no `libvirt` consegue virar root (ex.: montando
  `/` num contêiner ou numa VM) — é o comportamento normal dessas
  ferramentas numa workstation de desenvolvimento, mas vale saber.
- **Chaves de repositório conferidas.** As chaves de Docker, Microsoft,
  GitHub CLI, Mozilla, Google e Tailscale são baixadas e comparadas com o
  fingerprint esperado antes de entrarem no sistema; a do Chaotic-AUR é
  buscada pelo fingerprint completo.
- **Versões fixadas.** Os repositórios pessoais executados (Dotfiles,
  vscode_nautilus, nautilus-copypath) ficam fixados num commit e os ícones
  Yaru numa tag com SHA-256 (constantes no topo de
  `internal/modules/desktop.go`); a regra UDEV do Solaar vem embutida no
  binário (`internal/modules/42-logitech-unify-permissions.rules`). Assets de release do GitHub
  (GCM, adw-gtk3, deploy-dev) são conferidos pelo SHA-256 que o GitHub
  publica.
- **Scripts de instalação remotos.** fnm, rustup e Claude Code ainda são
  instalados pelo `curl | sh` oficial de cada um, como o usuário alvo (não
  como root). O Tailscale vem do repositório oficial assinado.
- **O que não é verificado.** O `.deb` do Chrome e os pacotes do Bitwarden
  são baixados por HTTPS dos sites oficiais, sem conferência de assinatura
  pelo post-install (o Chrome passa a se atualizar pelo repositório assinado
  do Google depois de instalado; no Fedora/RHEL ele já é instalado pelo
  repositório).

## Requisitos

- Acesso root (`sudo`) — exceto em `--dry-run`.
- Terminal interativo para a TUI (sem ele, o modo texto é usado).
