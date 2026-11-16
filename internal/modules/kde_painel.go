package modules

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// No KDE, o módulo apps_gnome deixa os painéis do Plasma exatamente como os
// da máquina de referência: aplica o modelo GNOME/painel-kde.appletsrc e
// GNOME/painel-kde.plasmashellrc (exportados por
// scripts/exportar-painel-kde.sh), com um painel por monitor, inclusive os
// de monitores que não estejam conectados agora (aparecem quando o monitor
// for ligado). O scripting do plasmashell não cria painel em monitor
// ausente, então os arquivos são gravados direto, com o plasmashell parado.
var painelKDE = &Variante{
	Title: "Painéis do Plasma",
	Desc:  "Substitui os painéis do Plasma pelos de GNOME/painel-kde.appletsrc (um por monitor, com os mesmos widgets, apps fixados, bandeja e configurações), reiniciando o plasmashell",
	Run:   aplicarPainelKDE,
}

const (
	arquivoApplets     = "plasma-org.kde.plasma.desktop-appletsrc"
	arquivoPlasmashell = "plasmashellrc"
)

// scriptPararPlasma para o plasmashell (pelo systemd, como no Plasma 6, ou
// pelo kquitapp6) e espera ele sair: ao sair, ele grava a configuração em
// memória por cima dos arquivos. Sem set -e: o que vale é o plasmashell ter
// saído, conferido no fim; um "systemctl stop" que sai com erro (já visto no
// Kubuntu 26.04, rodando pelo su) cai no kquitapp6 em vez de abortar.
const scriptPararPlasma = `if systemctl --user is-active -q plasma-plasmashell.service 2>/dev/null; then
	systemctl --user stop plasma-plasmashell.service || echo "systemctl --user stop saiu com $?; tentando o kquitapp6" >&2
fi
if pgrep -u "$(id -u)" -x plasmashell >/dev/null; then
	kquitapp6 plasmashell 2>/dev/null || true
fi
for _ in $(seq 50); do
	pgrep -u "$(id -u)" -x plasmashell >/dev/null || exit 0
	sleep 0.2
done
echo "o plasmashell não fechou" >&2
exit 1
`

// scriptIniciarPlasma volta o plasmashell, se houver uma sessão gráfica
// aberta (sem ela, os painéis aparecem no próximo login).
const scriptIniciarPlasma = `if systemctl --user is-active -q graphical-session.target 2>/dev/null; then
	systemctl --user start plasma-plasmashell.service
fi
`

