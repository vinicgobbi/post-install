# POST — Pós-instalação com TUI

Binário em Go, com interface interativa no terminal (TUI), para configurar uma
workstation Ubuntu, Fedora (Workstation ou KDE Plasma), Linux Mint (Cinnamon), Zorin OS (Core) ou CachyOS (KDE Plasma ou GNOME) recém-instalada: mirrors, atualização do sistema, Podman,
VSCode, Go, PHP + SQL Server, ferramentas de terminal (gh, tea, nmap, pipx, ffmpeg,
bashtop, FUSE 2 para AppImages), Flatpaks (incluindo jogos — Steam, Heroic,
ProtonPlus, PrismLauncher), Tailscale + Trayscale, Chrome, Bitwarden,
temas e ícones Yaru (com todas as variantes do tema nos apps Flatpak),
extensões e atalhos do GNOME, o ambiente do usuário (zsh,
Oh My Zsh, dotfiles pessoais), mise com Node + pnpm, Rust (eza, topgrade, cargo-update)
e Ruby (Bundler e Jekyll), Claude Code, deploy-dev, importação de perfis OpenVPN e as extensões "Abrir com o
VSCode"/"Copiar caminho" no Nautilus.

## Sistemas suportados

- **Ubuntu com GNOME** (`ID=ubuntu` no `/etc/os-release` e o `gnome-shell`
  instalado), nas **LTS com suporte padrão** — hoje **22.04 (jammy), 24.04
  (noble) e 26.04 (resolute)**;
