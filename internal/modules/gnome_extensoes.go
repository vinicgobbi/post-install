package modules

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vinicgobbi/post-install/internal/runner"
)

var extensoesGnome = Module{
	ID:    "extensoes_gnome",
	Title: "Extensões do GNOME",
	Desc:  "Instala (extensions.gnome.org ou repositório git), ativa e configura as extensões de GNOME/extensoes.txt e GNOME/extensoes.dconf",
	Run: func(c *Ctx) error {
		if c.Sys.IsKDE() {
			c.Log.Infof("Ambiente Plasma detectado: pulando extensões do GNOME.")
			return nil
		}
		if !runner.Exists("gnome-extensions") {
			c.Log.Warnf("gnome-extensions não encontrado (GNOME Shell não instalado?): pulando extensões.")
			return nil
		}
		lista := filepath.Join(c.GNOMEDir, "extensoes.txt")
		ativar, desativar, git, err := lerListaExtensoes(lista)
		if errors.Is(err, os.ErrNotExist) {
			c.Log.Warnf("%s não encontrado: pulando extensões.", lista)
			return nil
		}
		if err != nil {
			return err
		}

		versao, err := versaoGnomeShell(c)
		if err != nil {
			return err
		}
		c.Log.Infof("Instalando extensões do GNOME (GNOME Shell %s)...", versao)

		get := func(u string) ([]byte, error) { return c.Get(u, 30*time.Second) }
		var instaladas []string
		forcadas := false
		for _, uuid := range ativar {
			if extensaoDoSistema(uuid) {
				instaladas = append(instaladas, uuid)
				continue
			}
			if src, ok := git[uuid]; ok {
				if src.Ref == "" {
					c.Log.Warnf("%s: sem commit fixado em extensoes.txt — instalando o que estiver na branch principal de %s.", uuid, src.Repo)
				}
				if err := instalarExtensaoGit(c, src); err != nil {
					c.Log.Warnf("%s: %v", uuid, err)
					continue
				}
				instaladas = append(instaladas, uuid)
				c.Log.Successf("Extensão %s instalada de %s.", uuid, src.Repo)
				continue
			}
			info, err := ConsultarExtensao(get, uuid, versao)
			if err != nil {
				c.Log.Warnf("%s: %v", uuid, err)
				continue
			}
			if !info.Compativel {
				if !slices.Contains(c.ForcarExtensoes, uuid) {
					c.Log.Warnf("%s não tem versão para o GNOME Shell %s (última suportada: %s): não será instalada automaticamente.", uuid, versao, info.MaiorShell)
					continue
				}
				c.Log.Warnf("Instalando %s mesmo assim (feita para o GNOME Shell %s): pode não funcionar corretamente.", uuid, info.MaiorShell)
				forcadas = true
			}
			if err := instalarExtensao(c, info); err != nil {
				c.Log.Warnf("%s: %v", uuid, err)
				continue
			}
			instaladas = append(instaladas, uuid)
			c.Log.Successf("Extensão %s instalada.", uuid)
		}

		// Ativação via gsettings: no Wayland, extensões recém-instaladas só
		// são carregadas no próximo login, e o "gnome-extensions enable"
		// falharia antes disso.
		atuaisAtivas, _ := strvDoUsuario(c, "enabled-extensions")
		atuaisDesativadas, _ := strvDoUsuario(c, "disabled-extensions")
		novasAtivas := mesclar(atuaisAtivas, instaladas, desativar)
		novasDesativadas := mesclar(atuaisDesativadas, desativar, instaladas)

		script := "set -e\n" +
			"gsettings set org.gnome.shell disable-user-extensions false\n" +
			"gsettings set org.gnome.shell enabled-extensions " + shQuote(strv(novasAtivas)) + "\n" +
			"gsettings set org.gnome.shell disabled-extensions " + shQuote(strv(novasDesativadas)) + "\n"
		if forcadas {
			// Sem isso o GNOME recusa carregar extensões que não declaram a
			// versão atual. Vale para todas as extensões do usuário.
			c.Log.Warnf("Desativando a checagem de versão das extensões (disable-extension-version-validation) para carregar as instaladas mesmo assim. Isso vale para todas as extensões, inclusive as instaladas no futuro; para reverter: gsettings reset org.gnome.shell disable-extension-version-validation")
			script += "gsettings set org.gnome.shell disable-extension-version-validation true\n"
		}
		if conf := filepath.Join(c.GNOMEDir, "extensoes.dconf"); fileExists(conf) {
			script += "dconf load /org/gnome/shell/extensions/ < " + shQuote(conf) + "\n"
		}
		if err := c.AsUser(script); err != nil {
			return err
		}
		c.Log.Successf("%d extensões ativadas e configuradas (as novas carregam no próximo login).", len(instaladas))
		return nil
	},
}

