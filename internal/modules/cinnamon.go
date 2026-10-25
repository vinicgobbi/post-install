package modules

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gwlApplet é a lista de janelas padrão do Cinnamon, que guarda os apps
// fixados no painel.
const gwlApplet = "grouped-window-list@cinnamon.org"

const (
	smileFlatpak = "it.mijorus.smile"

	// volumeStep é o caminho do script das teclas de volume, referenciado
	// em CINNAMON/atalhos.dconf.
	volumeStep = "/usr/local/bin/cinnamon-volume-step"
)

var atalhosCinnamon = Module{
	ID:    "atalhos_cinnamon",
	Title: "Atalhos do Cinnamon",
	Desc:  "Aplica CINNAMON/atalhos.dconf: Super+L bloqueia a tela, Print e Shift+Super+S abrem a captura do Flameshot, Shift+Super+R grava a tela, Super+V o histórico da área de transferência (CopyQ), Super+. os emojis (Smile) e volume de 2 em 2%",
	Run: func(c *Ctx) error {
		if !c.Sys.IsCinnamon() {
			c.Log.Infof("Ambiente %s detectado: pulando atalhos do Cinnamon.", c.Sys.DesktopName())
			return nil
		}
		conf := filepath.Join(c.CinnamonDir, "atalhos.dconf")
		if !fileExists(conf) {
			c.Log.Warnf("%s não encontrado: pulando atalhos.", conf)
			return nil
		}

		// Os atalhos chamam esses programas, então eles precisam existir
		// mesmo quando os módulos de apps não foram marcados.
		if err := step(
			func() error { return garantirPacote(c, "flameshot", "Flameshot (captura de tela)") },
			func() error { return garantirPacote(c, "copyq", "CopyQ (histórico da área de transferência)") },
			func() error { return garantirFlatpak(c, smileFlatpak, "Smile") },
			func() error { return c.WriteFile(volumeStep, volumeStepScript, 0o755) },
		); err != nil {
			return err
		}

		if err := c.AsUser(scriptAtalhosCinnamon(conf)); err != nil {
			return err
		}
		c.Log.Successf("Atalhos do Cinnamon aplicados (o CopyQ inicia sozinho no próximo login).")
		return nil
	},
}

// garantirPacote instala, com o gerenciador nativo, o programa chamado por
// um atalho se ele ainda não estiver no PATH (o pacote tem o nome do
// binário).
func garantirPacote(c *Ctx, bin, nome string) error {
	if _, err := lookPath(bin); err == nil {
		return nil
	}
	c.Log.Infof("Instalando o %s...", nome)
	return c.install(bin)
}

// copyqAutostart inicia o CopyQ no login, só no ícone da bandeja.
const copyqAutostart = `[Desktop Entry]
Type=Application
Name=CopyQ
Icon=copyq
Exec=copyq
X-GNOME-Autostart-enabled=true
`

// scriptAtalhosCinnamon aplica os atalhos e prepara o que eles usam: o
// CopyQ no login (com o item escolhido no Super+V colado na janela atual,
// como no Windows, se ainda não houver configuração dele) e o Super+. livre
// do seletor de emojis do IBus, que só funciona em campos de texto com o
// IBus ativo.
func scriptAtalhosCinnamon(conf string) string {
	return `set -e
dconf load /org/cinnamon/desktop/keybindings/ < ` + shQuote(conf) + `
gsettings set org.freedesktop.ibus.panel.emoji hotkey "@as []" 2>/dev/null || true

mkdir -p "$HOME/.config/autostart"
cat > "$HOME/.config/autostart/copyq.desktop" <<'POST_INSTALL_COPYQ'
` + copyqAutostart + `POST_INSTALL_COPYQ
if [ ! -f "$HOME/.config/copyq/copyq.conf" ]; then
    mkdir -p "$HOME/.config/copyq"
    printf '[Options]\ntray_item_paste=true\n' > "$HOME/.config/copyq/copyq.conf"
fi
`
}

