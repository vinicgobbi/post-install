package modules

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var lookPath = exec.LookPath

var ovpn = Module{
	ID:    "ovpn",
	Title: "Importar perfis OpenVPN",
	Desc:  "Instala o plugin OpenVPN do NetworkManager e importa os .ovpn de ./OVPN (aplicando DNS/domínio quando presentes no arquivo)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Importando perfis OpenVPN para o NetworkManager...")
		if err := instalarPluginOpenVPN(c); err != nil {
			return err
		}

		arquivos, _ := filepath.Glob(filepath.Join(c.OVPNDir, "*.ovpn"))
		if len(arquivos) == 0 {
			c.Log.Warnf("Nenhum arquivo .ovpn encontrado em %s — pulando importação.", c.OVPNDir)
			return nil
		}

		saida, _ := c.Output("nmcli", "-g", "NAME,TYPE", "connection", "show")
		existentes := conexoesNM(saida)
		for _, arquivo := range arquivos {
			nome := strings.TrimSuffix(filepath.Base(arquivo), ".ovpn")
			avisarPermissaoOVPN(c, arquivo)

			// Reimporta do zero para continuar idempotente em reexecuções —
			// mas só apaga se a conexão com esse nome for mesmo uma VPN, para
			// não levar junto uma conexão cabeada/Wi-Fi homônima.
			if tipo, ok := existentes[nome]; ok {
				if tipo != "vpn" {
					c.Log.Warnf("Já existe uma conexão '%s' do tipo %s (não VPN): pulando %s para não sobrescrevê-la.", nome, tipo, filepath.Base(arquivo))
					continue
				}
				c.Try("nmcli", "connection", "delete", "id", nome)
			}
			if !c.Try("nmcli", "connection", "import", "type", "openvpn", "file", arquivo) {
				c.Log.Warnf("Falha ao importar %s, pulando.", arquivo)
				continue
			}

			// DNS e domínio de busca nem sempre são preenchidos pela importação
			// automática do plugin; quando o .ovpn declara essas diretivas,
			// aplica manualmente.
			conteudo, err := os.ReadFile(arquivo)
			if err != nil {
				return err
			}
			dns, dominios := opcoesDNS(string(conteudo))
			if len(dns) > 0 {
				c.Try("nmcli", "connection", "modify", nome, "ipv4.dns", strings.Join(dns, " "))
			}
			if len(dominios) > 0 {
				c.Try("nmcli", "connection", "modify", nome, "ipv4.dns-search", strings.Join(dominios, " "))
			}
			c.Log.Successf("Perfil '%s' importado.", nome)
		}
		return nil
	},
}

// conexoesNM interpreta a saída de "nmcli -g NAME,TYPE connection show"
// (nome -> tipo). No modo -g, ":" e "\" dentro dos valores vêm escapados
// com "\".
func conexoesNM(saida string) map[string]string {
	out := map[string]string{}
	for _, linha := range strings.Split(saida, "\n") {
		if linha == "" {
			continue
		}
		var campos []string
		var atual strings.Builder
		for i := 0; i < len(linha); i++ {
			switch {
			case linha[i] == '\\' && i+1 < len(linha):
				i++
				atual.WriteByte(linha[i])
			case linha[i] == ':':
				campos = append(campos, atual.String())
				atual.Reset()
			default:
				atual.WriteByte(linha[i])
			}
		}
		campos = append(campos, atual.String())
		if len(campos) == 2 {
			out[campos[0]] = campos[1]
		}
	}
	return out
}

// avisarPermissaoOVPN avisa quando um .ovpn (que costuma trazer chave privada
// e certificados embutidos) pode ser lido por qualquer usuário da máquina.
func avisarPermissaoOVPN(c *Ctx, arquivo string) {
	if info, err := os.Stat(arquivo); err == nil && info.Mode().Perm()&0o004 != 0 {
		c.Log.Warnf("%s pode ser lido por qualquer usuário (permissão %o) e costuma conter chaves privadas: considere 'chmod 600'.", filepath.Base(arquivo), info.Mode().Perm())
	}
}

// O pacote "-gnome" traz o diálogo de autenticação GTK. No Arch o pacote
// oficial já o inclui.
func instalarPluginOpenVPN(c *Ctx) error {
	switch c.Sys.PkgMgr {
	case "dnf":
		if c.Probe("rpm", "-q", "NetworkManager-openvpn") {
			return nil
		}
		return c.install("NetworkManager-openvpn", "NetworkManager-openvpn-gnome")
	case "pacman":
		if c.Probe("pacman", "-Qq", "networkmanager-openvpn") {
			return nil
		}
		return c.install("networkmanager-openvpn")
	default:
		if c.Probe("dpkg", "-s", "network-manager-openvpn") {
			return nil
		}
		return c.install("network-manager-openvpn", "network-manager-openvpn-gnome")
	}
}