// extGit é a origem de uma extensão instalada de um repositório git. Ref é o
// commit (ou tag) a usar; vazio = branch principal.
type extGit struct {
	Repo string
	Ref  string
}

// lerListaExtensoes lê o arquivo de UUIDs: uma por linha, "#" comenta, o
// prefixo "-" marca extensões a manter desativadas e uma URL depois do UUID
// (opcionalmente seguida de um commit/tag) indica o repositório git de onde
// instalar (devolvido em git, por UUID).
func lerListaExtensoes(path string) (ativar, desativar []string, git map[string]extGit, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		linha := strings.TrimSpace(sc.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		if uuid, ok := strings.CutPrefix(linha, "-"); ok {
			desativar = append(desativar, strings.TrimSpace(uuid))
			continue
		}
		campos := strings.Fields(linha)
		ativar = append(ativar, campos[0])
		if len(campos) > 1 {
			if git == nil {
				git = map[string]extGit{}
			}
			src := extGit{Repo: campos[1]}
			if len(campos) > 2 {
				src.Ref = campos[2]
			}
			git[campos[0]] = src
		}
	}
	return ativar, desativar, git, sc.Err()
}

// versaoGnomeShell devolve a versão maior do GNOME Shell ("50"), que é o que
// o extensions.gnome.org usa para escolher o pacote compatível.
func versaoGnomeShell(c *Ctx) (string, error) {
	out, err := c.Output("gnome-shell", "--version")
	if err != nil {
		return "", err
	}
	if maior := majorDaVersao(out); maior != "" {
		return maior, nil
	}
	return "", errorf("versão do GNOME Shell não reconhecida: %q", out)
}

// majorDaVersao: "GNOME Shell 50.1" -> "50".
func majorDaVersao(out string) string {
	campos := strings.Fields(out)
	if len(campos) == 0 {
		return ""
	}
	maior, _, _ := strings.Cut(campos[len(campos)-1], ".")
	return maior
}

// extensaoDoSistema: extensões que vêm com a distro (ex.: ubuntu-dock) não
// são baixadas, só ativadas.
func extensaoDoSistema(uuid string) bool {
	return fileExists(filepath.Join("/usr/share/gnome-shell/extensions", uuid))
}

// ExtInfo é o que o extensions.gnome.org diz sobre uma extensão para a
// versão do GNOME Shell da máquina.
type ExtInfo struct {
	UUID       string
	Compativel bool   // há versão publicada para o GNOME Shell da máquina
	MaiorShell string // maior versão do GNOME Shell suportada
	versionTag int    // pacote a baixar: o compatível ou, se não houver, o mais recente
}

// ConsultarExtensao decide a compatibilidade pelo shell_version_map: a API
// com shell_version não dá erro quando não há versão compatível, ela
// devolve a mais recente (que o GNOME não carregaria).
func ConsultarExtensao(get func(string) ([]byte, error), uuid, shell string) (ExtInfo, error) {
	body, err := get("https://extensions.gnome.org/extension-info/?" + url.Values{"uuid": {uuid}}.Encode())
	if err != nil {
		return ExtInfo{}, errorf("não encontrada no extensions.gnome.org (%v)", err)
	}
	var d struct {
		Mapa map[string]struct {
			PK int `json:"pk"`
		} `json:"shell_version_map"`
	}
	if err := json.Unmarshal(semControle(body), &d); err != nil {
		return ExtInfo{}, err
	}
	if len(d.Mapa) == 0 {
		return ExtInfo{}, errorf("nenhuma versão publicada no extensions.gnome.org")
	}
	info := ExtInfo{UUID: uuid}
	for v := range d.Mapa {
		if info.MaiorShell == "" || versaoMaior(v, info.MaiorShell) {
			info.MaiorShell = v
		}
	}
	if v, ok := d.Mapa[shell]; ok {
		info.Compativel, info.versionTag = true, v.PK
	} else {
		info.versionTag = d.Mapa[info.MaiorShell].PK
	}
	return info, nil
}

