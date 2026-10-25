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
		uri := (&url.URL{Scheme: "file", Path: destino}).String()

		c.Log.Infof("Aplicando o papel de parede '%s'...", filepath.Base(c.Wallpaper))
		aplicar := `gsettings set org.gnome.desktop.background picture-uri ` + shQuote(uri) + `
gsettings set org.gnome.desktop.background picture-uri-dark ` + shQuote(uri) + `
gsettings set org.gnome.desktop.background picture-options 'zoom'`
		if err := c.AsUser("set -e\ninstall -D -m 0644 " + shQuote(c.Wallpaper) + " " + shQuote(destino) + "\n" + aplicar); err != nil {
			return err
		}
		c.Log.Successf("Papel de parede aplicado: %s", destino)
		return nil
	},
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