// volumeStepScript muda o volume no passo pedido: as teclas de volume do
// Cinnamon têm passo fixo de 5%, sem configuração como o volume-step do
// GNOME.
const volumeStepScript = `#!/bin/sh
# Instalado pelo post-install (módulo atalhos_cinnamon). As teclas de volume
# do Cinnamon têm passo fixo de 5%; este script muda o volume da saída padrão
# no passo pedido ("+2" sobe 2%, "-2" desce 2%) e mostra o OSD do Cinnamon.
set -eu
sinal=${1%%[0-9]*}
passo=${1#[+-]}
case "$sinal" in
+ | -) ;;
*) echo "uso: $0 +N | -N" >&2; exit 2 ;;
esac
case "$passo" in
'' | *[!0-9]*) echo "uso: $0 +N | -N" >&2; exit 2 ;;
esac

# Mesmo teto do Cinnamon: 100%, ou 150% com o volume amplificado ligado.
maximo=100
[ "$(gsettings get org.cinnamon.desktop.sound allow-amplified-volume 2>/dev/null)" = true ] && maximo=150

# Como no Cinnamon, subir o volume tira o mudo; descer, não.
if command -v wpctl >/dev/null 2>&1; then
    [ "$sinal" = + ] && wpctl set-mute @DEFAULT_AUDIO_SINK@ 0
    wpctl set-volume -l "$(awk "BEGIN { print $maximo / 100 }")" @DEFAULT_AUDIO_SINK@ "$passo%$sinal"
    estado=$(wpctl get-volume @DEFAULT_AUDIO_SINK@)
    vol=$(echo "$estado" | awk '{ printf "%d", $2 * 100 + 0.5 }')
    case "$estado" in *MUTED*) mudo=1 ;; *) mudo=0 ;; esac
else
    [ "$sinal" = + ] && pactl set-sink-mute @DEFAULT_SINK@ 0
    pactl set-sink-volume @DEFAULT_SINK@ "$sinal$passo%"
    vol=$(pactl get-sink-volume @DEFAULT_SINK@ | grep -o '[0-9]*%' | head -n 1 | tr -d %)
    if [ "$vol" -gt "$maximo" ]; then
        pactl set-sink-volume @DEFAULT_SINK@ "$maximo%"
        vol=$maximo
    fi
    case "$(pactl get-sink-mute @DEFAULT_SINK@)" in *yes*) mudo=1 ;; *) mudo=0 ;; esac
fi

if [ "$mudo" = 1 ] || [ "$vol" -eq 0 ]; then
    icone=audio-volume-muted-symbolic
elif [ "$vol" -lt 34 ]; then
    icone=audio-volume-low-symbolic
elif [ "$vol" -lt 67 ]; then
    icone=audio-volume-medium-symbolic
else
    icone=audio-volume-high-symbolic
fi
gdbus call --session --dest org.Cinnamon --object-path /org/Cinnamon --method org.Cinnamon.ShowOSD \
    "{'icon': <'$icone'>, 'level': <$vol>}" >/dev/null 2>&1 || true
`

var preferenciasCinnamon = Module{
	ID:    "preferencias_cinnamon",
	Title: "Preferências do Cinnamon",
	Desc:  "Aplica CINNAMON/preferencias.dconf: notificações sem som",
	Run: func(c *Ctx) error {
		if !c.Sys.IsCinnamon() {
			c.Log.Infof("Ambiente %s detectado: pulando preferências do Cinnamon.", c.Sys.DesktopName())
			return nil
		}
		conf := filepath.Join(c.CinnamonDir, "preferencias.dconf")
		if !fileExists(conf) {
			c.Log.Warnf("%s não encontrado: pulando preferências.", conf)
			return nil
		}
		if err := c.AsUser("dconf load /org/cinnamon/ < " + shQuote(conf)); err != nil {
			return err
		}
		c.Log.Successf("Preferências do Cinnamon aplicadas.")
		return nil
	},
}

