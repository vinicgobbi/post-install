package modules

import (
	"strconv"
	"strings"

	"github.com/vinicgobbi/post-install/internal/runner"
)

// No Ubuntu, os pacotes "firefox" e "thunderbird" do repositório oficial são
// só pacotes de transição que instalam o snap. Seguimos a instrução oficial
// da Mozilla para usar o repositório APT dela (https://packages.mozilla.org/apt):
// chave em /etc/apt/keyrings, conferência do fingerprint, repositório com as
// suítes "mozilla" (Firefox) e "thunderbird-deb" (Thunderbird) e pinning com
// prioridade 1000, para os .deb da Mozilla sempre ganharem dos do Ubuntu.
const (
	mozillaKeyring     = "/etc/apt/keyrings/packages.mozilla.org.asc"
	mozillaFingerprint = "35BAA0B33E9EB396F59CA838C0BA5CE6DC6315A3"
	mozillaSources     = "/etc/apt/sources.list.d/mozilla.sources"
	mozillaList        = "/etc/apt/sources.list.d/mozilla.list"
)

const mozillaPreferences = `Package: *
Pin: origin packages.mozilla.org
Pin-Priority: 1000
`

// usaFormatoDeb822 segue a instrução da Mozilla: arquivo .sources (deb822) do
// Ubuntu 26.04 (Resolute) em diante; .list de uma linha no 24.04 (Noble) e
// anteriores.
func usaFormatoDeb822(versao string) bool {
	maior, menor, _ := strings.Cut(versao, ".")
	ma, err1 := strconv.Atoi(maior)
	mi, err2 := strconv.Atoi(menor)
	if err1 != nil || err2 != nil {
		return true
	}
	return ma > 24 || (ma == 24 && mi > 4)
}

func configurarRepoMozilla(c *Ctx) error {
	c.Log.Infof("Configurando o repositório APT da Mozilla (Firefox e Thunderbird sem snap)...")

	// Um único arquivo para as duas suítes; remove o do outro formato, se
	// existir (ex.: configurado à mão antes), para o apt não acusar
	// repositório duplicado.
	arquivo, conteudo, antigo := mozillaSources, "Types: deb\n"+
		"URIs: https://packages.mozilla.org/apt\n"+
		"Suites: mozilla thunderbird-deb\n"+
		"Components: main\n"+
		"Signed-By: "+mozillaKeyring+"\n", mozillaList
	if !usaFormatoDeb822(c.Sys.VersionID) {
		arquivo, antigo = mozillaList, mozillaSources
		conteudo = "deb [signed-by=" + mozillaKeyring + "] https://packages.mozilla.org/apt mozilla main\n" +
			"deb [signed-by=" + mozillaKeyring + "] https://packages.mozilla.org/apt thunderbird-deb main\n"
	}

	return step(
		func() error {
			return instalarChaveAPT(c, "https://packages.mozilla.org/apt/repo-signing-key.gpg", mozillaKeyring, mozillaFingerprint)
		},
		func() error { return c.RemoveAll(antigo) },
		func() error { return c.WriteFile(arquivo, conteudo, 0o644) },
		func() error { return c.WriteFile("/etc/apt/preferences.d/mozilla", mozillaPreferences, 0o644) },
	)
}

// instalarFirefoxThunderbird instala os .deb da Mozilla e remove os snaps.
func instalarFirefoxThunderbird(c *Ctx) error {
	c.Log.Infof("Instalando Firefox e Thunderbird (.deb da Mozilla)...")
	// --allow-downgrades: o .deb da Mozilla substitui o pacote de transição
	// do Ubuntu, que tem epoch maior (ex.: 1:1snap1); sem ele o "apt -y"
	// recusa a troca.
	if err := c.Run("apt", "install", "-y", "--allow-downgrades", "firefox", "thunderbird"); err != nil {
		return err
	}
	removerSnapsMozilla(c)
	return nil
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

var firefoxThunderbird = Module{
	ID:    "firefox_thunderbird",
	Grupo: GrupoAplicativos,
	Title: "Firefox e Thunderbird (sem snap)",
	Desc:  "Repositório APT da Mozilla e os .deb do Firefox e do Thunderbird, removendo as versões snap (o perfil do snap é copiado antes)",
	Run: func(c *Ctx) error {
		err := step(
			func() error { return configurarRepoMozilla(c) },
			func() error { return c.Run("apt", "update") },
			func() error { return instalarFirefoxThunderbird(c) },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Firefox e Thunderbird instalados pelos .deb da Mozilla.")
		return nil
	},
}
