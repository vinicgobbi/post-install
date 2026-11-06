package runner

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Host é por onde o runner consulta a máquina e a rede: tudo o que roda
// mesmo no dry-run (comandos de consulta, binários no PATH, leitura de
// arquivos e requisições HTTP). Com Runner.Host nil, as consultas vão para a
// máquina real; os testes injetam um Host falso para que o --dry-run de
// cada módulo seja reproduzível.
type Host interface {
	// Consultar executa um comando somente-leitura e devolve stdout e
	// stderr; err não nil equivale a código de saída diferente de zero.
	Consultar(name string, args []string) (stdout, stderr []byte, err error)
	LookPath(name string) (string, error)
	// FS é a raiz "/" do sistema (caminhos sem a barra inicial, como no
	// os.DirFS("/")).
	FS() fs.FS
	Transport() http.RoundTripper
}

// LookPath procura um binário no PATH.
func (r *Runner) LookPath(name string) (string, error) {
	if r.Host != nil {
		return r.Host.LookPath(name)
	}
	return exec.LookPath(name)
}

// Exists informa se um binário está no PATH (veja também a função Exists,
// que sempre consulta a máquina real).
func (r *Runner) Exists(name string) bool {
	_, err := r.LookPath(name)
	return err == nil
}

// ReadFile lê um arquivo da máquina.
func (r *Runner) ReadFile(path string) ([]byte, error) {
	if r.Host != nil {
		return fs.ReadFile(r.Host.FS(), caminhoFS(path))
	}
	return os.ReadFile(path)
}

// Stat devolve as informações de um arquivo da máquina.
func (r *Runner) Stat(path string) (fs.FileInfo, error) {
	if r.Host != nil {
		return fs.Stat(r.Host.FS(), caminhoFS(path))
	}
	return os.Stat(path)
}

// FileExists informa se o caminho existe.
func (r *Runner) FileExists(path string) bool {
	_, err := r.Stat(path)
	return err == nil
}

// Glob equivale a filepath.Glob.
func (r *Runner) Glob(pattern string) ([]string, error) {
	if r.Host == nil {
		return filepath.Glob(pattern)
	}
	achados, err := fs.Glob(r.Host.FS(), caminhoFS(pattern))
	for i, a := range achados {
		achados[i] = "/" + a
	}
	return achados, err
}

// WalkDir equivale a filepath.WalkDir.
func (r *Runner) WalkDir(root string, fn fs.WalkDirFunc) error {
	if r.Host == nil {
		return filepath.WalkDir(root, fn)
	}
	return fs.WalkDir(r.Host.FS(), caminhoFS(root), func(p string, d fs.DirEntry, err error) error {
		return fn("/"+p, d, err)
	})
}

// HTTPClient devolve um cliente HTTP com o timeout dado, para quem precisa
// medir a requisição (ex.: o teste de velocidade dos mirrors).
func (r *Runner) HTTPClient(timeout time.Duration) *http.Client {
	c := &http.Client{Timeout: timeout}
	if r.Host != nil {
		c.Transport = r.Host.Transport()
	}
	return c
}

// consultarHost é o Probe/Output via Host, com o mesmo registro no .raw.log
// da execução real.
func (r *Runner) consultarHost(name string, args []string) (stdout []byte, err error) {
	out, errOut, err := r.Host.Consultar(name, args)
	r.Log.RawWrite(fmt.Sprintf("$ %s\n%s%s", describe(name, args), out, errOut))
	if err != nil {
		return out, fmt.Errorf("%s: %w", describe(name, args), err)
	}
	return out, nil
}

// caminhoFS converte um caminho do sistema (absoluto ou relativo ao
// diretório atual) para o formato do fs.FS da raiz: sem a barra inicial.
func caminhoFS(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean("/" + path)
	}
	if p := strings.TrimPrefix(abs, "/"); p != "" {
		return p
	}
	return "."
}
