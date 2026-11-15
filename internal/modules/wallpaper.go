package modules

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var papelParede = Module{
	ID:    "papel_parede",
	Grupo: GrupoGNOME,
	Title: "Papel de parede",
	Desc:  "Copia o papel de parede escolhido para ~/.local/share/backgrounds e aplica no GNOME",
	Run: func(c *Ctx) error {
		return aplicarPapelParede(c, func(destino string) string {
			uri := uriArquivo(destino)
			return `gsettings set org.gnome.desktop.background picture-uri ` + shQuote(uri) + `
gsettings set org.gnome.desktop.background picture-uri-dark ` + shQuote(uri) + `
gsettings set org.gnome.desktop.background picture-options 'zoom'`
		})
	},
	Arch: &Variante{},
	// No KDE, o plasma-apply-wallpaperimage aplica pela sessão do Plasma
	// (precisa dela aberta).
	KDE: &Variante{
		Desc: "Copia o papel de parede escolhido para ~/.local/share/backgrounds e aplica no KDE Plasma",
		Run: func(c *Ctx) error {
			return aplicarPapelParede(c, func(destino string) string {
				return `plasma-apply-wallpaperimage ` + shQuote(destino)
			})
		},
	},
	Fedora: &Variante{},
	Mint: &Variante{
		Desc: "Copia o papel de parede escolhido para ~/.local/share/backgrounds e aplica no Cinnamon",
		Run: func(c *Ctx) error {
			return aplicarPapelParede(c, func(destino string) string {
				return `gsettings set org.cinnamon.desktop.background picture-uri ` + shQuote(uriArquivo(destino)) + `
gsettings set org.cinnamon.desktop.background picture-options 'zoom'`
			})
		},
	},
}

// aplicarPapelParede copia a imagem escolhida e roda, como o usuário, os
// comandos que aplicar devolve para o caminho da cópia.
func aplicarPapelParede(c *Ctx, aplicar func(destino string) string) error {
	if c.Wallpaper == "" {
		c.Log.Infof("Nenhum papel de parede escolhido: mantendo o atual.")
		return nil
	}
	home, err := userHome(c)
	if err != nil {
		return err
	}
	// Mesmo lugar onde o próprio GNOME guarda as imagens escolhidas em
	// Configurações → Plano de fundo.
	destino := filepath.Join(home, ".local/share/backgrounds", filepath.Base(c.Wallpaper))

	c.Log.Infof("Aplicando o papel de parede '%s'...", filepath.Base(c.Wallpaper))
	if err := c.AsUser("set -e\ninstall -D -m 0644 " + shQuote(c.Wallpaper) + " " + shQuote(destino) + "\n" + aplicar(destino)); err != nil {
		return err
	}
	c.Log.Successf("Papel de parede aplicado: %s", destino)
	return nil
}

// uriArquivo é a URI file:// de um caminho, como o gsettings espera.
func uriArquivo(caminho string) string {
	return (&url.URL{Scheme: "file", Path: caminho}).String()
}

// userHome devolve a home do usuário alvo (via NSS, cobrindo contas de domínio).
func userHome(c *Ctx) (string, error) {
	out, err := c.Output("getent", "passwd", c.User)
	if err != nil {
		return "", err
	}
	campos := strings.Split(strings.TrimSpace(out), ":")
	if len(campos) < 6 || campos[5] == "" {
		return "", errorf("não foi possível descobrir a home de %s", c.User)
	}
	return campos[5], nil
}

// ListWallpapers devolve as imagens da pasta de papéis de parede, em ordem
// alfabética.
func ListWallpapers(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && slices.Contains([]string{".jpg", ".jpeg", ".png", ".webp"}, ext) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// shQuote protege um valor para uso dentro de um script de shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
