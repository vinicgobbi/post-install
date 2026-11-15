package modules

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// No KDE, o módulo apps_gnome recria o painel do Plasma: o mesmo layout da
// máquina de referência (barra inferior flutuante de 46 px, com o menu e os
// apps fixados no centro) e, como launchers do gerenciador de tarefas, os
// apps de GNOME/favoritos.txt instalados na máquina.
var painelKDE = &Variante{
	Title: "Painel do Plasma",
	Desc:  "Recria o painel do Plasma (barra inferior flutuante com mídia, menu, apps fixados de GNOME/favoritos.txt, bandeja, relógio e mostrar a área de trabalho) e aplica na sessão atual",
	Run:   aplicarPainelKDE,
}

func aplicarPainelKDE(c *Ctx) error {
	if _, err := c.LookPath("plasmashell"); err != nil {
		c.Log.Warnf("KDE Plasma não instalado: pulando o painel.")
		return nil
	}
	var launchers []string
	favs, err := lerFavoritos(c, filepath.Join(c.GNOMEDir, "favoritos.txt"))
	switch {
	case errors.Is(err, os.ErrNotExist):
		c.Log.Warnf("GNOME/favoritos.txt não encontrado: painel sem apps fixados.")
	case err != nil:
		return err
	default:
		home, err := userHome(c)
		if err != nil {
			return err
		}
		apps := appsInstalados(c, dirsAplicativos(home), desktopsAtuais(c))
		launchers = escolherFavoritos(favs, apps)
		c.Log.Infof("Apps fixados: %s", strings.Join(launchers, ", "))
	}
	js, err := scriptPainelKDE(launchers)
	if err != nil {
		return err
	}
	// O plasmashell roda o script na sessão em execução (precisa do D-Bus da
	// sessão gráfica, que o AsUser aponta); o painel muda na hora.
	if err := c.AsUser("busctl --user call org.kde.plasmashell /PlasmaShell org.kde.PlasmaShell evaluateScript s " + shQuote(js)); err != nil {
		return err
	}
	c.Log.Successf("Painel do Plasma recriado.")
	return nil
}

// Itens da bandeja do sistema: extraItems são os plasmoides ativos,
// shownItems os sempre visíveis e hiddenItems os que ficam no menu
// expandido (inclui ícones de apps, como o do Solaar). knownItems evita que
// a bandeja ative sozinha os que ficaram de fora (clima, dwell clicker).
var (
	bandejaExtra = []string{
		"org.kde.kdeconnect", "org.kde.plasma.vault", "org.kde.kscreen", "org.kde.plasma.battery",
		"org.kde.plasma.bluetooth", "org.kde.plasma.brightness", "org.kde.plasma.cameraindicator",
		"org.kde.plasma.clipboard", "org.kde.plasma.devicenotifier", "org.kde.plasma.keyboardindicator",
		"org.kde.plasma.keyboardlayout", "org.kde.plasma.manage-inputmethod", "org.kde.plasma.mediacontroller",
		"org.kde.plasma.networkmanagement", "org.kde.plasma.notifications", "org.kde.plasma.printmanager",
		"org.kde.plasma.volume",
	}
	bandejaVisiveis = []string{
		"org.kde.plasma.networkmanagement", "org.kde.plasma.volume", "org.kde.plasma.battery",
		"org.kde.plasma.notifications",
	}
	bandejaOcultos = []string{
		"org.kde.plasma.bluetooth", "org.kde.plasma.brightness", "org.kde.plasma.clipboard",
		"org.kde.plasma.devicenotifier", "org.kde.kdeconnect", "org.kde.plasma.vault", "org.kde.kscreen",
		"org.kde.plasma.keyboardindicator", "org.kde.plasma.keyboardlayout",
		"org.kde.plasma.manage-inputmethod", "org.kde.plasma.mediacontroller",
		"org.kde.plasma.printmanager", "org.kde.plasma.cameraindicator", "indicator-solaar",
		"Xwayland Video Bridge",
	}
	bandejaConhecidos = append(append([]string{}, bandejaExtra...),
		"org.kde.plasma.dwellclicker", "org.kde.plasma.weather")
)

// scriptPainelKDE monta o script do Plasma (API de scripting do
// plasmashell) que apaga os painéis atuais e cria o painel, com os widgets
// na ordem da esquerda para a direita. Os espaçadores dos dois lados do menu
// e dos apps fixados os deixam centralizados.
func scriptPainelKDE(launchers []string) (string, error) {
	ls := make([]string, len(launchers))
	for i, id := range launchers {
		ls[i] = "applications:" + id
	}
	vals := map[string]any{
		"launchers":  strings.Join(ls, ","),
		"extra":      bandejaExtra,
		"visiveis":   bandejaVisiveis,
		"ocultos":    bandejaOcultos,
		"conhecidos": bandejaConhecidos,
	}
	b, err := json.Marshal(vals)
	if err != nil {
		return "", err
	}
	return `var v = ` + string(b) + `;
panels().forEach(function (p) { p.remove(); });
var p = new Panel("org.kde.panel");
p.location = "bottom";
p.height = 46;
p.floating = true;
p.lengthMode = "fill";
p.alignment = "center";
p.hiding = "none";
p.addWidget("org.kde.plasma.mediacontroller");
p.addWidget("org.kde.plasma.panelspacer");
var menu = p.addWidget("org.kde.plasma.kickoff");
menu.currentConfigGroup = ["General"];
menu.writeConfig("icon", "start-here");
var tarefas = p.addWidget("org.kde.plasma.icontasks");
tarefas.currentConfigGroup = ["General"];
tarefas.writeConfig("fill", false);
tarefas.writeConfig("launchers", v.launchers);
p.addWidget("org.kde.plasma.panelspacer");
var bandeja = p.addWidget("org.kde.plasma.systemtray");
bandeja.currentConfigGroup = ["General"];
bandeja.writeConfig("extraItems", v.extra);
bandeja.writeConfig("shownItems", v.visiveis);
bandeja.writeConfig("hiddenItems", v.ocultos);
bandeja.writeConfig("knownItems", v.conhecidos);
p.addWidget("org.kde.plasma.digitalclock");
p.addWidget("org.kde.plasma.showdesktop");
`, nil
}
