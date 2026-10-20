## v1.7.0 (2026-10-20)

### Feat

- verifica se há versão nova e oferece atualizar ao iniciar
- avisa por PR quando há versão nova dos pins

## v1.6.0 (2026-10-20)

### Feat

- endurece a segurança da instalação

## v1.5.0 (2026-10-17)

### Feat

- adiciona ferramentas de terminal, deploy-dev, atalhos do GNOME e Now Bar

## v1.4.0 (2026-10-11)

### Feat

- instala, ativa e configura extensões do GNOME

## v1.3.0 (2026-10-11)

### Feat

- usa o repositório da Microsoft do Ubuntu 26.04 (resolute)

## v1.2.1 (2026-10-11)

### Fix

- mostra o tamanho de imagens pequenas em KB

## v1.2.0 (2026-10-11)

### Feat

- TUI oferece o usuário detectado ou outro usuário
- detecta o usuário que executou o sudo
- escolha e aplicação do papel de parede na TUI

### Fix

- scripts como usuário usam a sessão D-Bus dele

## v1.1.1 (2026-10-11)

### Fix

- autostart do Solaar inicia oculto (-w hide)
- verifica a chave da Mozilla sem depender do ~/.gnupg do root

## v1.1.0 (2026-10-11)

### Feat

- instala Firefox e Thunderbird via repositório da Mozilla no Ubuntu

## v1.0.0 (2026-10-11)

### BREAKING CHANGE

- lib/, modules/*.sh e config.sh foram removidos; o
setup.sh agora baixa e executa o binário da última release. Falhas em
um módulo não abortam mais a execução inteira (só os dependentes são
pulados), exceto com --fail-fast.

### Feat

- reescreve o post-install em Go com TUI
- install nautilus-copypath extension alongside vscode_nautilus
- add com.rtosta.zapzap to Flatpak lists for KDE and GTK environments
- add Arch Linux support with Chaotic-AUR and yay integration
- adiciona verificação para contas de domínio antes de trocar o shell padrão
- adiciona suporte para verificar seleção de módulos antes da instalação via Snap
- adiciona suporte a dependências entre módulos no registro
- otimiza downloads paralelos para instalação do Chrome, GCM e configurações do usuário
- atualiza módulo de importação OpenVPN para usar diretório OVPN e adiciona README
- adiciona scripts para instalação de aplicativos via Snap e extensão do VSCode no gerenciador de arquivos
- otimiza seleção de espelhos para APT no Ubuntu com verificação de velocidade
- otimiza seleção de espelhos para APT no Ubuntu com verificação de velocidade
- atualiza comando de instalação do virt-manager para usar qemu-system-x86 no Ubuntu
- adiciona funcionalidade de logging em arquivos para scripts de instalação e migração
- adiciona módulo para instalação do virt-manager (QEMU/KVM) e suas dependências
- adiciona script de limpeza final para remoção de pacotes órfãos e limpeza de cache
- remove Remmina da lista de aplicativos instalados via Snap e atualiza a documentação
- atualiza documentação e lógica de migração para Snap, exclui Heroic Games Launcher
- adiciona suporte à migração de apps para Snap no Ubuntu e implementa instalação via Snap
- adiciona script de limpeza final para remoção de pacotes órfãos e limpeza de cache
- adiciona suporte a repositório de fallback para Ubuntu 24.04 e inclui plugin Snap no gnome-software
- atualiza comandos de instalação para usar 'apt' em vez de 'apt-get' em scripts de configuração
- atualiza lógica de instalação do Solaar, ajusta comentários e simplifica configuração de regras UDEV
- aprimora suporte a ambientes de desktop, adiciona lógica para instalação de Flatpaks e configurações específicas para KDE e GNOME
- aprimora detecção de sistema operacional para incluir suporte a derivados do Ubuntu e atualiza instalação de pacotes base
- atualiza scripts de instalação e configuração, adiciona suporte para Solaar via Flatpak e ajusta autostart
- adiciona módulo para importação de perfis OpenVPN e módulo de limpeza final
- adiciona novos módulos para instalação de Tailscale, extensões PHP, Chrome, Bitwarden, temas e ícones, ambiente do usuário, ferramentas Rust, Claude Code e extensão do VSCode no Nautilus
- atualiza README e adiciona novos módulos para instalação de Flatpaks de jogos, Rust e Claude Code
- adiciona scripts para instalação e configuração de extensões PHP, Chrome, Bitwarden, temas e ícones, ambiente do usuário e limpeza final
- atualiza README e adiciona novos módulos para instalação de ferramentas Rust, Claude Code e extensão do VSCode no Nautilus
- add post-installation setup script for Linux workstations

### Fix

- use --locked on cargo install for reproducible builds
- remove com.mikrotik.WinBox from GTK Flatpak list
- normalize line endings in .gitignore and README.md

### Refactor

- remove Snap migration script and clean up related code
- transfere a responsabilidade de configurar os dotfiles para um script indepedente