- **Fedora Workstation** (`ID=fedora` e `VARIANT_ID=workstation`) e
  **Fedora KDE Plasma** (`VARIANT_ID=kde` e o `plasmashell` instalado), nas
  **versões em manutenção** — hoje **43, 44 e 45**. Todos os módulos existem
  no Workstation, com os mesmos IDs; veja [No Fedora](#no-fedora) o que muda
  em cada um e [No KDE Plasma](#no-kde-plasma) o que muda no KDE;
- **Linux Mint com Cinnamon** (`ID=linuxmint` e o `cinnamon` instalado) e
  **Zorin OS Core** (`ID=zorin` e o `gnome-shell` instalado), nas versões com
  **base** numa das LTS do Ubuntu da lista (pelo `UBUNTU_CODENAME`) — hoje o
  Mint 21.x e 22.x e o Zorin OS 17 e 18. Veja
  [No Linux Mint e no Zorin OS](#no-linux-mint-e-no-zorin-os);
- **CachyOS com KDE Plasma ou GNOME** (`ID=cachyos` e o `plasmashell` ou o
  `gnome-shell` instalado; com os dois, vale o KDE). É rolling, então não há
  lista de versões. Veja [No CachyOS](#no-cachyos).

A lista de versões fica em
[`internal/sysinfo/versoes.go`](internal/sysinfo/versoes.go) e é mantida
pelo workflow `versoes.yml` (veja [Versões suportadas](#versões-suportadas)).

Qualquer outro sistema faz o programa parar com uma mensagem clara em vez de
continuar em um estado incerto — inclusive outros derivados do Ubuntu
(Pop!_OS, elementary...), as outras edições do Mint e do Zorin (MATE, Xfce,
Lite e o LMDE, com base Debian), um Mint ou Zorin com base fora da lista,
sabores com outro desktop (Kubuntu, Xubuntu...), versões
fora do suporte padrão ou interinas entre as LTS suportadas, as variantes
atômicas do Fedora (Silverblue, Kinoite), o Arch puro e outros derivados
dele (Manjaro, EndeavourOS...) e o CachyOS com outros desktops (Hyprland,
Cosmic...).

**Versões mais novas que as da lista** (ex.: o Ubuntu 26.10, ou uma LTS que
saiu depois da release do post-install) podem funcionar, mas não foram
testadas: o programa avisa e pede confirmação antes de seguir (na TUI, uma
tela com "Sair" como padrão; no modo texto, `[s/N]`). Sem terminal
interativo, segue só com `--allow-untested-version` ou `--yes`; no
`--dry-run`, que não altera nada, só avisa. Nelas, o Ubuntu usa o
repositório da Microsoft da última LTS da lista (compatível).

### Firefox e Thunderbird sem snap

No Ubuntu, os pacotes `firefox` e `thunderbird` do repositório oficial são só
pacotes de transição que instalam a versão snap. Por isso:

- o módulo `firefox_thunderbird` segue a instrução oficial da Mozilla para o
  [repositório APT](https://packages.mozilla.org/apt) dela: chave em
  `/etc/apt/keyrings/packages.mozilla.org.asc`, conferência do fingerprint
  (`35BAA0B33E9EB396F59CA838C0BA5CE6DC6315A3`), repositório com as suítes
  `mozilla` (Firefox) e `thunderbird-deb` (Thunderbird) — em
  `mozilla.sources` (deb822) do Ubuntu 26.04 em diante, ou `mozilla.list` no
  24.04 e anteriores — e `/etc/apt/preferences.d/mozilla` com prioridade 1000
  para `packages.mozilla.org`;
- em seguida, instala os `.deb` da Mozilla (com
  `--allow-downgrades`, já que o pacote de transição tem epoch maior) e, depois,
  remove os snaps de ambos, se instalados. Antes de remover, copia o perfil do
  snap para `~/.mozilla/firefox` / `~/.thunderbird` quando o usuário ainda não
  tem um. A remoção é sem `--purge`, então o snapd guarda um snapshot por
  alguns dias.

## No Fedora

Os módulos têm os mesmos IDs e as mesmas flags nas duas distribuições; no
Fedora, cada um usa o `dnf` e os pacotes e repositórios RPM equivalentes.
Os repositórios de terceiros são gravados em `/etc/yum.repos.d` (com os
nomes das instruções oficiais) e as chaves, conferidas pelo fingerprint, em
`/etc/pki/rpm-gpg`; o dnf as importa na primeira instalação.

| ID | No Fedora |
|----|-----------|
| `mirrors` | `fastestmirror=True` e `max_parallel_downloads=10` em `/etc/dnf/libdnf5.conf.d/50-post-install.conf` (o Fedora já usa o metalink; o `/etc/dnf/dnf.conf` continua prevalecendo) |
| `atualizacao` / `limpeza` | `dnf upgrade --refresh` / `dnf autoremove` + `dnf clean all` |
| `ferramentas` | `gh` pelo repositório RPM oficial; `btop` (o `bashtop` não existe no Fedora), `ffmpeg-free` e `fuse-libs` |
| `shell` | igual (o `chsh` vem do `util-linux`) |
| `firefox_thunderbird` | os RPM do próprio Fedora (sem snap) |
| `chrome` | repositório RPM do Google, com a assinatura de cada pacote conferida |
| `bitwarden` | o `.rpm` oficial |
| `remover_libreoffice` | `dnf remove`, só se houver algo instalado |
| `flatpaks` / `flatpaks_jogos` | Flathub ativado e sem filtro |
| `solaar` | `solaar` + `solaar-udev` do Fedora (sem o grupo `plugdev`), com o Flatpak como alternativa |
| `git_gcm` | o `.tar.gz` da release, no mesmo layout do `.deb` (`/usr/local/share/gcm-core`) |
| `podman` | `podman` + `podman-compose` |
| `vscode` / `mise` | repositórios RPM oficiais |
| `php_sqlserver` | repositório da Microsoft para o **RHEL** (a Microsoft não publica para o Fedora; o módulo avisa), extensões em `/etc/php.d` |
| `rust` / `ruby` | dependências de compilação do Fedora |
| `tailscale` | repositório RPM oficial |
| `firewall` | a interface do firewalld, que o Fedora já traz ativo: `firewall-config` no GNOME e `plasma-firewall` no KDE (a zona padrão não muda) |
| `virt_manager` | libvirt com os daemons modulares (`virtqemud`...), o padrão do Fedora |
| `temas_yaru` | só os **ícones** Yaru-dark; o tema é o **adw-gtk3-dark** (o visual do libadwaita nos apps GTK 3), também nos Flatpaks; e os botões de **minimizar e maximizar** voltam às janelas (`button-layout` `appmenu:minimize,maximize,close`) |
| `extensoes_gnome` | no lugar da dock, do AppIndicator e do Tiling Assistant do Ubuntu, as equivalentes do extensions.gnome.org (veja [Extensões do GNOME](#extensões-do-gnome)) |
| `nautilus` | `nautilus-python` e `python3-gobject` |
| `ovpn` | `NetworkManager-openvpn-gnome`; no KDE, `NetworkManager-openvpn` e `plasma-nm-openvpn` (o diálogo do Plasma) |
| `go`, `node`, `claude_code`, `deploy_dev`, `papel_parede`, `atalhos_gnome`, `apps_gnome` | iguais |

No **Fedora KDE Plasma**, os módulos do desktop seguem a variante do KDE
(veja [No KDE Plasma](#no-kde-plasma)) e o `temas_yaru` fica indisponível:
o Plasma mantém o tema dele.

## No Linux Mint e no Zorin OS

Os dois usam o `apt` e os repositórios da base Ubuntu, então os módulos
funcionam como no Ubuntu da base (ex.: o Mint 22 e o Zorin OS 18 como o
Ubuntu 24.04): o codinome, o repositório da Microsoft, o formato do
repositório da Mozilla e a origem do mise seguem a base. A TUI mostra, por
exemplo, `Linux Mint 22.2 (base Ubuntu 24.04, noble)`.

Nos dois:

| ID | No Mint e no Zorin |
|----|--------------------|
| `mirrors` | só os repositórios da base Ubuntu mudam (no Mint, em `/etc/apt/sources.list.d/official-package-repositories.list`); o do Mint ou do Zorin fica como está |
| `firefox_thunderbird` | também pelo repositório da Mozilla (prioridade 1000), no lugar dos `.deb` da distribuição; sem snaps, a remoção deles não faz nada |
| `mise` | com base anterior à 26.04, pelo repositório APT oficial (`mise.jdx.dev/deb`, chave conferida), já que o `extrepo` não reconhece a distribuição |
| `temas_yaru` | **indisponível**: o tema e os ícones da distribuição (Mint-Y, temas do Zorin) ficam como estão |

No **Zorin OS** (GNOME), o resto é igual ao Ubuntu; o `extensoes_gnome`
ativa a barra de tarefas e o AppIndicator que vêm com o Zorin no lugar da
dock e do AppIndicator do Ubuntu, sem baixar o Dash to Dock por cima da
barra (veja [Extensões do GNOME](#extensões-do-gnome)).

No **Linux Mint** (Cinnamon), os módulos do desktop têm uma variante do
Cinnamon, com os mesmos IDs:

| ID | No Mint |
|----|---------|
| `flatpaks` | só o Flathub e a lista de apps (o Mint já traz o Flatpak, e a loja dele mostra os Flatpaks); sem o GNOME Software com o plugin snap, que puxaria o snapd que o Mint bloqueia |
| `papel_parede` | `org.cinnamon.desktop.background` |
| `atalhos_gnome` | **Atalhos do Cinnamon**: `GNOME/atalhos-cinnamon.dconf` carregado em `/org/cinnamon/` (Print e Shift+Super+S abrem o Gradia; Shift+Print e Alt+Print continuam com o Cinnamon) |
| `nautilus` | **Nemo**: ações do Nemo (`~/.local/share/nemo/actions`) "Abrir com o VSCode" (na seleção ou no fundo da pasta) e "Copiar caminho", e a pasta Projetos |
| `apps_gnome` | só os **favoritos do menu** (`org.cinnamon favorite-apps`), com as alternativas do Mint em `favoritos.txt`; o menu do Cinnamon não tem pastas |
| `extensoes_gnome` | **indisponível** (as extensões do GNOME Shell não rodam no Cinnamon) |

## No CachyOS

O CachyOS é da família Arch: os módulos usam o `pacman` (`-S --needed
--noconfirm`) e os repositórios oficiais (do Arch e do CachyOS). O que só
existe no **AUR** é instalado com o `paru`, que o CachyOS traz, **como o
usuário alvo** (o `makepkg` não roda como root): Google Chrome
(`google-chrome`), o VSCode da Microsoft (`visual-studio-code-bin`) e, no
`php_sqlserver`, o `msodbcsql`, o `mssql-tools` e o `php-sqlsrv`. O `paru`
chama o `sudo pacman` no fim, então, só durante cada instalação do AUR, um
arquivo em `/etc/sudoers.d/90-post-install-aur` libera **só o
`/usr/bin/pacman`** sem senha para esse usuário: a regra é validada com
`visudo -cf` antes de entrar no lugar e removida no fim, mesmo se a
instalação falhar. O `--skipreview` pula a revisão interativa dos
PKGBUILDs — os pacotes do AUR não têm a mesma revisão dos repositórios
oficiais.

O padrão do CachyOS é o fish; o módulo `shell` troca para o Zsh como nas
outras distribuições, e os scripts como o usuário rodam sempre no bash (veja
`c.AsUser` em [Adicionando ou editando um módulo](#adicionando-ou-editando-um-módulo)).
O tema do CachyOS não muda.

| ID | No CachyOS |
|----|------------|
| `mirrors` | `cachyos-rate-mirrors`, que mede e reordena os mirrors do Arch e do CachyOS |
| `atualizacao` / `limpeza` | `pacman -Syu` / remove os órfãos (`pacman -Qdtq`, se houver) e `pacman -Sc` |
| `ferramentas` | `github-cli`, `tea`, `nmap`, `python-pipx`, `ffmpeg`, `btop` e `fuse2`, todos dos repositórios oficiais |
| `firefox_thunderbird` / `bitwarden` / `solaar` / `podman` / `go` / `mise` / `tailscale` | pacotes dos repositórios oficiais (o Trayscale continua pelo Flatpak) |
| `chrome` / `vscode` | AUR (`google-chrome`, `visual-studio-code-bin`) |
| `remover_libreoffice` | `pacman -Rns`, só se houver algum pacote `libreoffice*` |
| `flatpaks` | só o Flatpak, o Flathub e a lista de apps (o Discover e o GNOME Software já mostram os Flatpaks) |
| `git_gcm` | o `.tar.gz` da release, como no Fedora |
| `php_sqlserver` | PHP e Composer dos repositórios; `msodbcsql`, `mssql-tools` e `php-sqlsrv` do AUR; as extensões entram em `/etc/php/conf.d` só se o PHP ainda não as carregar |
| `rust` / `ruby` | dependências de compilação do Arch (`base-devel`...) |
| `firewall` | `ufw` + `gufw` (não ativa nem cria regras) |
| `ovpn` | `networkmanager-openvpn` |
| `virt_manager` | `virt-manager`, `qemu-desktop`, `libvirt` e `dnsmasq`, com o `libvirtd` |
| `temas_yaru` | no GNOME, **só os botões** de minimizar e maximizar, se ainda não estiverem; no KDE, **indisponível** |

No **GNOME**, os outros módulos do desktop (papel de parede, extensões,
atalhos, Nautilus, favoritos e pastas) funcionam como no Fedora. No **KDE
Plasma**, eles têm a variante do KDE (veja [No KDE Plasma](#no-kde-plasma)).

## No KDE Plasma

No CachyOS com KDE e no Fedora KDE, os módulos do desktop têm uma variante
do KDE, com os mesmos IDs (e o tema da distribuição não muda):

| ID | No KDE |
|----|--------|
| `papel_parede` | `plasma-apply-wallpaperimage` (precisa da sessão do Plasma aberta) |
| `atalhos_gnome` | **Atalhos do KDE**: o Print continua com o Spectacle; Meta+Shift+S captura uma região (junto do padrão Meta+Shift+Print) e Meta+Shift+R grava uma região, no `kglobalshortcutsrc`, e o volume muda de 2 em 2% (`VolumeStep` no `plasmaparc`, como o `volume-step` do GNOME); valem a partir do próximo login |
| `nautilus` | **Dolphin**: service menus (`~/.local/share/kio/servicemenus`) "Abrir com o VSCode" e "Copiar caminho", e a pasta Projetos (diretório XDG) |
| `apps_gnome` | **Painel do Plasma**: apaga os painéis atuais e recria o painel pelo scripting do `plasmashell` (precisa da sessão do Plasma aberta; muda na hora): barra inferior flutuante de 46 px com controle de mídia, menu e apps fixados centralizados, bandeja do sistema (rede, volume, bateria e notificações visíveis; o resto no menu expandido), relógio e mostrar a área de trabalho. Os apps fixados são os de `GNOME/favoritos.txt` instalados (Configurações do sistema, Dolphin, Discover e Konsole nas alternativas do KDE) |
| `extensoes_gnome` | **indisponível**: as extensões do GNOME não rodam no Plasma |

## Módulos

Os módulos ficam em grupos, mostrados nessa ordem na TUI e no `--list`, e
rodam de cima para baixo. Cada um é autocontido: configura o próprio
repositório, se precisar, e instala o que usa, então pode ser marcado ou
desmarcado sozinho.

| Grupo | ID | O que faz |
|-------|----|-----------|
| Sistema | `mirrors` | aponta o APT para o mirror do Ubuntu mais rápido medido na rede |
| | `atualizacao` | `apt update` + `apt upgrade` |
| Terminal | `ferramentas` | git, curl, jq, bat, gh (repositório oficial), tea, nmap, pipx, ffmpeg, bashtop, FUSE 2 |
| | `shell` | Zsh como shell padrão e os dotfiles (Oh My Zsh, plugins, tema, fontes) |
| Aplicativos | `firefox_thunderbird` | `.deb` da Mozilla no lugar dos snaps |
| | `chrome` | Google Chrome |
| | `bitwarden` | Bitwarden desktop nativo |
| | `remover_libreoffice` | remove o LibreOffice nativo (vem como Flatpak) |
| | `flatpaks` | Flathub, GNOME Software com Flatpak e a lista de apps |
| | `flatpaks_jogos` | Steam, Heroic, ProtonPlus, PrismLauncher |
| | `solaar` | Solaar (PPA), regras UDEV e início com a sessão |
| Desenvolvimento | `git_gcm` | Git Credential Manager, com o chaveiro do GNOME |
| | `podman` | Podman e podman-compose, rootless (faixas de subuid/subgid para o usuário) |
| | `vscode` | VSCode (repositório da Microsoft) |
| | `go` | Go (`golang`) |
| | `mise` | mise (extrepo ou PPA oficial), ativado no `.zshrc`/`.bashrc` |
| | `node` | Node LTS + pnpm pelo mise |
| | `php_sqlserver` | PHP, Composer, msodbcsql18/mssql-tools18 e sqlsrv/pdo_sqlsrv |
| | `rust` | Rust stable pelo mise; eza, topgrade, cargo-update |
| | `ruby` | Ruby pelo mise, Bundler e Jekyll |
| | `claude_code` | Claude Code |
| | `deploy_dev` | deploy-dev |
| Rede e virtualização | `tailscale` | Tailscale (repositório oficial) + Trayscale |
| | `firewall` | ufw + gufw (não ativa o firewall) |
| | `ovpn` | importa os `.ovpn` de `OVPN/` no NetworkManager |
| | `virt_manager` | QEMU/KVM, libvirt e virt-manager |
| GNOME e aparência | `temas_yaru` | temas e ícones Yaru, sincronizados com os apps Flatpak |
| | `papel_parede` | aplica o papel de parede escolhido |
| | `extensoes_gnome` | extensões de `GNOME/extensoes.txt` e as preferências delas |
| | `atalhos_gnome` | atalhos de `GNOME/atalhos.dconf` |
| | `nautilus` | "Abrir com o VSCode", "Copiar caminho" e a pasta Projetos |
| | `apps_gnome` | favoritos e pastas do menu de apps |
| Finalização | `limpeza` | `apt autoremove` + `apt clean` |

O `shell` roda antes do `mise` porque o instalador do Oh My Zsh troca o
`~/.zshrc`, e o `mise` acrescenta a ativação nele. Desmarcado o `shell`, ele
escreve só nos arquivos que existirem (`.zshrc`/`.bashrc`). `node`, `rust` e
`ruby` dependem do `mise` e o marcam junto.

## mise (Node, pnpm, Rust, Ruby)

O módulo `mise` instala o [mise](https://mise.jdx.dev) pelo repositório
oficial, como indica a documentação dele: pelo `extrepo` no Ubuntu 22.04 e
24.04 e pela PPA `jdxcode/mise` do 26.04 em diante. A linha
`eval "$(mise activate zsh)"` (ou `bash`) vai para o `~/.zshrc` e o
`~/.bashrc`.

As ferramentas ficam como globais do usuário em `~/.config/mise/config.toml`
(`mise use -g`): `node@lts` e `pnpm@latest` (módulo `node`), `rust@stable`
(módulo `rust`) e `ruby@latest` (módulo `ruby`). Para atualizar, use
`mise upgrade`. Para fixar outra versão num projeto, use `mise use node@22`
dentro dele, o que grava um `mise.toml` no projeto.

Ao rodar de novo numa máquina configurada por uma versão anterior, os módulos
`node` e `ruby` tiram dos rc as linhas do fnm e do `GEM_HOME` que o
post-install tinha escrito. A pasta `~/.local/share/fnm` e as gems antigas em
`~/.local/share/gem` ficam como estão e podem ser apagadas à mão.

## Como usar

### Pacote da release

Cada release publica um pacote com o binário e a estrutura de pastas pronta:

```
post-install-linux-amd64.tar.gz
└── post-install/
    ├── post-install      # binário
    ├── OVPN/README.md    # coloque aqui os .ovpn a importar
    ├── Wallpapers/       # papéis de parede oferecidos na TUI
    ├── GNOME/            # extensões, atalhos, favoritos e pastas do GNOME
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

1. Mostra o sistema detectado (distribuição e versão). Se houver uma versão mais
   nova do post-install, a primeira tela oferece atualizar agora ou ignorar (veja [Atualização](#atualização)).
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
   - os módulos aparecem agrupados (veja [Módulos](#módulos)); o painel
     abaixo mostra a descrição do módulo e as dependências
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
| `--ovpn-dir` / `--wallpapers-dir` / `--gnome-dir` / `--log-dir` | pastas de `.ovpn`, papéis de parede, configurações do GNOME e logs (padrão: `OVPN/`, `Wallpapers/`, `GNOME/` e `logs/` ao lado do binário) |
| `--no-update-check` | não verifica se há versão nova do post-install |
| `--allow-untested-version` | segue, sem perguntar, numa versão da distribuição mais nova que as testadas (por sua conta e risco) |
| `--version` | mostra a versão |

Sem terminal interativo (pipe, CI), o programa usa o modo texto
automaticamente e roda todos os módulos (menos os de `--skip`); nesse caso
`--user` é obrigatório.

## Atualização

Ao iniciar, o post-install consulta a última release no GitHub (com prazo de
5 s; sem rede, só avisa e segue). Se houver uma versão mais nova:

- **TUI** — a primeira tela oferece *Atualizar agora* ou *Ignorar e continuar*;
- **modo texto** — pergunta `Atualizar agora? [S/n]`;
- **`--dry-run`, `--yes` ou sem terminal** — só avisa, sem atualizar.

Atualizar baixa o `.tar.gz` da release, confere o SHA-256 (pelo `.sha256`
publicado e pelo digest que o GitHub calcula) e substitui o binário. Quando
ele roda de dentro do pacote da release (com `GNOME/` ao lado), atualiza
também `GNOME/`, `Wallpapers/`, `README.md` e `OVPN/README.md` — os seus
`.ovpn` e os `logs/` nunca são tocados. Os arquivos ficam com o dono do
binário atual. A execução em andamento continua sendo a versão antiga: ao
final, o programa mostra o comando para executar de novo (com as mesmas
opções) e sai.

Builds locais (`go build`, versão `dev`) não verificam atualização.

## Log em arquivo

Cada execução grava dois arquivos em `logs/` (permissão `0600`, com dono
transferido para o usuário alvo — o programa roda como root):

- `setup_<timestamp>.log` — só as mensagens (info/aviso/erro/sucesso/passo),
  com hora e nível. Bom para uma leitura rápida.
- `setup_<timestamp>.raw.log` — transcrição completa, incluindo cada comando
  executado e a saída crua de `apt`/`flatpak`/`curl`. Bom para
  investigar uma falha.

## Estrutura do projeto

```
cmd/post-install/main.go    # flags, verificação de root, TUI x modo texto
internal/
  sysinfo/                  # detecção da distribuição e da versão, e checagem de suporte (versoes.go)
  runner/                   # execução de comandos, downloads, escrita de arquivos, dry-run
  runlog/                   # logs .log/.raw.log e repasse das mensagens à interface
  engine/                   # dependências entre módulos e execução em ordem
  modules/                  # os módulos (um arquivo por grupo)
    module.go               #   tipo Module, grupos, ordem de execução (All) e helpers
    pins.go                 #   commits fixados dos repositórios pessoais executados
    sistema.go              #   Sistema e Finalização: atualização, limpeza
    mirrors.go              #   mirrors (teste de velocidade dos mirrors do Ubuntu)
    terminal.go             #   Terminal: ferramentas (gh, tea...), Zsh e dotfiles
    apps.go                 #   Aplicativos: Chrome, Bitwarden, LibreOffice, Flatpaks, Solaar
    mozilla.go              #   Firefox e Thunderbird (repositório da Mozilla, remoção dos snaps)
    dev.go                  #   Desenvolvimento: GCM, Podman, VSCode, Go, mise, Node, PHP/SQL Server, Rust, Ruby, Claude Code, deploy-dev
    rede.go                 #   Rede e virtualização: Tailscale, firewall, OpenVPN, virt-manager
    aparencia.go            #   temas Yaru (+ sincronização com o Flatpak) e Nautilus
    wallpaper.go            #   papel de parede (cópia para ~/.local/share/backgrounds + gsettings)
    gnome_extensoes.go      #   extensões do GNOME (extensions.gnome.org ou git + gsettings + dconf)
    gnome_atalhos.go        #   atalhos do GNOME (dconf)
    gnome_apps.go           #   favoritos e pastas do menu de apps (por categoria)
    golden_test.go          #   testes golden do dry-run (cenários em testdata/golden/)
  config/flatpaks.go        # listas de Flatpaks (apps e jogos)
  update/                   # verificação e aplicação de versão nova (release do GitHub)
  tui/                      # interface Bubble Tea
  plain/                    # saída em texto simples
.github/workflows/release.yml
.cz.toml                    # configuração do commitizen
```

## Adicionando ou editando um módulo

1. Declare o módulo no arquivo do grupo dele, em `internal/modules/`:
   ```go
   var meuModulo = Module{
       ID:    "meu_modulo",
       Grupo: GrupoDesenvolvimento,
       Title: "Título no menu",
       Desc:  "Descrição de uma linha",
       Deps:  []string{"mise"}, // opcional
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
2. Inclua-o em `All()` (`internal/modules/module.go`) entre os módulos do
   mesmo grupo — um teste garante que cada grupo fica junto e na ordem.
3. O `Run` é o caminho do Ubuntu. Para o módulo existir também no Fedora,
   preencha `Fedora` com uma `Variante` (sem ela, o módulo fica indisponível
   lá: some da TUI e do `--list`, e `--modules` só avisa). `Title`/`Desc`
   vazios e `Deps` nil herdam os do módulo:
   ```go
   Fedora: &Variante{
       Desc: "Descrição no Fedora", // opcional
       Run: func(c *Ctx) error {
           return c.install("pacote-no-fedora") // dnf no Fedora
       },
   },
   ```
   `Disponiveis(sys)` devolve a lista já adaptada à distribuição detectada;
   no Ubuntu, é a própria `All()`.
4. O Zorin OS usa o `Run` do Ubuntu. No Linux Mint também, a menos que o
   campo `Mint` tenha uma `Variante` — necessária nos módulos que mexem no
   desktop (o Mint usa o Cinnamon). `Mint: &Variante{Indisponivel: true}`
   tira o módulo do Mint. O campo `Zorin` funciona do mesmo jeito no Zorin
   OS. Um teste garante que todo módulo do grupo "GNOME e aparência" tem a
   variante do Mint.
5. No CachyOS, o módulo precisa do campo `Arch` (como o `Fedora`: sem ele,
   fica indisponível; um teste garante que todos têm). No KDE Plasma
   (CachyOS e Fedora KDE), o campo `KDE` fica por cima da variante da
   família — necessário nos módulos do grupo "GNOME e aparência" (um teste
   confere). Como vale nas duas famílias, ele só usa pacotes de mesmo nome
   nas duas; o que muda por família fica na variante dela (com `c.kde()`).

Dentro de `Run`, use os helpers do `Ctx` em vez de `os/exec` direto, para a
saída ir para a TUI/logs e o `--dry-run` funcionar:

- `c.Run(...)`/`c.RunEnv(...)` — comando que altera o sistema (falha = erro)
- `c.Try(...)` — comando que altera o sistema, silencioso, devolve `bool`
- `c.Probe(...)`/`c.Output(...)` — consultas somente-leitura (rodam mesmo no dry-run)
- `c.AsUser(script)` — shell como o usuário alvo (`su -s /bin/bash - usuario -c`: sempre no bash, mesmo que o shell de login dele seja outro, como o fish)
- `c.install(...)`, `c.garantir(comando, pacote)`, `c.flatpakInstall(...)`
- `c.install(...)` usa o `apt` no Ubuntu, o `dnf` no Fedora e o `pacman` no
  CachyOS (os nomes de pacote são os da distribuição);
  `c.pacoteInstalado(pkg)` consulta o `dpkg`, o `rpm` ou o `pacman`
- `c.instalarAUR(...)` — pacotes do AUR pelo `paru`, como o usuário alvo,
  com o `pacman` liberado no sudoers só durante a instalação (CachyOS)
- `instalarChaveAPT(c, url, destino, fingerprints...)` — chave de repositório conferida
- `instalarChaveRPM(c, url, nome, fingerprints...)` e `repoRPM(c, nome, título, baseurl, chave)` —
  chave conferida em `/etc/pki/rpm-gpg` (com `rpm --import`) e `.repo` em
  `/etc/yum.repos.d` com `gpgcheck=1`
- `c.Download(url, destino)`, `c.Temp(nome)`, `c.WriteFile(...)`, `c.AppendFile(...)`
- `c.Log.Infof/Successf/Warnf(...)` — avisos marcam o módulo como "concluído com avisos"
- `c.Exists(...)`, `c.LookPath(...)`, `c.ReadFile(...)`, `c.Stat(...)`, `c.FileExists(...)`,
  `c.Glob(...)` — consultas à máquina (no lugar de `os.*`/`exec.LookPath`)

Toda consulta à máquina ou à rede passa pelo `Ctx` porque os **testes
golden** trocam a máquina por uma falsa. `golden_test.go` descreve cenários
(Ubuntu 22.04, 24.04 e 26.04 recém-instalados, um 26.04 já configurado,
o Fedora 44 Workstation e KDE, o Linux Mint 22, o Zorin OS 18 e o CachyOS com KDE e com GNOME),
roda todos os módulos em dry-run contra cada um e compara o resultado com
`internal/modules/testdata/golden/<cenário>.txt`. Uma mudança em qualquer
módulo, inclusive na lista de Flatpaks, aparece como diferença nesses
arquivos. Se a mudança for intencional, regrave e revise o diff antes de
commitar:

```bash
go test ./internal/modules -run TestGolden -update
git diff internal/modules/testdata/golden/
```

Os commits e versões fixados pelo `pins.yml` aparecem normalizados (ex.:
`<dotfilesCommit>`), então os PRs de atualização dos pins não mexem nos
golden. Os arquivos de `GNOME/` usados nos testes são fixtures próprias, em
`internal/modules/testdata/gnome/`.

## Detecção de sistema (`internal/sysinfo`)

`sysinfo.Detect` lê `/etc/os-release`, recusa o que não for suportado (veja
[Sistemas suportados](#sistemas-suportados)) e preenche:

- `Familia` — `ubuntu`, `fedora` (com `VARIANT_ID=workstation` ou `kde`) ou
  `arch` (o CachyOS); o Linux Mint e o Zorin OS são da família `ubuntu`
- `Desktop` — `Cinnamon` no Mint (confere o binário `cinnamon`), `KDE` no
  Fedora KDE (`plasmashell`), `KDE` ou `GNOME` no CachyOS (`plasmashell` ou
  `gnome-shell`), `GNOME` nos outros (confere o `gnome-shell`)
- `Codename` — `jammy`, `noble`, `resolute`... (de `UBUNTU_CODENAME`; no
  Mint e no Zorin, o da base), usado na URL do repositório do Tailscale e no
  teste de mirrors; vazio no Fedora
- `UbuntuVersion` — a versão do Ubuntu (no Mint e no Zorin, a da base,
  tirada de `versoes.go` pelo codinome); vazio no Fedora
- `MSRepoVersion` — a versão nas URLs do repositório da Microsoft
  (msodbcsql/mssql-tools), tirada de `versoes.go`: no Ubuntu, a da própria LTS
  (ou a da última da lista, nas mais novas); no Fedora, a do RHEL
- `NaoTestada` — versão mais nova que todas as de `versoes.go` (pede
  confirmação)
- `Arch` — `dpkg --print-architecture`

### Versões suportadas

`internal/sysinfo/versoes.go` lista, da mais antiga para a mais nova:

- **Ubuntu:** as LTS com suporte padrão (sem contar o ESM pago), pelas datas
  do [`distro-info-data`](https://salsa.debian.org/debian/distro-info-data)
  (a mesma fonte do `ubuntu-distro-info --supported`);
- **Fedora:** as versões em manutenção, pela API do
  [Bodhi](https://bodhi.fedoraproject.org/releases/) (fim de vida ainda não
  alcançado);
- para cada uma, a versão do repositório da Microsoft a usar, conferindo se
  ele já existe em `packages.microsoft.com`.

O workflow `versoes.yml` roda o `scripts/atualizar-versoes.sh` toda semana
(ou à mão) e abre um PR na `development` quando a lista muda — uma versão
nova entra, uma fora de suporte sai. A mudança só chega às máquinas na
próxima release do post-install.

O GNOME é conferido pelo binário `gnome-shell`, e não por
`XDG_CURRENT_DESKTOP`: o sudo costuma limpar essa variável.

## Papel de parede

Os papéis de parede ficam em `Wallpapers/` (`.jpg`, `.jpeg`, `.png` ou
`.webp`) — para oferecer outra imagem, basta colocá-la na pasta, sem
recompilar. O módulo `papel_parede`, como o usuário alvo:

1. copia a imagem escolhida para `~/.local/share/backgrounds/` (o mesmo lugar
   onde o GNOME guarda as imagens escolhidas em Configurações → Plano de
   fundo);
2. aplica com `gsettings` (`picture-uri`, `picture-uri-dark` e
   `picture-options 'zoom'`).

Os scripts que rodam como o usuário (`su -s /bin/bash - usuario`) usam a sessão D-Bus dele
quando há uma sessão gráfica aberta; sem isso o `gsettings` descarta as
alterações sem erro.

## Extensões do GNOME

A pasta `GNOME/` define as extensões (editável sem recompilar):

- `extensoes.txt` — uma extensão (UUID) por linha; `#` comenta, o prefixo
  `-` mantém a extensão desativada (ex.: `-ding@rastersoft.com`) e uma URL
  git depois do UUID instala a extensão do repositório em vez do
  extensions.gnome.org (ex.: a Now Pill). Depois da URL, opcionalmente, vem o
  commit (ou tag) a instalar — recomendado, para uma mudança na branch
  principal não chegar sozinha às instalações (sem ele, o módulo avisa).
  A mesma lista serve às duas distribuições, com duas marcações:
  - `a | b` — equivalentes: a primeira que vier com o sistema é ativada; se
    nenhuma vier, a **última** da linha é baixada do extensions.gnome.org
    (ex.: `ubuntu-dock@ubuntu.com | zorin-taskbar@zorinos.com | dash-to-dock@micxgx.gmail.com`
    — no Ubuntu, a dock dele; no Zorin, a barra de tarefas dele; no Fedora,
    o Dash to Dock, que usa as mesmas preferências de `extensoes.dconf`);
  - `uuid sistema` — só é ativada se vier com o sistema, nunca é baixada
    (ex.: as do snapd, que só existem no Ubuntu);
- `extensoes.dconf` — as preferências delas, no formato do `dconf dump`;
- `atalhos.dconf` — os atalhos de teclado, aplicados pelo módulo
  `atalhos_gnome` (veja [Atalhos do GNOME](#atalhos-do-gnome));
- `atalhos-cinnamon.dconf` — os atalhos no Linux Mint (Cinnamon), no formato
  do Cinnamon (o `binding` é uma lista).

O módulo `extensoes_gnome`, como o usuário alvo:

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

O módulo `atalhos_gnome` aplica `GNOME/atalhos.dconf` com
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

## Favoritos e pastas do menu de apps

O módulo `apps_gnome` roda perto do fim, depois de tudo instalado, e organiza
o que existir de fato na máquina — por isso funciona mesmo com módulos
desmarcados ou apps instalados à mão:

- **Favoritos** — fixa na barra de favoritos os apps de
  `GNOME/favoritos.txt`, na ordem do arquivo. Cada linha aceita alternativas
  separadas por `|` (ex.: `org.gnome.Ptyxis.desktop | org.gnome.Terminal.desktop`),
  e entra a primeira instalada; o que não existir é pulado.
- **Pastas** — apaga as pastas atuais do menu de apps (inclusive as do Ubuntu,
  como System/Utilities) e recria as de `GNOME/pastas.txt`: Games,
  Development, Office, Communication, Internet, Multimedia, System e
  Utilities. Os nomes seguem o idioma do sistema: cada pasta aponta um
  `.directory` padrão (`Game.directory`, `Office.directory`...), cuja tradução
  o GNOME usa; sem o arquivo na máquina (o `gnome-menus` pode faltar), fica o
  nome fixo, em inglês. Cada pasta reúne os apps pelas
  [categorias do freedesktop.org](https://specifications.freedesktop.org/menu-spec/latest/category-registry.html)
  que o `.desktop` declara (`Game`, `Development`, `Office`...), então
  apps instalados depois também caem na pasta certa. `apps =` força apps que
  declaram categorias ruins (ex.: o Vesktop só declara `Network`).

A ordem das pastas no arquivo é a prioridade: um app que se encaixe em mais
de uma (a Steam declara `Network` e `Game`) fica só na primeira e é excluído
das outras (`excluded-apps`). Pastas sem nenhum app instalado não aparecem —
passam a aparecer quando algo daquela categoria for instalado. Apps sem
nenhuma categoria conhecida ficam soltos no menu (o log lista quais). A
disposição do menu é zerada, para o GNOME reordenar tudo.

Um app instalado depois que se encaixe em duas pastas aparece nas duas, já
que as exclusões são calculadas na hora da execução; basta rodar de novo
`sudo ./post-install --modules apps_gnome` para reorganizar.

## Temas Yaru nos apps Flatpak

O Flatpak não expõe o `/usr` do host ao sandbox, então os apps Flatpak não
enxergam os temas de `/usr/share/themes`: só os publicados no Flathub
(`org.gtk.Gtk3theme.*`), onde faltam as variantes de cor do Yaru
(`Yaru-blue-dark`, `Yaru-sage`...). Com uma delas escolhida, os apps Flatpak
caíam no Adwaita. O módulo `temas_yaru`:

1. instala `yaru-theme-gtk` e `yaru-theme-icon`;
2. instala o script `/usr/local/sbin/yaru-flatpak-sync`, que copia todos os
   `/usr/share/themes/Yaru*` para `/var/lib/yaru-flatpak/themes`, resolvendo
   os links (que apontam para fora da pasta e não existiriam no sandbox), e
   troca a cópia de uma vez;
3. instala o hook `/etc/apt/apt.conf.d/99yaru-flatpak`, que roda o script
   depois de cada execução do `dpkg`. O script só copia de novo quando a
   versão do `yaru-theme-gtk` muda, então nas outras vezes sai na hora: uma
   atualização do Yaru pelo `apt` chega sozinha aos apps Flatpak;
4. libera a cópia e o `~/.local/share/themes` para todos os apps
   (`flatpak override --system --filesystem=xdg-data/themes:ro
   --filesystem=/var/lib/yaru-flatpak/themes:ro`);
5. cria, para o usuário alvo, um link em `~/.local/share/themes` para cada
   tema da cópia (sem mexer em uma pasta de mesmo nome que já esteja lá). É
   por ele que o GTK de dentro do sandbox encontra o tema.
6. aplica os ícones `Yaru-dark` para o usuário alvo.

Vale para apps GTK 3. Apps em GTK 4 com libadwaita não usam temas GTK, só o
modo claro/escuro. Uma variante de cor nova do Yaru precisa de mais um link:
basta rodar de novo `sudo ./post-install --modules temas_yaru`. Para forçar
a cópia à mão: `sudo yaru-flatpak-sync --force`.

## Ruby, Bundler e Jekyll

O módulo `ruby` instala o Ruby pelo mise (`ruby@latest`), que o compila na
máquina com o ruby-build. Por isso instala antes as dependências de
compilação (`build-essential`, `libssl-dev`, `libyaml-dev`, `zlib1g-dev`,
`libffi-dev` e outras), que também servem às gems nativas. O Ruby fica na home
do usuário, e nada de Ruby exige `sudo` depois:

- o Bundler e o Jekyll são instalados como gems desse Ruby e listados no
  `~/.default-gems`, que o mise reinstala a cada versão nova do Ruby;
- `bundle config set --global path vendor/bundle` (em `~/.bundle/config`):
  o `bundle install` de cada projeto grava as gems dentro do próprio projeto,
  em `vendor/bundle` — que o Jekyll já ignora no build e o `.gitignore` do
  `jekyll new` já exclui.

Para um projeto específico usar as gems do Ruby do mise em vez de
`vendor/bundle`: `bundle config set --local path.system true`.

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
- **Commits dos repositórios pessoais, Now Pill e regra UDEV do Solaar** — o workflow `.github/workflows/pins.yml`, todo dia (ou à mão
  em Actions → Pins → Run workflow). Ele roda `scripts/atualizar-pins.sh`,
  que confere o último commit/tag de cada um, atualiza o pin e abre o PR `fix: atualiza versões fixadas` com links de
  comparação para revisar o que mudou. Se a regra do Solaar mudar, o PR
  destaca isso (ela roda como root). Precisa de *Settings → Actions → General
  → Allow GitHub Actions to create and approve pull requests*.

O script também roda localmente: `bash scripts/atualizar-pins.sh`.

## Segurança

- **Grupos com poder de root.** O usuário alvo entra nos grupos
  `libvirt`/`kvm` (módulo `virt_manager`). Quem está no `libvirt` consegue
  virar root (ex.: montando `/` numa VM) — é o comportamento normal numa
  workstation de desenvolvimento, mas vale saber. O Podman roda rootless e
  não precisa de grupo: os contêineres têm só os privilégios do usuário.
- **Chaves de repositório conferidas.** As chaves de Microsoft,
  GitHub CLI, Mozilla, Google e Tailscale são baixadas e comparadas com o
  fingerprint esperado antes de entrarem no sistema.
- **Versões fixadas.** Os repositórios pessoais executados (Dotfiles,
  vscode_nautilus, nautilus-copypath) ficam fixados num commit (constantes
  em `internal/modules/pins.go`); a regra UDEV do Solaar vem embutida no
  binário (`internal/modules/42-logitech-unify-permissions.rules`). Assets de release do GitHub
  (GCM, deploy-dev) são conferidos pelo SHA-256 que o GitHub
  publica.
- **Scripts de instalação remotos.** O Claude Code ainda é instalado pelo
  `curl | sh` oficial, como o usuário alvo (não como root). O mise vem do
  repositório assinado (extrepo ou PPA), e Node, pnpm, Rust e Ruby vêm dele,
  que confere os downloads de cada ferramenta. O Tailscale vem do repositório
  oficial assinado.
- **O que não é verificado.** O `.deb` do Chrome e o do Bitwarden (e, no
  Fedora, o `.rpm` do Bitwarden) são baixados por HTTPS dos sites oficiais,
  sem conferência de assinatura pelo post-install (o Chrome passa a se
  atualizar pelo repositório assinado do Google depois de instalado; no
  Fedora ele já é instalado pelo repositório, com a assinatura conferida).

## Requisitos

- Ubuntu 22.04, 24.04 ou 26.04 com GNOME, Fedora Workstation ou KDE Plasma
  43, 44 ou 45 (ou mais novos, com confirmação), Linux Mint (Cinnamon) e Zorin OS
  (Core) com uma dessas bases Ubuntu, ou CachyOS com KDE Plasma ou GNOME — veja
  [Sistemas suportados](#sistemas-suportados).
- Acesso root (`sudo`) — exceto em `--dry-run`.
- Terminal interativo para a TUI (sem ele, o modo texto é usado).
