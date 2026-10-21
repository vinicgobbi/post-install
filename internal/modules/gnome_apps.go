package modules

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var appsGnome = Module{
	ID:    "apps_gnome",
	Title: "Favoritos e pastas do menu de apps",
	Desc:  "Fixa os apps de GNOME/favoritos.txt na barra de favoritos e recria as pastas do menu de apps por categoria (GNOME/pastas.txt), conforme os apps instalados na máquina",
	Run: func(c *Ctx) error {
		if c.Sys.IsKDE() {
			c.Log.Infof("Ambiente Plasma detectado: pulando favoritos e pastas do GNOME.")
			return nil
		}
		if _, err := lookPath("gnome-shell"); err != nil {
			c.Log.Warnf("GNOME Shell não instalado: pulando favoritos e pastas.")
			return nil
		}
		home, err := userHome(c)
		if err != nil {
			return err
		}
		apps := appsInstalados(dirsAplicativos(home), desktopsAtuais(c))
		c.Log.Infof("%d apps visíveis no menu.", len(apps))

		var script strings.Builder
		script.WriteString("set -e\n")

		if favs, err := lerFavoritos(filepath.Join(c.GNOMEDir, "favoritos.txt")); errors.Is(err, os.ErrNotExist) {
			c.Log.Warnf("GNOME/favoritos.txt não encontrado: mantendo os favoritos atuais.")
		} else if err != nil {
			return err
		} else {
			escolhidos := escolherFavoritos(favs, apps)
			c.Log.Infof("Favoritos: %s", strings.Join(escolhidos, ", "))
			script.WriteString("gsettings set org.gnome.shell favorite-apps " + shQuote(strv(escolhidos)) + "\n")
		}

		pastas, err := lerPastas(filepath.Join(c.GNOMEDir, "pastas.txt"))
		if errors.Is(err, os.ErrNotExist) {
			c.Log.Warnf("GNOME/pastas.txt não encontrado: mantendo as pastas atuais.")
		} else if err != nil {
			return err
		} else {
			org := organizarPastas(pastas, apps)
			for i, p := range pastas {
				if n := len(org[i].membros); n > 0 {
					c.Log.Infof("Pasta %s: %d apps", p.Nome, n)
				}
			}
			if soltos := appsSoltos(org, apps); len(soltos) > 0 {
				c.Log.Infof("Fora das pastas (sem categoria conhecida): %s", strings.Join(soltos, ", "))
			}
			// Do zero: apaga as pastas atuais (inclusive as da distro, como
			// System/Utilities do Ubuntu) e zera a disposição do menu, para o
			// GNOME reordenar tudo.
			script.WriteString("dconf reset -f /org/gnome/desktop/app-folders/\n")
			script.WriteString("dconf load /org/gnome/desktop/app-folders/ <<'POST_INSTALL_PASTAS'\n")
			script.WriteString(dconfPastas(pastas, org, diretorioExiste))
			script.WriteString("POST_INSTALL_PASTAS\n")
			script.WriteString("gsettings reset org.gnome.shell app-picker-layout\n")
		}

		if err := c.AsUser(script.String()); err != nil {
			return err
		}
		c.Log.Successf("Favoritos e pastas do menu de apps aplicados.")
		return nil
	},
}

// ---- Configuração ----

// pasta é uma seção de GNOME/pastas.txt.
type pasta struct {
	Nome       string
	Diretorio  string // .directory com o nome traduzido (opcional)
	Categorias []string
	Apps       []string // IDs de .desktop colocados na pasta à força
}

// lerPastas lê as seções "[Nome]" com "diretorio = X.directory",
// "categorias = A B" e "apps = x y", na ordem do arquivo (que é a
// prioridade).
func lerPastas(path string) ([]pasta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []pasta
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		linha := strings.TrimSpace(sc.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		if nome, ok := strings.CutPrefix(linha, "["); ok {
			out = append(out, pasta{Nome: strings.TrimSpace(strings.TrimSuffix(nome, "]"))})
			continue
		}
		k, v, ok := strings.Cut(linha, "=")
		if !ok || len(out) == 0 {
			return nil, errorf("%s: linha inválida: %q", filepath.Base(path), linha)
		}
		p := &out[len(out)-1]
		switch strings.TrimSpace(k) {
		case "diretorio":
			p.Diretorio = strings.TrimSpace(v)
		case "categorias":
			p.Categorias = strings.Fields(v)
		case "apps":
			p.Apps = strings.Fields(v)
		default:
			return nil, errorf("%s: chave desconhecida %q em [%s]", filepath.Base(path), strings.TrimSpace(k), p.Nome)
		}
	}
	return out, sc.Err()
}

