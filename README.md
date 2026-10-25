# POST — Pós-instalação com TUI

Binário em Go, com interface interativa no terminal (TUI), para configurar uma
workstation Ubuntu recém-instalada: mirrors, atualização do sistema, Docker,
VSCode, Go, PHP + SQL Server, ferramentas de terminal (gh, tea, nmap, pipx, ffmpeg,
bashtop, FUSE 2 para AppImages), Flatpaks (incluindo jogos — Steam, Heroic,
ProtonPlus, PrismLauncher), Tailscale + Trayscale, Chrome, Bitwarden,
temas e ícones Yaru (com todas as variantes do tema nos apps Flatpak),
extensões e atalhos do GNOME, o ambiente do usuário (zsh,
Oh My Zsh, fnm/Node, dotfiles pessoais), Rust (rustup, eza, topgrade, cargo-update),
Ruby (Bundler e Jekyll), Claude Code, deploy-dev, importação de perfis OpenVPN e as extensões "Abrir com o
VSCode"/"Copiar caminho" no Nautilus.

## Sistemas suportados

Só o **Ubuntu com GNOME** (`ID=ubuntu` no `/etc/os-release` e o
`gnome-shell` instalado), nas versões **22.04 (jammy), 24.04 (noble) e
26.04 (resolute)**, além das mais novas que a 26.04.

Qualquer outro sistema faz o programa parar com uma mensagem clara em vez de
continuar em um estado incerto — inclusive derivados do Ubuntu (Linux Mint,
Pop!_OS, Zorin...), sabores com outro desktop (Kubuntu, Xubuntu...) e versões
anteriores à 22.04 ou interinas entre as LTS suportadas. Versões mais novas
que a 26.04 usam o repositório da Microsoft do 26.04 (compatível), com um
aviso.

### Firefox e Thunderbird sem snap

No Ubuntu, os pacotes `firefox` e `thunderbird` do repositório oficial são só
pacotes de transição que instalam a versão snap. Por isso:

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

1. Mostra o sistema detectado (versão do Ubuntu). Se houver uma versão mais
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
| `--ovpn-dir` / `--wallpapers-dir` / `--gnome-dir` / `--log-dir` | pastas de `.ovpn`, papéis de parede, configurações do GNOME e logs (padrão: `OVPN/`, `Wallpapers/`, `GNOME/` e `logs/` ao lado do binário) |
| `--no-update-check` | não verifica se há versão nova do post-install |
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
  sysinfo/                  # detecção da versão do Ubuntu e checagem de suporte
  runner/                   # execução de comandos, downloads, escrita de arquivos, dry-run
  runlog/                   # logs .log/.raw.log e repasse das mensagens à interface
  engine/                   # dependências entre módulos e execução em ordem
  modules/                  # os módulos (um arquivo por grupo)
    module.go               #   tipo Module, ordem de execução (All) e helpers
    mirrors.go              #   mirrors (teste de velocidade dos mirrors do Ubuntu)
    repositorios.go         #   atualização, repositórios, pacotes base
    mozilla.go              #   repositório APT da Mozilla e remoção dos snaps
    apps.go                 #   Solaar, LibreOffice, Flatpaks, Tailscale, PHP, Chrome+GCM, Bitwarden
    desktop.go              #   temas Yaru (+ sincronização com o Flatpak), ambiente do usuário, Rust, Ruby, Claude Code, deploy-dev, Nautilus
    wallpaper.go            #   papel de parede (cópia para ~/.local/share/backgrounds + gsettings)
    gnome_extensoes.go      #   extensões do GNOME (extensions.gnome.org ou git + gsettings + dconf)
    gnome_atalhos.go        #   atalhos do GNOME (dconf)
    gnome_apps.go           #   favoritos e pastas do menu de apps (por categoria)
    sistema.go              #   OpenVPN, virt-manager, limpeza
  config/flatpaks.go        # listas de Flatpaks (apps e jogos)
  update/                   # verificação e aplicação de versão nova (release do GitHub)
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
- `c.install(...)`, `c.flatpakInstall(...)`
- `c.Download(url, destino)`, `c.Temp(nome)`, `c.WriteFile(...)`, `c.AppendFile(...)`
- `c.Log.Infof/Successf/Warnf(...)` — avisos marcam o módulo como "concluído com avisos"