var (
	reDNS    = regexp.MustCompile(`(?m)^\s*dhcp-option\s+DNS\s+(\S+)`)
	reDomain = regexp.MustCompile(`(?m)^\s*dhcp-option\s+DOMAIN(?:-SEARCH)?\s+(\S+)`)
)

// opcoesDNS extrai as diretivas "dhcp-option DNS" e "dhcp-option DOMAIN" /
// "DOMAIN-SEARCH" de um arquivo .ovpn.
func opcoesDNS(conteudo string) (dns, dominios []string) {
	for _, m := range reDNS.FindAllStringSubmatch(conteudo, -1) {
		dns = append(dns, m[1])
	}
	for _, m := range reDomain.FindAllStringSubmatch(conteudo, -1) {
		dominios = append(dominios, m[1])
	}
	return dns, dominios
}

var virtManager = Module{
	ID:    "virt_manager",
	Title: "Instalar virt-manager (QEMU/KVM)",
	Desc:  "QEMU/KVM, libvirt e a interface gráfica virt-manager para máquinas virtuais",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando virt-manager (QEMU/KVM + libvirt)...")

		if cpu, err := os.ReadFile("/proc/cpuinfo"); err == nil &&
			!strings.Contains(string(cpu), "vmx") && !strings.Contains(string(cpu), "svm") {
			c.Log.Warnf("CPU sem suporte a virtualização por hardware (VT-x/AMD-V) ou não habilitado na BIOS/UEFI — o KVM pode não funcionar.")
		}

		var pkgs []string
		switch c.Sys.PkgMgr {
		case "dnf":
			pkgs = []string{"qemu-kvm", "libvirt", "virt-install", "virt-manager"}
		case "pacman":
			pkgs = []string{"qemu-desktop", "libvirt", "virt-manager"}
		default:
			// "qemu-kvm" virou pacote virtual em versões recentes do Ubuntu;
			// "qemu-system-x86" é o pacote real que o provê.
			pkgs = []string{"qemu-system-x86", "libvirt-daemon-system", "libvirt-clients", "bridge-utils", "virt-manager"}
		}
		err := step(
			func() error { return c.install(pkgs...) },
			func() error { return c.Run("systemctl", "enable", "--now", "libvirtd") },
			func() error { return c.addUserToGroup("libvirt") },
			func() error { return c.addUserToGroup("kvm") },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("virt-manager instalado. '%s' foi adicionado aos grupos libvirt/kvm (é preciso reabrir a sessão para valer).", c.User)
		return nil
	},
}

var limpeza = Module{
	ID:    "limpeza",
	Title: "Limpeza final",
	Desc:  "Remove pacotes órfãos e limpa o cache do gerenciador de pacotes",
	Run: func(c *Ctx) error {
		c.Log.Infof("Limpando o sistema...")
		var err error
		switch c.Sys.PkgMgr {
		case "dnf":
			err = step(
				func() error { return c.Run("dnf", "autoremove", "-y") },
				func() error { return c.Run("dnf", "clean", "all") },
			)
		case "pacman":
			if orfaos, _ := c.Output("pacman", "-Qtdq"); strings.TrimSpace(orfaos) != "" {
				err = c.Run("pacman", append([]string{"-Rns", "--noconfirm"}, strings.Fields(orfaos)...)...)
			}
			if err == nil {
				err = c.Run("pacman", "-Sc", "--noconfirm")
			}
		default:
			err = step(
				func() error { return c.Run("apt", "autoremove", "-y") },
				func() error { return c.Run("apt", "clean") },
			)
		}
		if err != nil {
			return err
		}

		// Os clones feitos como usuário usam mktemp e se apagam sozinhos; aqui
		// só sobra o diretório de trabalho desta execução (o main também o
		// remove ao sair, isto só libera o espaço mais cedo).
		if err := c.RemoveAll(c.WorkDir); err != nil {
			c.Log.Warnf("Não foi possível remover %s: %v", c.WorkDir, err)
		}
		c.Log.Successf("Instalação finalizada com sucesso!")
		c.Log.Infof("Recomenda-se reiniciar a máquina para aplicar as mudanças de grupo e kernel.")
		return nil
	},
}
