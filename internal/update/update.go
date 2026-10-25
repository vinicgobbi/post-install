// Package update confere se há uma release mais nova do post-install no
// GitHub e, se o usuário quiser, substitui o binário (e, quando ele roda de
// dentro do pacote da release, também GNOME/, CINNAMON/, Wallpapers/ e os
// READMEs) pela versão nova. O processo atual continua com a versão antiga: é preciso
// executar de novo depois de atualizar.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	repo   = "vinicgobbi/post-install"
	pacote = "post-install-linux-amd64.tar.gz"
	// maxArquivo limita cada arquivo extraído (o maior hoje é um papel de
	// parede 4K de poucos MB), para um pacote adulterado não lotar o disco.
	maxArquivo = 256 << 20
)

// Release é uma versão nova disponível.
type Release struct {
	Tag    string
	url    string // .tar.gz da release
	sumURL string // .sha256 do .tar.gz
	digest string // SHA-256 calculado pelo GitHub no upload (pode faltar)
}

// semDowngrade recusa redirecionamentos de https:// para http://.
func semDowngrade(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("redirecionamentos demais")
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("redirecionamento inseguro para %s", req.URL)
	}
	return nil
}

var client = &http.Client{CheckRedirect: semDowngrade}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "post-install")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return resp, nil
}

// Check devolve a última release se ela for mais nova que atual, ou nil se
// não houver nada novo. Builds locais (versão "dev" ou fora do formato
// vX.Y.Z) não são conferidos.
func Check(ctx context.Context, atual string) (*Release, error) {
	if _, ok := parseVersao(atual); !ok {
		return nil, nil
	}
	resp, err := get(ctx, "https://api.github.com/repos/"+repo+"/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rel struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	if !Newer(rel.Tag, atual) {
		return nil, nil
	}
	r := &Release{Tag: rel.Tag}
	for _, a := range rel.Assets {
		switch a.Name {
		case pacote:
			r.url = a.URL
			r.digest, _ = strings.CutPrefix(a.Digest, "sha256:")
		case pacote + ".sha256":
			r.sumURL = a.URL
		}
	}
	if r.url == "" || r.sumURL == "" {
		return nil, fmt.Errorf("a release %s não tem %s e o .sha256 dele", rel.Tag, pacote)
	}
	return r, nil
}