// lerFavoritos lê um favorito por linha, com alternativas separadas por "|".
func lerFavoritos(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out [][]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		linha := strings.TrimSpace(sc.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		var alts []string
		for _, a := range strings.Split(linha, "|") {
			if a = strings.TrimSpace(a); a != "" {
				alts = append(alts, a)
			}
		}
		out = append(out, alts)
	}
	return out, sc.Err()
}

// escolherFavoritos fica, de cada linha, com a primeira alternativa
// instalada.
func escolherFavoritos(favs [][]string, apps map[string][]string) []string {
	var out []string
	for _, alts := range favs {
		for _, a := range alts {
			if _, ok := apps[a]; ok && !slices.Contains(out, a) {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

// ---- Apps instalados ----

// dirsAplicativos são as pastas de .desktop, da maior para a menor
// prioridade (a primeira ocorrência de um ID vale, como no GNOME).
func dirsAplicativos(home string) []string {
	return []string{
		filepath.Join(home, ".local/share/applications"),
		filepath.Join(home, ".local/share/flatpak/exports/share/applications"),
		"/var/lib/flatpak/exports/share/applications",
		"/usr/local/share/applications",
		"/usr/share/applications",
		"/var/lib/snapd/desktop/applications",
	}
}

// desktopsAtuais são os nomes que o GNOME usa em OnlyShowIn/NotShowIn (o
// Ubuntu acrescenta "ubuntu", como em XDG_CURRENT_DESKTOP=ubuntu:GNOME).
func desktopsAtuais(c *Ctx) []string {
	if c.Sys.ID == "ubuntu" {
		return []string{"ubuntu", "GNOME"}
	}
	return []string{"GNOME"}
}

// appsInstalados devolve os apps visíveis no menu (ID do .desktop ->
// categorias). O ID de um arquivo em subpasta segue a especificação:
// applications/kde/foo.desktop -> kde-foo.desktop.
func appsInstalados(dirs, desktops []string) map[string][]string {
	vistos := map[string]bool{}
	apps := map[string][]string{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".desktop") {
				return nil
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return nil
			}
			id := strings.ReplaceAll(rel, string(filepath.Separator), "-")
			if vistos[id] {
				return nil
			}
			vistos[id] = true
			if cats, visivel := lerDesktop(path, desktops); visivel {
				apps[id] = cats
			}
			return nil
		})
	}
	return apps
}

// lerDesktop lê o grupo [Desktop Entry] e diz se o app aparece no menu do
// GNOME (Type=Application, sem NoDisplay/Hidden, respeitando OnlyShowIn,
// NotShowIn e TryExec), devolvendo as categorias.
func lerDesktop(path string, desktops []string) (cats []string, visivel bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	kv := map[string]string{}
	dentro := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		linha := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(linha, "[") {
			if dentro {
				break
			}
			dentro = linha == "[Desktop Entry]"
			continue
		}
		if !dentro || linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		if k, v, ok := strings.Cut(linha, "="); ok {
			kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	lista := func(k string) []string {
		var out []string
		for _, s := range strings.Split(kv[k], ";") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	cruza := func(a, b []string) bool {
		return slices.ContainsFunc(a, func(s string) bool { return slices.Contains(b, s) })
	}
	switch {
	case kv["Type"] != "Application", kv["NoDisplay"] == "true", kv["Hidden"] == "true":
		return nil, false
	case kv["OnlyShowIn"] != "" && !cruza(lista("OnlyShowIn"), desktops):
		return nil, false
	case cruza(lista("NotShowIn"), desktops):
		return nil, false
	case kv["TryExec"] != "" && !executavel(kv["TryExec"]):
		return nil, false
	}
	return lista("Categories"), true
}

func executavel(prog string) bool {
	if filepath.IsAbs(prog) {
		info, err := os.Stat(prog)
		return err == nil && info.Mode()&0o111 != 0
	}
	_, err := exec.LookPath(prog)
	return err == nil
}

// ---- Organização ----

// organizacao é o resultado para uma pasta: os apps que ela vai mostrar
// (membros), os que precisam ser listados explicitamente em "apps" (os
// colocados à força, que não batem pelas categorias) e os que batem pelas
// categorias mas ficaram em outra pasta ("excluded-apps").
type organizacao struct {
	membros, apps, excluidos []string
}

// organizarPastas decide uma única pasta para cada app instalado: a que o
// lista em "apps" ou, senão, a primeira (na ordem do arquivo) cujas
// categorias ele declare. Como o GNOME mostra um app em toda pasta cujas
// categorias batam, os que ficaram em outra pasta entram no excluded-apps.
func organizarPastas(pastas []pasta, apps map[string][]string) []organizacao {
	org := make([]organizacao, len(pastas))
	destino := map[string]int{}
	ids := make([]string, 0, len(apps))
	for id := range apps {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	for _, id := range ids {
		destino[id] = -1
		for i, p := range pastas {
			if slices.Contains(p.Apps, id) {
				destino[id] = i
				break
			}
		}
		if destino[id] >= 0 {
			continue
		}
		for i, p := range pastas {
			if cruzaCategorias(p.Categorias, apps[id]) {
				destino[id] = i
				break
			}
		}
	}

	for _, id := range ids {
		d := destino[id]
		for i, p := range pastas {
			bate := cruzaCategorias(p.Categorias, apps[id])
			switch {
			case i == d:
				org[i].membros = append(org[i].membros, id)
				if !bate {
					org[i].apps = append(org[i].apps, id)
				}
			case bate:
				org[i].excluidos = append(org[i].excluidos, id)
			}
		}
	}
	return org
}

func cruzaCategorias(a, b []string) bool {
	return slices.ContainsFunc(a, func(s string) bool { return slices.Contains(b, s) })
}

// appsSoltos são os apps que não caíram em nenhuma pasta.
func appsSoltos(org []organizacao, apps map[string][]string) []string {
	var out []string
	for id := range apps {
		if !slices.ContainsFunc(org, func(o organizacao) bool { return slices.Contains(o.membros, id) }) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

var semAcento = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e",
	"í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c")

// idPasta gera o ID (nome da chave no dconf) a partir do nome:
// "Comunicação" -> "comunicacao".
func idPasta(nome string) string {
	var b strings.Builder
	for _, r := range semAcento.Replace(strings.ToLower(nome)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// gvStrv é strv com o tipo explícito quando vazia: no "dconf load" não há
// schema, e "[]" sozinho não diz que é uma lista de strings.
func gvStrv(items []string) string {
	if len(items) == 0 {
		return "@as []"
	}
	return strv(items)
}

// diretorioExiste diz se um arquivo .directory está instalado (o GNOME
// procura em desktop-directories dos diretórios de dados do XDG).
func diretorioExiste(nome string) bool {
	for _, d := range []string{"/usr/local/share/desktop-directories", "/usr/share/desktop-directories"} {
		if fileExists(filepath.Join(d, nome)) {
			return true
		}
	}
	return false
}

// dconfPastas monta o conteúdo para "dconf load /org/gnome/desktop/app-folders/".
// Todas as pastas são definidas, mesmo as vazias: o GNOME não mostra pasta
// vazia, e ela passa a aparecer quando um app da categoria for instalado.
// Com o .directory da pasta instalado, o nome é o dele (translate=true: o
// GNOME mostra a tradução do idioma do sistema); sem ele, o nome fixo — sem
// o arquivo, o GNOME mostraria "X.directory" literalmente.
func dconfPastas(pastas []pasta, org []organizacao, existe func(string) bool) string {
	ids := make([]string, len(pastas))
	for i, p := range pastas {
		ids[i] = idPasta(p.Nome)
	}
	var b strings.Builder
	b.WriteString("[/]\nfolder-children=" + gvStrv(ids) + "\n")
	for i, p := range pastas {
		b.WriteString("\n[folders/" + ids[i] + "]\n")
		if p.Diretorio != "" && existe(p.Diretorio) {
			b.WriteString("name=" + gvString(p.Diretorio) + "\ntranslate=true\n")
		} else {
			b.WriteString("name=" + gvString(p.Nome) + "\ntranslate=false\n")
		}
		b.WriteString("categories=" + gvStrv(p.Categorias) + "\n")
		b.WriteString("apps=" + gvStrv(org[i].apps) + "\n")
		b.WriteString("excluded-apps=" + gvStrv(org[i].excluidos) + "\n")
	}
	return b.String()
}
