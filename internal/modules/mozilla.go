package modules

import (
	"strings"

	"github.com/vinicgobbi/post-install/internal/runner"
)

// No Ubuntu, os pacotes "firefox" e "thunderbird" do repositório oficial são
// só pacotes de transição que instalam o snap. Usamos o repositório APT da
// Mozilla (https://packages.mozilla.org/apt) com pinning para que os .deb
// dela sempre ganhem — e os de transição do Ubuntu nunca sejam instalados.
const (
	mozillaKeyring     = "/etc/apt/keyrings/packages.mozilla.org.asc"
	mozillaFingerprint = "35BAA0B33E9EB396F59CA838C0BA5CE6DC6315A3"
)

// usaRepoMozilla diz se o sistema usa o repositório da Mozilla (só Ubuntu e
// sabores com ID=ubuntu; Mint e Debian já trazem .deb de verdade).
func usaRepoMozilla(c *Ctx) bool {
	return c.Sys.PkgMgr == "apt" && c.Sys.ID == "ubuntu"
}

const mozillaPreferences = `Package: *
Pin: origin packages.mozilla.org
Pin-Priority: 1000

# Nunca instalar os pacotes de transição do Ubuntu, que puxam o snap.
Package: firefox* thunderbird*
Pin: release o=Ubuntu
Pin-Priority: -1
`

func configurarRepoMozilla(c *Ctx) error {
	c.Log.Infof("Configurando o repositório APT da Mozilla (Firefox e Thunderbird sem snap)...")

	tmp := c.Temp("packages.mozilla.org.asc")
	if err := c.Download("https://packages.mozilla.org/apt/repo-signing-key.gpg", tmp); err != nil {
		return err
	}
	if !c.DryRun {
		out, err := c.Output("gpg", "-n", "-q", "--import", "--import-options", "import-show", tmp)
		if err != nil {
			return err
		}
		if !strings.Contains(out, mozillaFingerprint) {
			return errorf("a chave baixada de packages.mozilla.org não tem o fingerprint esperado (%s)", mozillaFingerprint)
		}
	}

	return step(
		func() error { return c.Run("install", "-m", "0644", tmp, mozillaKeyring) },
		func() error {
			return c.WriteFile("/etc/apt/sources.list.d/mozilla.sources", "Types: deb\n"+
				"URIs: https://packages.mozilla.org/apt\n"+
				"Suites: mozilla thunderbird-deb\n"+
				"Components: main\n"+
				"Signed-By: "+mozillaKeyring+"\n", 0o644)
		},
		func() error { return c.WriteFile("/etc/apt/preferences.d/mozilla", mozillaPreferences, 0o644) },
	)
}

// instalarFirefoxThunderbird instala os dois como pacotes nativos.
func instalarFirefoxThunderbird(c *Ctx) error {
	c.Log.Infof("Instalando Firefox e Thunderbird (pacotes nativos)...")
	switch {
	case usaRepoMozilla(c):
		// --allow-downgrades: o .deb da Mozilla substitui o pacote de transição
		// do Ubuntu, que tem epoch maior (ex.: 1:1snap1).
		if err := c.Run("apt", "install", "-y", "--allow-downgrades", "firefox", "thunderbird"); err != nil {
			return err
		}
		removerSnapsMozilla(c)
		return nil
	case c.Sys.PkgMgr == "apt" && !c.Probe("apt-cache", "show", "firefox"):
		// Debian puro só tem o Firefox ESR.
		return c.install("firefox-esr", "thunderbird")
	default:
		return c.install("firefox", "thunderbird")
	}
}

// removerSnapsMozilla remove as versões snap, depois de os .deb já estarem
// instalados. Antes, copia o perfil do snap para o local do .deb (quando o
// usuário ainda não tem um), para não perder favoritos, senhas e e-mails.
// Sem --purge: o snapd guarda um snapshot automático por alguns dias.
func removerSnapsMozilla(c *Ctx) {
	if !runner.Exists("snap") {
		return
	}
	perfis := map[string]struct{ origem, destino string }{
		"firefox":     {"$HOME/snap/firefox/common/.mozilla/firefox", "$HOME/.mozilla/firefox"},
		"thunderbird": {"$HOME/snap/thunderbird/common/.thunderbird", "$HOME/.thunderbird"},
	}
	for _, nome := range []string{"firefox", "thunderbird"} {
		if !c.Probe("snap", "list", nome) {
			continue
		}
		p := perfis[nome]
		if err := c.AsUser(`if [ -d "` + p.origem + `" ] && [ ! -e "` + p.destino + `" ]; then
    mkdir -p "$(dirname "` + p.destino + `")"
    cp -a "` + p.origem + `" "` + p.destino + `"
    echo "Perfil do snap copiado para ` + p.destino + `"
fi`); err != nil {
			c.Log.Warnf("Falha ao copiar o perfil do %s (snap): %v — mantendo o snap instalado.", nome, err)
			continue
		}
		if err := c.Run("snap", "remove", nome); err != nil {
			c.Log.Warnf("Falha ao remover o snap do %s: %v", nome, err)
			continue
		}
		c.Log.Successf("Snap do %s removido (versão .deb da Mozilla instalada no lugar).", nome)
	}
}
