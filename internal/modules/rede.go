package modules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// instalarTailscalePacote configura o repositório oficial do Tailscale
// (pkgs.tailscale.com, assinado) e instala o pacote — o mesmo que o
// install.sh deles faz, mas sem executar um script baixado como root.
func instalarTailscalePacote(c *Ctx) error {
	const (
		base    = "https://pkgs.tailscale.com/stable/ubuntu"
		keyring = "/usr/share/keyrings/tailscale-archive-keyring.gpg"
	)
	codename := c.Sys.Codename
	return step(
		func() error {
			err := instalarChaveAPT(c, base+"/"+codename+".noarmor.gpg", keyring, fprTailscale)
			if err != nil {
				return errorf("repositório do Tailscale indisponível para o Ubuntu %s: %w", codename, err)
			}
			return nil
		},
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/tailscale.list",
				"deb [signed-by="+keyring+"] "+base+" "+codename+" main\n", 0o644)
		},
		func() error { return c.Run("apt", "update") },
		func() error { return c.install("tailscale") },
	)
}

var tailscale = Module{
	ID:    "tailscale",
	Grupo: GrupoRede,
	Title: "Tailscale + Trayscale",
	Desc:  "Tailscale (VPN mesh, pelo repositório oficial) e o cliente gráfico Trayscale via Flatpak",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o Tailscale e o Trayscale...")
		err := step(
			func() error { return instalarTailscalePacote(c) },
			func() error { return c.Run("systemctl", "enable", "--now", "tailscaled") },
			c.ensureFlathub,
			func() error { return c.flatpakInstall("dev.deedles.Trayscale") },
			func() error { return c.Run("tailscale", "set", "--operator="+c.User) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Tailscale instalado e '%s' definido como operator.", c.User)
		return nil
	},
}

var firewall = Module{
	ID:    "firewall",
	Grupo: GrupoRede,
	Title: "Firewall (ufw + gufw)",
	Desc:  "Instala o ufw e a interface gráfica gufw (não ativa o firewall nem cria regras)",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando o ufw e o gufw...")
		if err := c.install("ufw", "gufw"); err != nil {
			return err
		}
		c.Log.Successf("ufw e gufw instalados.")
		return nil
	},
}

var ovpn = Module{
	ID:    "ovpn",
	Grupo: GrupoRede,
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

// O pacote "-gnome" traz o diálogo de autenticação GTK.
func instalarPluginOpenVPN(c *Ctx) error {
	if c.Probe("dpkg", "-s", "network-manager-openvpn") {
		return nil
	}
	return c.install("network-manager-openvpn", "network-manager-openvpn-gnome")
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
	Grupo: GrupoRede,
	Title: "virt-manager (QEMU/KVM)",
	Desc:  "QEMU/KVM, libvirt e a interface gráfica virt-manager para máquinas virtuais",
	Run: func(c *Ctx) error {
		c.Log.Infof("Instalando virt-manager (QEMU/KVM + libvirt)...")

		if cpu, err := os.ReadFile("/proc/cpuinfo"); err == nil &&
			!strings.Contains(string(cpu), "vmx") && !strings.Contains(string(cpu), "svm") {
			c.Log.Warnf("CPU sem suporte a virtualização por hardware (VT-x/AMD-V) ou não habilitado na BIOS/UEFI — o KVM pode não funcionar.")
		}

		// "qemu-kvm" virou pacote virtual em versões recentes do Ubuntu;
		// "qemu-system-x86" é o pacote real que o provê.
		pkgs := []string{"qemu-system-x86", "libvirt-daemon-system", "libvirt-clients", "bridge-utils", "virt-manager"}
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