## Detecção de sistema (`internal/sysinfo`)

`sysinfo.Detect` lê `/etc/os-release`, recusa o que não for suportado (veja
[Sistemas suportados](#sistemas-suportados)) e preenche:

- `Codename` — `jammy`, `noble`, `resolute`... (de `UBUNTU_CODENAME`), usado
  nas URLs dos repositórios (Docker, Tailscale) e no teste de mirrors
- `MSRepoVersion` — a versão nas URLs do repositório da Microsoft
  (msodbcsql/mssql-tools): a própria versão nas LTS suportadas, `26.04` nas
  mais novas
- `Arch` — `dpkg --print-architecture`

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
caíam no Adwaita. O módulo `temas_icones`:

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

Vale para apps GTK 3. Apps em GTK 4 com libadwaita não usam temas GTK, só o
modo claro/escuro. Uma variante de cor nova do Yaru precisa de mais um link:
basta rodar de novo `sudo ./post-install --modules temas_icones`. Para forçar
a cópia à mão: `sudo yaru-flatpak-sync --force`.

## Ruby, Bundler e Jekyll

O módulo `ruby` instala o Ruby com os headers de desenvolvimento e o que as
gems nativas precisam para compilar (`ruby-full`, `ruby-dev`,
`build-essential` e `zlib1g-dev`). O Bundler e o Jekyll são instalados **como gems do usuário**, como
recomenda a documentação do Jekyll, e nada de Ruby exige `sudo` depois:

- `GEM_HOME=~/.local/share/gem` (com `~/.local/share/gem/bin` no `PATH`) é
  acrescentado ao `~/.zshrc` e ao `~/.bashrc`: `gem install` grava na home;
- `bundle config set --global path vendor/bundle` (em `~/.bundle/config`):
  o `bundle install` de cada projeto grava as gems dentro do próprio projeto,
  em `vendor/bundle` — que o Jekyll já ignora no build e o `.gitignore` do
  `jekyll new` já exclui.

Para um projeto específico usar as gems do usuário (`GEM_HOME`) em vez de
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
- **Commits dos repositórios pessoais, Now Bar e regra UDEV do Solaar** — o workflow `.github/workflows/pins.yml`, todo dia (ou à mão
  em Actions → Pins → Run workflow). Ele roda `scripts/atualizar-pins.sh`,
  que confere o último commit/tag de cada um, atualiza o pin e abre o PR `fix: atualiza versões fixadas` com links de
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
  fingerprint esperado antes de entrarem no sistema.
- **Versões fixadas.** Os repositórios pessoais executados (Dotfiles,
  vscode_nautilus, nautilus-copypath) ficam fixados num commit (constantes
  no topo de `internal/modules/desktop.go`); a regra UDEV do Solaar vem embutida no
  binário (`internal/modules/42-logitech-unify-permissions.rules`). Assets de release do GitHub
  (GCM, deploy-dev) são conferidos pelo SHA-256 que o GitHub
  publica.
- **Scripts de instalação remotos.** fnm, rustup e Claude Code ainda são
  instalados pelo `curl | sh` oficial de cada um, como o usuário alvo (não
  como root). O Tailscale vem do repositório oficial assinado.
- **O que não é verificado.** O `.deb` do Chrome e o do Bitwarden
  são baixados por HTTPS dos sites oficiais, sem conferência de assinatura
  pelo post-install (o Chrome passa a se atualizar pelo repositório assinado
  do Google depois de instalado).

## Requisitos

- Ubuntu 22.04, 24.04 ou 26.04 (ou mais novo) com GNOME — veja
  [Sistemas suportados](#sistemas-suportados).
- Acesso root (`sudo`) — exceto em `--dry-run`.
- Terminal interativo para a TUI (sem ele, o modo texto é usado).
