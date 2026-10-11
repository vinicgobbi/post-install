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

		existentes, _ := c.Output("nmcli", "-g", "NAME", "connection", "show")
		for _, arquivo := range arquivos {
			nome := strings.TrimSuffix(filepath.Base(arquivo), ".ovpn")

			// Reimporta do zero para continuar idempotente em reexecuções.
			for _, n := range strings.Split(existentes, "\n") {
				if n == nome {
					c.Try("nmcli", "connection", "delete", nome)
					break
				}
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

// O pacote "-gnome" só traz o diálogo de autenticação GTK; no Plasma quem
// cobre isso é o plasma-nm. No Arch o pacote oficial já cobre os dois.
func instalarPluginOpenVPN(c *Ctx) error {
	switch c.Sys.PkgMgr {
	case "dnf":
		if c.Probe("rpm", "-q", "NetworkManager-openvpn") {
			return nil
		}
		if c.Sys.IsKDE() {
			return c.install("NetworkManager-openvpn")
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
		if c.Sys.IsKDE() {
			return c.install("network-manager-openvpn")
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
			if _, statErr := os.Stat(sudoersAURFile); statErr == nil {
				c.RemoveAll(sudoersAURFile)
				c.Log.Infof("Regra temporária de sudo sem senha para o pacman (liberada para o yay) removida.")
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

		// Remove só o que esta execução criou em /tmp (diretório de trabalho
		// e clones feitos pelos módulos), sem varrer o /tmp inteiro.
		for _, p := range []string{c.WorkDir, "/tmp/yay-build", "/tmp/vscode_nautilus", "/tmp/nautilus-copypath"} {
			c.RemoveAll(p)
		}
		c.Log.Successf("Instalação finalizada com sucesso!")
		c.Log.Infof("Recomenda-se reiniciar a máquina para aplicar as mudanças de grupo e kernel.")
		return nil
	},
}
