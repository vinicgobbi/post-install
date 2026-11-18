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
	// (precisa dela aberta); a tela de bloqueio é o kscreenlockerrc, e a de
	// login (Plasma Login ou SDDM), a configuração do sistema.
	KDE: &Variante{
		Desc: "Copia o papel de parede escolhido para ~/.local/share/backgrounds e aplica no KDE Plasma, na tela de bloqueio e na tela de login (Plasma Login ou SDDM)",
		Run: func(c *Ctx) error {
			return step(
				func() error {
					return aplicarPapelParede(c, func(destino string) string {
						return `plasma-apply-wallpaperimage ` + shQuote(destino) + "\n" + bloqueioKDE(uriArquivo(destino))
					})
				},
				func() error { return telaLoginKDE(c) },
			)
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

// bloqueioKDE são os comandos (como o usuário) que põem a imagem na tela de
// bloqueio do Plasma, como Configurações → Tela de bloqueio faz.
func bloqueioKDE(uri string) string {
	grupo := `kwriteconfig6 --file kscreenlockerrc --group Greeter `
	imagem := grupo + `--group Wallpaper --group org.kde.image --group General `
	return grupo + `--key WallpaperPlugin org.kde.image
` + imagem + `--key Image ` + shQuote(uri) + `
` + imagem + `--key PreviewImage ` + shQuote(uri)
}

// telaLoginKDE põe o papel de parede escolhido na tela de login: no Plasma
// Login Manager (Plasma 6.5+) e/ou no SDDM, o que estiver instalado.
func telaLoginKDE(c *Ctx) error {
	if c.Wallpaper == "" {
		return nil
	}
	if c.Exists("plasmalogin") {
		if err := fundoPlasmaLogin(c); err != nil {
			return err
		}
	}
	if c.Exists("sddm") {
		return fundoSDDM(c)
	}
	return nil
}

const (
	usuarioPlasmaLogin = "plasmalogin"
	confPlasmaLogin    = "/etc/plasmalogin.conf"
)

// fundoPlasmaLogin faz o mesmo que o KCM do Plasma Login: copia a imagem
// para ~plasmalogin/wallpapers (a home do greeter, que só ele lê) e grava a
// URI em [Greeter][Wallpaper][org.kde.image][General] do plasmalogin.conf.
func fundoPlasmaLogin(c *Ctx) error {
	home, err := homeDe(c, usuarioPlasmaLogin)
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "wallpapers")
	destino := filepath.Join(dir, filepath.Base(c.Wallpaper))
	dono := []string{"-o", usuarioPlasmaLogin, "-g", usuarioPlasmaLogin}

	c.Log.Infof("Aplicando o papel de parede na tela de login (Plasma Login)...")
	if err := step(
		func() error { return c.Run("install", append(append([]string{"-d", "-m", "0755"}, dono...), dir)...) },
		func() error {
			return c.Run("install", append(append([]string{"-m", "0644"}, dono...), c.Wallpaper, destino)...)
		},
		func() error {
			return c.Run("kwriteconfig6", "--file", confPlasmaLogin, "--group", "Greeter", "--group", "Wallpaper",
				"--group", "org.kde.image", "--group", "General", "--key", "Image", uriArquivo(destino))
		},
	); err != nil {
		return err
	}
	c.Log.Successf("Tela de login (Plasma Login) com o papel de parede: %s", destino)
	return nil
}

const (
	temasSDDM = "/usr/share/sddm/themes"
	// fundosSDDM fica fora da home (que o usuário sddm não lê) e do tema
	// (que é do pacote).
	fundosSDDM = "/usr/local/share/backgrounds"
)

// fundoSDDM grava o fundo no theme.conf.user do tema atual do SDDM (o
// Breeze e os derivados leem a chave background dele), como o KCM do SDDM.
func fundoSDDM(c *Ctx) error {
	tema := temaSDDM(c)
	dirTema := filepath.Join(temasSDDM, tema)
	if !c.FileExists(filepath.Join(dirTema, "theme.conf")) {
		c.Log.Warnf("Tema '%s' do SDDM não encontrado em %s: mantendo o fundo da tela de login.", tema, temasSDDM)
		return nil
	}
	destino := filepath.Join(fundosSDDM, filepath.Base(c.Wallpaper))
	conf := filepath.Join(dirTema, "theme.conf.user")
	geral := []string{"--file", conf, "--group", "General", "--key"}

	c.Log.Infof("Aplicando o papel de parede na tela de login (SDDM, tema '%s')...", tema)
	if err := step(
		func() error { return c.Run("install", "-D", "-m", "0644", c.Wallpaper, destino) },
		func() error { return c.Run("kwriteconfig6", append(geral, "type", "image")...) },
		func() error { return c.Run("kwriteconfig6", append(geral, "background", destino)...) },
	); err != nil {
		return err
	}
	c.Log.Successf("Tela de login (SDDM) com o papel de parede: %s", destino)
	return nil
}

// temaSDDM é o [Theme] Current da configuração do SDDM, lida na ordem em que
// ele a lê (o último valor vale); sem nenhum, o Breeze.
func temaSDDM(c *Ctx) string {
	var confs []string
	for _, padrao := range []string{"/usr/lib/sddm/sddm.conf.d/*.conf", "/etc/sddm.conf.d/*.conf"} {
		achados, _ := c.Glob(padrao)
		confs = append(confs, achados...)
	}
	tema := "breeze"
	for _, conf := range append(confs, "/etc/sddm.conf") {
		b, err := c.ReadFile(conf)
		if err != nil {
			continue
		}
		for _, s := range lerINI(string(b)) {
			if len(s.grupos) == 1 && s.grupos[0] == "Theme" {
				if v := strings.TrimSpace(s.valor("Current")); v != "" {
					tema = v
				}
			}
		}
	}
	return tema
}

// uriArquivo é a URI file:// de um caminho, como o gsettings espera.
func uriArquivo(caminho string) string {
	return (&url.URL{Scheme: "file", Path: caminho}).String()
}

// userHome devolve a home do usuário alvo (via NSS, cobrindo contas de domínio).
func userHome(c *Ctx) (string, error) {
	return homeDe(c, c.User)
}

// homeDe devolve a home de um usuário (via NSS).
func homeDe(c *Ctx, usuario string) (string, error) {
	out, err := c.Output("getent", "passwd", usuario)
	if err != nil {
		return "", err
	}
	campos := strings.Split(strings.TrimSpace(out), ":")
	if len(campos) < 6 || campos[5] == "" {
		return "", errorf("não foi possível descobrir a home de %s", usuario)
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