var painelCinnamon = Module{
	ID:    "painel_cinnamon",
	Title: "Painel do Cinnamon",
	Desc:  "Aplica os applets de CINNAMON/painel.dconf e fixa na lista de janelas os apps de CINNAMON/fixados.txt instalados na máquina",
	Run: func(c *Ctx) error {
		if !c.Sys.IsCinnamon() {
			c.Log.Infof("Ambiente %s detectado: pulando o painel do Cinnamon.", c.Sys.DesktopName())
			return nil
		}
		home, err := userHome(c)
		if err != nil {
			return err
		}

		var script strings.Builder
		script.WriteString("set -e\n")

		// O arquivo de configuração da lista de janelas depende da instância
		// dela no painel: a de painel.dconf, se for aplicado, ou a atual.
		var applets string
		painel := filepath.Join(c.CinnamonDir, "painel.dconf")
		if b, err := os.ReadFile(painel); errors.Is(err, os.ErrNotExist) {
			c.Log.Warnf("CINNAMON/painel.dconf não encontrado: mantendo os applets atuais.")
			if applets, err = c.OutputAsUser("gsettings get org.cinnamon enabled-applets"); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			applets = string(b)
		}

		fixados, err := lerFavoritos(filepath.Join(c.CinnamonDir, "fixados.txt"))
		switch {
		case errors.Is(err, os.ErrNotExist):
			c.Log.Warnf("CINNAMON/fixados.txt não encontrado: mantendo os apps fixados atuais.")
		case err != nil:
			return err
		default:
			inst := instanciaApplet(applets, gwlApplet)
			if inst == "" {
				c.Log.Warnf("%s não está no painel: sem onde fixar os apps.", gwlApplet)
				break
			}
			// O Cinnamon também mostra os apps restritos ao GNOME
			// (OnlyShowIn=GNOME, como o GNOME Terminal).
			dirs := dirsAplicativos(home)
			ids := idsCinnamon(escolherFavoritos(fixados, appsInstalados(dirs, []string{"X-Cinnamon", "GNOME"})), dirs)
			c.Log.Infof("Fixados no painel: %s", strings.Join(ids, ", "))

			arq := filepath.Join(home, ".config/cinnamon/spices", gwlApplet, inst+".json")
			atual, err := os.ReadFile(arq)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			novo, err := fixarApps(atual, ids)
			if err != nil {
				return errorf("%s: %w", arq, err)
			}
			// Antes do painel.dconf: uma instância nova da lista de janelas
			// já encontra o arquivo pronto, em vez de criar um com o padrão.
			script.WriteString("mkdir -p " + shQuote(filepath.Dir(arq)) + "\n")
			script.WriteString("cat > " + shQuote(arq) + " <<'POST_INSTALL_FIXADOS'\n" + string(novo) + "\nPOST_INSTALL_FIXADOS\n")
		}

		if fileExists(painel) {
			script.WriteString("dconf load /org/cinnamon/ < " + shQuote(painel) + "\n")
		}
		script.WriteString(reiniciarCinnamon)

		if err := c.AsUser(script.String()); err != nil {
			return err
		}
		c.Log.Successf("Painel do Cinnamon aplicado.")
		return nil
	},
}

// reiniciarCinnamon recarrega o Cinnamon em execução, que só relê os apps
// fixados ao iniciar (e, se o usuário mexesse neles antes disso, gravaria
// por cima a lista antiga que tem na memória). No X11 as janelas continuam
// abertas, como no Ctrl+Alt+Esc; no Wayland reiniciar derruba a sessão, então
// fica para o próximo login.
const reiniciarCinnamon = `
pid=$(pgrep -u "$USER" -x -n cinnamon || true)
if [ -n "$pid" ]; then
    if tr '\0' ' ' < "/proc/$pid/cmdline" | grep -q -- '--wayland'; then
        echo "Cinnamon em Wayland: os apps fixados aparecem no próximo login."
    else
        dbus-send --session --type=method_call --dest=org.Cinnamon /org/Cinnamon org.Cinnamon.RestartCinnamon boolean:false || true
    fi
fi
`

// instanciaApplet acha, numa lista enabled-applets (no formato do gsettings
// ou do dconf), a instância do applet uuid
// ("painel:lado:posição:uuid:instância").
func instanciaApplet(applets, uuid string) string {
	re := regexp.MustCompile(`:` + regexp.QuoteMeta(uuid) + `:(\d+)`)
	if m := re.FindStringSubmatch(applets); m != nil {
		return m[1]
	}
	return ""
}

// idsCinnamon converte IDs de .desktop nos IDs que o Cinnamon usa: os apps
// exportados pelo Flatpak ganham o sufixo ":flatpak". Vale a primeira pasta
// (na ordem de dirs) onde o .desktop aparece, como em appsInstalados.
func idsCinnamon(ids, dirs []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, d := range dirs {
			if fileExists(filepath.Join(d, id)) {
				if strings.Contains(d, "/flatpak/") {
					id += ":flatpak"
				}
				break
			}
		}
		out = append(out, id)
	}
	return out
}

// fixarApps troca a lista "pinned-apps" no JSON de configuração da lista de
// janelas, mantendo as outras opções. Sem arquivo (applet que ainda não
// rodou), cria um só com essa chave: o Cinnamon completa o resto pelo
// schema do applet, preservando o valor.
func fixarApps(atual []byte, ids []string) ([]byte, error) {
	dados := map[string]any{}
	if len(atual) > 0 {
		if err := json.Unmarshal(atual, &dados); err != nil {
			return nil, err
		}
	}
	chave, _ := dados["pinned-apps"].(map[string]any)
	if chave == nil {
		chave = map[string]any{"type": "generic"}
	}
	if ids == nil {
		ids = []string{}
	}
	chave["value"] = ids
	dados["pinned-apps"] = chave

	// Sem escapar "<" e ">" (dos atalhos, como "<Super>grave"), como o
	// próprio Cinnamon grava.
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(dados); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}