// versaoMaior compara versões como "3.38", "45" e "50" numericamente.
func versaoMaior(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func instalarExtensao(c *Ctx, info ExtInfo) error {
	zip := c.Temp(info.UUID + ".zip")
	u := fmt.Sprintf("https://extensions.gnome.org/download-extension/%s.shell-extension.zip?version_tag=%d", url.PathEscape(info.UUID), info.versionTag)
	if err := c.Download(u, zip); err != nil {
		return err
	}
	return c.AsUser("gnome-extensions install --force " + shQuote(zip))
}

// instalarExtensaoGit baixa o commit fixado (ou a branch principal) do
// repositório e roda o "make install" dele, que copia a extensão para
// ~/.local/share/gnome-shell/extensions.
func instalarExtensaoGit(c *Ctx, src extGit) error {
	if !runner.Exists("make") {
		if err := c.install("make"); err != nil {
			return err
		}
	}
	ref := src.Ref
	if ref == "" {
		ref = "HEAD"
	}
	return c.AsUser(`set -e
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
git init --quiet "$dir"
git -C "$dir" fetch --quiet --depth 1 ` + shQuote(src.Repo) + ` ` + shQuote(ref) + `
git -C "$dir" -c advice.detachedHead=false checkout --quiet FETCH_HEAD
make -C "$dir" install
`)
}

// VerificarExtensoes roda antes da execução (sem root, sem o Ctx) para a
// interface perguntar sobre as extensões sem versão compatível. Devolve a
// versão do GNOME Shell e as incompatíveis; sem GNOME ou sem a lista, nada.
func VerificarExtensoes(gnomeDir string) (string, []ExtInfo) {
	ativar, _, git, err := lerListaExtensoes(filepath.Join(gnomeDir, "extensoes.txt"))
	if err != nil || !runner.Exists("gnome-shell") {
		return "", nil
	}
	out, err := exec.Command("gnome-shell", "--version").Output()
	if err != nil {
		return "", nil
	}
	shell := majorDaVersao(string(out))

	client := &http.Client{Timeout: 15 * time.Second}
	get := func(u string) ([]byte, error) {
		resp, err := client.Get(u)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		incompt []ExtInfo
	)
	for _, uuid := range ativar {
		if _, ok := git[uuid]; ok || extensaoDoSistema(uuid) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if info, err := ConsultarExtensao(get, uuid, shell); err == nil && !info.Compativel {
				mu.Lock()
				incompt = append(incompt, info)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	slices.SortFunc(incompt, func(a, b ExtInfo) int { return strings.Compare(a.UUID, b.UUID) })
	return shell, incompt
}

// semControle troca caracteres de controle crus por espaço: o
// extensions.gnome.org às vezes os devolve dentro das descrições, o que é
// JSON inválido para o encoding/json.
func semControle(b []byte) []byte {
	out := make([]byte, len(b))
	for i, ch := range b {
		if ch < 0x20 {
			ch = ' '
		}
		out[i] = ch
	}
	return out
}

// strvDoUsuario lê uma lista (as) de org.gnome.shell como o usuário alvo.
func strvDoUsuario(c *Ctx, chave string) ([]string, error) {
	out, err := c.OutputAsUser("gsettings get org.gnome.shell " + chave)
	if err != nil {
		return nil, err
	}
	return parseStrv(out), nil
}

// parseStrv interpreta a saída do gsettings para listas de strings, como
// "['a', 'b']" ou "@as []".
func parseStrv(s string) []string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "@as"))
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.Trim(strings.TrimSpace(item), `'"`); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// strv monta uma lista GVariant de strings, escapando "\" e "'" (sintaxe
// de texto do GVariant), para um item estranho não quebrar o gsettings.
func strv(items []string) string {
	esc := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = "'" + esc.Replace(it) + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// mesclar devolve base + incluir (sem repetir), sem os itens de remover.
func mesclar(base, incluir, remover []string) []string {
	var out []string
	for _, it := range slices.Concat(base, incluir) {
		if !slices.Contains(out, it) && !slices.Contains(remover, it) {
			out = append(out, it)
		}
	}
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
