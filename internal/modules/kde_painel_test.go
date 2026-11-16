package modules

import (
	"strings"
	"testing"
)

const modeloPainelTeste = `# comentário
[Containments][2]
location=4
plugin=org.kde.panel

[Containments][2][Applets][3]
plugin=org.kde.plasma.kickoff

[Containments][2][Applets][7]
plugin=org.kde.plasma.systemtray

[Containments][2][Applets][7][Applets][8]
plugin=org.kde.plasma.volume

[Containments][2][General]
AppletOrder=3;7

[Containments][9]
lastScreen=1
plugin=org.kde.panel
`

const modeloViewsTeste = `[PlasmaViews][Panel 2][Defaults]
thickness=46

[PlasmaViews][Panel 9][Defaults]
thickness=46
`

// Sem colisão (máquina nova: área de trabalho 1 e o painel padrão 2), os
// IDs do modelo ficam; o painel padrão e a view dele saem.
func TestMesclarPainelSemColisao(t *testing.T) {
	atual := `[Containments][1]
plugin=org.kde.plasma.folder

[Containments][1][Wallpaper][org.kde.image][General]
Image=file:///foto.jpg

[Containments][2]
plugin=org.kde.panel

[Containments][2][Applets][5]
plugin=org.kde.plasma.icontasks

[ScreenMapping]
screenMapping=
`
	views := `[PlasmaViews][Panel 2]
floating=0

[Updates]
performed=x
`
	applets, novasViews, n := mesclarPainel(lerINI(atual), lerINI(views), lerINI(modeloPainelTeste), lerINI(modeloViewsTeste))
	if n != 2 {
		t.Errorf("painéis = %d, esperado 2", n)
	}
	for _, want := range []string{"Image=file:///foto.jpg", "[ScreenMapping]", "[Containments][2][Applets][7][Applets][8]", "AppletOrder=3;7", "[Containments][9]"} {
		if !strings.Contains(applets, want) {
			t.Errorf("faltou %q em:\n%s", want, applets)
		}
	}
	for _, nao := range []string{"icontasks", "comentário"} {
		if strings.Contains(applets, nao) {
			t.Errorf("sobrou %q em:\n%s", nao, applets)
		}
	}
	if strings.Contains(novasViews, "floating=0") || !strings.Contains(novasViews, "[Updates]") || !strings.Contains(novasViews, "[PlasmaViews][Panel 9][Defaults]") {
		t.Errorf("views erradas:\n%s", novasViews)
	}
}

// Com colisão (a área de trabalho do segundo monitor usa o ID 9), os IDs do
// modelo são somados ao maior em uso, inclusive na ordem dos widgets e nas
// views.
func TestMesclarPainelComColisao(t *testing.T) {
	atual := `[Containments][1]
plugin=org.kde.plasma.folder

[Containments][9]
plugin=org.kde.plasma.folder
`
	applets, views, _ := mesclarPainel(lerINI(atual), nil, lerINI(modeloPainelTeste), lerINI(modeloViewsTeste))
	for _, want := range []string{"[Containments][1]\n", "[Containments][9]\nplugin=org.kde.plasma.folder", "[Containments][11]\n", "[Containments][11][Applets][16][Applets][17]", "AppletOrder=12;16", "[Containments][18]\n"} {
		if !strings.Contains(applets, want) {
			t.Errorf("faltou %q em:\n%s", want, applets)
		}
	}
	if !strings.Contains(views, "[PlasmaViews][Panel 11][Defaults]") || !strings.Contains(views, "[PlasmaViews][Panel 18][Defaults]") {
		t.Errorf("views sem renumerar:\n%s", views)
	}
}