// parseVersao lê "vX.Y.Z" (o "v" é opcional).
func parseVersao(v string) ([3]int, bool) {
	var out [3]int
	partes := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(partes) != 3 {
		return out, false
	}
	for i, p := range partes {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Newer diz se a versão a é maior que b (ambas no formato vX.Y.Z).
func Newer(a, b string) bool {
	va, ok1 := parseVersao(a)
	vb, ok2 := parseVersao(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

// Apply baixa a release, confere o SHA-256 e substitui o executável exe. Se
// exe estiver dentro da estrutura do pacote da release (com GNOME/ ao lado),
// atualiza também GNOME/, CINNAMON/, Wallpapers/, README.md e OVPN/README.md — os .ovpn
// e os logs nunca são tocados. Os arquivos novos ficam com o dono do
// executável atual (o programa roda como root, mas o pacote é do usuário).
func Apply(ctx context.Context, r *Release, exe string) error {
	base := filepath.Dir(exe)
	info, err := os.Stat(exe)
	if err != nil {
		return err
	}
	uid, gid := -1, -1
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		uid, gid = int(st.Uid), int(st.Gid)
	}
	completo := isDir(filepath.Join(base, "GNOME"))

	// Tudo é preparado num diretório temporário na mesma pasta (mesmo
	// sistema de arquivos, para o rename final ser atômico) e só então
	// movido para o lugar, com o binário por último.
	stage, err := os.MkdirTemp(base, ".post-install-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	tgz := filepath.Join(stage, pacote)
	if err := baixar(ctx, r.url, tgz); err != nil {
		return err
	}
	if err := conferir(ctx, r, tgz); err != nil {
		return err
	}
	arquivos, err := extrair(tgz, filepath.Join(stage, "pacote"), completo)
	if err != nil {
		return err
	}
	if _, ok := arquivos["post-install"]; !ok {
		return errors.New("o pacote da release não contém o binário")
	}

	for rel, origem := range arquivos {
		if rel == "post-install" {
			continue
		}
		destino := filepath.Join(base, filepath.FromSlash(rel))
		if err := criarDirs(base, filepath.Dir(destino), uid, gid); err != nil {
			return err
		}
		if err := mover(origem, destino, 0o644, uid, gid); err != nil {
			return err
		}
	}
	return mover(arquivos["post-install"], exe, 0o755, uid, gid)
}

func baixar(ctx context.Context, url, dest string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	resp, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 1<<30)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// conferir compara o SHA-256 do pacote com o .sha256 publicado na release e
// com o digest que o GitHub calculou no upload.
func conferir(ctx context.Context, r *Release, tgz string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := get(ctx, r.sumURL)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	if err != nil {
		return err
	}
	campos := strings.Fields(string(body))
	if len(campos) == 0 {
		return errors.New("arquivo .sha256 da release vazio")
	}

	f, err := os.Open(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	for _, want := range []string{campos[0], r.digest} {
		if want != "" && !strings.EqualFold(got, want) {
			return fmt.Errorf("SHA-256 do pacote (%s) não confere com o da release (%s)", got, want)
		}
	}
	return nil
}

// permitido diz se um caminho do pacote (relativo a "post-install/") deve
// ser atualizado.
func permitido(rel string, completo bool) bool {
	if rel == "post-install" {
		return true
	}
	if !completo {
		return false
	}
	return rel == "README.md" || rel == "OVPN/README.md" ||
		strings.HasPrefix(rel, "GNOME/") || strings.HasPrefix(rel, "CINNAMON/") || strings.HasPrefix(rel, "Wallpapers/")
}

// extrair descompacta os arquivos permitidos do pacote em dir e devolve
// caminho relativo -> arquivo extraído. Só arquivos regulares; links,
// caminhos absolutos ou com ".." são ignorados.
func extrair(tgz, dir string, completo bool) (map[string]string, error) {
	f, err := os.Open(tgz)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	out := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		rel, ok := strings.CutPrefix(path.Clean(h.Name), "post-install/")
		if !ok || rel == "" || strings.HasPrefix(rel, "../") || path.IsAbs(rel) || !permitido(rel, completo) {
			continue
		}
		if h.Size > maxArquivo {
			return nil, fmt.Errorf("%s no pacote é grande demais (%d bytes)", rel, h.Size)
		}
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		w, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(w, io.LimitReader(tr, maxArquivo)); err != nil {
			w.Close()
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		out[rel] = dest
	}
}

// criarDirs cria as pastas que faltarem de base até dir, com o dono
// uid/gid (as que já existem ficam como estão).
func criarDirs(base, dir string, uid, gid int) error {
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == "." {
		return err
	}
	atual := base
	for _, parte := range strings.Split(rel, string(filepath.Separator)) {
		atual = filepath.Join(atual, parte)
		if err := os.Mkdir(atual, 0o755); errors.Is(err, os.ErrExist) {
			continue
		} else if err != nil {
			return err
		}
		if uid >= 0 {
			if err := os.Chown(atual, uid, gid); err != nil {
				return err
			}
		}
	}
	return nil
}

// mover põe origem no lugar de destino (rename atômico), com a permissão e
// o dono uid/gid dados.
func mover(origem, destino string, perm os.FileMode, uid, gid int) error {
	if err := os.Chmod(origem, perm); err != nil {
		return err
	}
	if uid >= 0 {
		if err := os.Chown(origem, uid, gid); err != nil {
			return err
		}
	}
	return os.Rename(origem, destino)
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