func aplicarPainelKDE(c *Ctx) error {
	if _, err := c.LookPath("plasmashell"); err != nil {
		c.Log.Warnf("KDE Plasma não instalado: pulando os painéis.")
		return nil
	}
	modeloApplets, err := c.ReadFile(filepath.Join(c.GNOMEDir, "painel-kde.appletsrc"))
	if errors.Is(err, fs.ErrNotExist) {
		c.Log.Warnf("GNOME/painel-kde.appletsrc não encontrado: mantendo os painéis atuais.")
		return nil
	}
	if err != nil {
		return err
	}
	modeloViews, err := c.ReadFile(filepath.Join(c.GNOMEDir, "painel-kde.plasmashellrc"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	home, err := userHome(c)
	if err != nil {
		return err
	}
	cfg := filepath.Join(home, ".config")

	c.Log.Infof("Parando o plasmashell para trocar os painéis...")
	if err := c.AsUser(scriptPararPlasma); err != nil {
		// A parada pode ter derrubado o plasmashell no meio do caminho: sem
		// isto, o usuário fica sem painéis até o próximo login.
		_ = c.AsUser(scriptIniciarPlasma)
		return err
	}
	// Lidos depois de parar o plasmashell, já com o que ele gravou ao sair.
	atualApplets, err := lerSeAchar(c, filepath.Join(cfg, arquivoApplets))
	if err != nil {
		return err
	}
	atualViews, err := lerSeAchar(c, filepath.Join(cfg, arquivoPlasmashell))
	if err != nil {
		return err
	}
	applets, views, n := mesclarPainel(lerINI(atualApplets), lerINI(atualViews), lerINI(string(modeloApplets)), lerINI(string(modeloViews)))
	c.Log.Infof("%d painéis no modelo (um por monitor).", n)

	script := "set -e\nmkdir -p " + shQuote(cfg) + "\n" +
		"cat > " + shQuote(filepath.Join(cfg, arquivoApplets)) + " <<'POST_INSTALL_PAINEL'\n" + applets + "POST_INSTALL_PAINEL\n" +
		"cat > " + shQuote(filepath.Join(cfg, arquivoPlasmashell)) + " <<'POST_INSTALL_PAINEL'\n" + views + "POST_INSTALL_PAINEL\n"
	// Volta o plasmashell mesmo se a gravação falhar.
	errGravar := c.AsUser(script)
	if err := c.AsUser(scriptIniciarPlasma); err != nil && errGravar == nil {
		return err
	}
	if errGravar != nil {
		return errGravar
	}
	c.Log.Successf("Painéis do Plasma aplicados (se a sessão não estiver aberta, aparecem no próximo login).")
	return nil
}

func lerSeAchar(c *Ctx, path string) (string, error) {
	b, err := c.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// ---- Arquivos de configuração do KDE (KConfig) ----

// secaoINI é um grupo do KConfig: "[Containments][2][Applets][5]" tem os
// grupos Containments, 2, Applets e 5. Sem grupos, são as linhas antes do
// primeiro cabeçalho.
type secaoINI struct {
	grupos []string
	linhas []string
}

// lerINI separa o arquivo em seções, descartando comentários e linhas em
// branco (o KConfig também os descarta ao regravar).
func lerINI(s string) []secaoINI {
	var out []secaoINI
	for _, linha := range strings.Split(s, "\n") {
		linha = strings.TrimRight(linha, "\r")
		t := strings.TrimSpace(linha)
		switch {
		case t == "" || strings.HasPrefix(t, "#"):
		case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]"):
			out = append(out, secaoINI{grupos: strings.Split(t[1:len(t)-1], "][")})
		default:
			if len(out) == 0 {
				out = append(out, secaoINI{})
			}
			out[len(out)-1].linhas = append(out[len(out)-1].linhas, linha)
		}
	}
	return out
}

func escreverINI(secoes []secaoINI) string {
	var b strings.Builder
	for i, s := range secoes {
		if i > 0 {
			b.WriteString("\n")
		}
		if s.grupos != nil {
			b.WriteString("[" + strings.Join(s.grupos, "][") + "]\n")
		}
		for _, l := range s.linhas {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

func (s secaoINI) valor(chave string) string {
	for _, l := range s.linhas {
		if k, v, ok := strings.Cut(l, "="); ok && k == chave {
			return v
		}
	}
	return ""
}

// idsINI são os IDs numéricos (de contêineres e applets, que o Plasma
// numera num contador só) citados nos cabeçalhos.
func idsINI(secoes []secaoINI) map[int]bool {
	ids := map[int]bool{}
	for _, s := range secoes {
		for i := 1; i < len(s.grupos); i++ {
			if s.grupos[i-1] == "Containments" || s.grupos[i-1] == "Applets" {
				if n, err := strconv.Atoi(s.grupos[i]); err == nil {
					ids[n] = true
				}
			}
		}
	}
	return ids
}

// contemPainel diz se a seção é de uma contêiner da lista (ela e os grupos
// abaixo dela).
func contemPainel(s secaoINI, paineis []string) bool {
	return len(s.grupos) >= 2 && s.grupos[0] == "Containments" && slices.Contains(paineis, s.grupos[1])
}

// mesclarPainel tira dos arquivos atuais os painéis (e as bandejas do
// sistema avulsas, do formato do Plasma 5) e as views deles, e acrescenta os
// do modelo. Se algum ID do modelo já estiver em uso na máquina (ex.: pela
// área de trabalho), os IDs do modelo são somados ao maior em uso. Devolve
// os dois arquivos e o número de painéis do modelo.
func mesclarPainel(applets, views, modApplets, modViews []secaoINI) (string, string, int) {
	var removidos []string
	for _, s := range applets {
		if len(s.grupos) == 2 && s.grupos[0] == "Containments" {
			switch s.valor("plugin") {
			case "org.kde.panel", "org.kde.plasma.private.systemtray":
				removidos = append(removidos, s.grupos[1])
			}
		}
	}
	var mantidos []secaoINI
	for _, s := range applets {
		if !contemPainel(s, removidos) {
			mantidos = append(mantidos, s)
		}
	}

	usados, doModelo := idsINI(mantidos), idsINI(modApplets)
	desloc := 0
	for id := range doModelo {
		if usados[id] {
			for u := range usados {
				desloc = max(desloc, u)
			}
			break
		}
	}
	novoID := func(s string) string {
		n, err := strconv.Atoi(s)
		if err != nil {
			return s
		}
		return strconv.Itoa(n + desloc)
	}

	paineis := 0
	for _, s := range modApplets {
		if len(s.grupos) < 2 || s.grupos[0] != "Containments" {
			continue
		}
		g := slices.Clone(s.grupos)
		for i := 1; i < len(g); i++ {
			if g[i-1] == "Containments" || g[i-1] == "Applets" {
				g[i] = novoID(g[i])
			}
		}
		linhas := slices.Clone(s.linhas)
		if desloc > 0 && g[len(g)-1] == "General" {
			for i, l := range linhas {
				// A ordem dos widgets no painel cita os IDs dos applets.
				if v, ok := strings.CutPrefix(l, "AppletOrder="); ok {
					partes := strings.Split(v, ";")
					for j := range partes {
						partes[j] = novoID(partes[j])
					}
					linhas[i] = "AppletOrder=" + strings.Join(partes, ";")
				}
			}
		}
		if len(g) == 2 && s.valor("plugin") == "org.kde.panel" {
			paineis++
		}
		mantidos = append(mantidos, secaoINI{grupos: g, linhas: linhas})
	}

	ehView := func(s secaoINI, ids []string) bool {
		if len(s.grupos) < 2 || s.grupos[0] != "PlasmaViews" {
			return false
		}
		id, ok := strings.CutPrefix(s.grupos[1], "Panel ")
		return ok && slices.Contains(ids, id)
	}
	var novasViews []secaoINI
	for _, s := range views {
		if !ehView(s, removidos) {
			novasViews = append(novasViews, s)
		}
	}
	for _, s := range modViews {
		if len(s.grupos) < 2 || s.grupos[0] != "PlasmaViews" || !strings.HasPrefix(s.grupos[1], "Panel ") {
			continue
		}
		g := slices.Clone(s.grupos)
		g[1] = "Panel " + novoID(strings.TrimPrefix(g[1], "Panel "))
		novasViews = append(novasViews, secaoINI{grupos: g, linhas: slices.Clone(s.linhas)})
	}
	return escreverINI(mantidos), escreverINI(novasViews), paineis
}
