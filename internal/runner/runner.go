// Package runner executa comandos externos, downloads e escritas de arquivo
// em nome dos módulos, mandando toda a saída para o log.
//
// Em modo DryRun, tudo que altera o sistema (comandos "Run"/"Try", escrita de
// arquivos, downloads) só é registrado no log; consultas somente-leitura
// (Output/Probe/requisições HTTP) continuam sendo executadas de verdade, para
// o fluxo dos módulos seguir o mesmo caminho que seguiria numa execução real.
package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vinicgobbi/post-install/internal/runlog"
)

type Runner struct {
	Ctx    context.Context
	Log    *runlog.Logger
	User   string // usuário alvo (para AsUser)
	DryRun bool
	// WorkDir é um diretório temporário exclusivo desta execução, removido
	// pelo módulo de limpeza.
	WorkDir string
}

// baseEnv é aplicado a todo comando: execução sem prompts do apt.
var baseEnv = []string{"DEBIAN_FRONTEND=noninteractive"}

func (r *Runner) command(env []string, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(r.Ctx, name, args...)
	cmd.Env = append(append(os.Environ(), baseEnv...), env...)
	// Grupo de processos próprio: no Ctrl+C mata também os filhos (ex.: o
	// "sh" de um instalador e o curl que ele disparou).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// describe monta a linha "$ comando args" mostrada no log. Scripts de várias
// linhas (bash -c / su -c) viram uma linha só, separada por "; ".
func describe(name string, args []string) string {
	parts := []string{name}
	for _, a := range args {
		if strings.Contains(a, "\n") {
			var linhas []string
			for _, l := range strings.Split(a, "\n") {
				if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
					linhas = append(linhas, l)
				}
			}
			a = "'" + strings.Join(linhas, "; ") + "'"
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// Run executa um comando que altera o sistema, transmitindo a saída para o
// log. Falha se o comando sair com código diferente de zero.
func (r *Runner) Run(name string, args ...string) error {
	return r.RunEnv(nil, name, args...)
}

// RunEnv é Run com variáveis de ambiente extras (ex.: ACCEPT_EULA=Y).
func (r *Runner) RunEnv(env []string, name string, args ...string) error {
	desc := describe(name, args)
	if len(env) > 0 {
		desc = strings.Join(env, " ") + " " + desc
	}
	if r.DryRun {
		r.Log.Infof("[dry-run] %s", desc)
		return nil
	}
	w := r.Log.OutputWriter()
	defer w.Close()
	fmt.Fprintf(w, "$ %s\n", desc)

	cmd := r.command(env, name, args...)
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		if r.Ctx.Err() != nil {
			return r.Ctx.Err()
		}
		return fmt.Errorf("%s: %w", desc, err)
	}
	return nil
}

// Try executa um comando que altera o sistema sem mostrar a saída na tela
// (só no .raw.log) e devolve se deu certo. Equivale a "cmd &>/dev/null".
func (r *Runner) Try(name string, args ...string) bool {
	if r.DryRun {
		r.Log.Infof("[dry-run] %s", describe(name, args))
		return true
	}
	return r.quiet(name, args...) == nil
}

// Probe executa uma consulta somente-leitura (ex.: "dpkg -s pacote") e devolve
// se ela teve sucesso. Roda mesmo em DryRun.
func (r *Runner) Probe(name string, args ...string) bool {
	return r.quiet(name, args...) == nil
}

func (r *Runner) quiet(name string, args ...string) error {
	cmd := r.command(nil, name, args...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	r.Log.RawWrite(fmt.Sprintf("$ %s\n%s", describe(name, args), buf.String()))
	return err
}

// Output executa uma consulta somente-leitura e devolve o stdout. Roda mesmo
// em DryRun.
func (r *Runner) Output(name string, args ...string) (string, error) {
	cmd := r.command(nil, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	r.Log.RawWrite(fmt.Sprintf("$ %s\n%s%s", describe(name, args), out, stderr.String()))
	if err != nil {
		return string(out), fmt.Errorf("%s: %w", describe(name, args), err)
	}
	return string(out), nil
}

// Shell executa um trecho de shell como root (bash -c).
func (r *Runner) Shell(script string) error {
	return r.Run("bash", "-c", script)
}

// userSession aponta o script para a sessão gráfica do usuário, se ela
// existir. O "su -" não traz XDG_RUNTIME_DIR nem o D-Bus da sessão, e sem
// eles o gsettings grava num backend em memória e descarta tudo sem erro.
const userSession = `if [ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ] && [ -S "/run/user/$(id -u)/bus" ]; then
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
fi
`

// AsUser executa um trecho de shell como o usuário alvo, em login shell (com
// $HOME e PATH corretos e, se houver, a sessão D-Bus dele).
func (r *Runner) AsUser(script string) error {
	if r.User == "" {
		return errors.New("usuário alvo não definido")
	}
	return r.Run("su", "-", r.User, "-c", userSession+script)
}

// OutputAsUser executa uma consulta somente-leitura como o usuário alvo (com
// a sessão D-Bus dele) e devolve o stdout. Roda mesmo em DryRun.
func (r *Runner) OutputAsUser(script string) (string, error) {
	if r.User == "" {
		return "", errors.New("usuário alvo não definido")
	}
	return r.Output("su", "-", r.User, "-c", userSession+script)
}

// Exists informa se um binário está no PATH.
func Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ---- Arquivos ----

func (r *Runner) WriteFile(path, content string, perm os.FileMode) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] escrever %s (%d bytes)", path, len(content))
		return nil
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

func (r *Runner) AppendFile(path, content string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] acrescentar a %s: %q", path, content)
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func (r *Runner) MkdirAll(path string, perm os.FileMode) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] mkdir -p %s", path)
		return nil
	}
	return os.MkdirAll(path, perm)
}

func (r *Runner) Chmod(path string, perm os.FileMode) error {
	if r.DryRun {
		return nil
	}
	return os.Chmod(path, perm)
}

// Symlink equivale a "ln -sf alvo link".
func (r *Runner) Symlink(target, link string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] ln -sf %s %s", target, link)
		return nil
	}
	if err := os.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(target, link)
}

func (r *Runner) RemoveAll(path string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] rm -rf %s", path)
		return nil
	}
	return os.RemoveAll(path)
}

// ---- Rede ----

// semDowngrade recusa redirecionamentos de https:// para http://, que
// tirariam a proteção do TLS no meio do caminho.
func semDowngrade(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("redirecionamentos demais")
	}
	if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf("redirecionamento inseguro de %s para %s", via[0].URL, req.URL)
	}
	return nil
}

var (
	httpClient = &http.Client{Timeout: 10 * time.Minute, CheckRedirect: semDowngrade}
	// getClient não tem timeout próprio: cada Get define o seu pelo contexto.
	getClient = &http.Client{CheckRedirect: semDowngrade}
)

// maxGet limita o corpo lido por Get (APIs, listas, páginas HTML): nada
// legítimo aqui chega perto disso.
const maxGet = 32 << 20

// Get faz uma requisição GET somente-leitura com timeout e devolve o corpo.
// Falha em status HTTP de erro (equivale a "curl -f").
func (r *Runner) Get(url string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(r.Ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "post-install")
	resp, err := getClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxGet+1))
	if err == nil && len(body) > maxGet {
		return nil, fmt.Errorf("GET %s: resposta maior que %d MiB", url, maxGet>>20)
	}
	return body, err
}

// Download baixa uma URL para dest com algumas tentativas antes de desistir,
// para tolerar instabilidade de rede.
func (r *Runner) Download(url, dest string) error {
	if r.DryRun {
		r.Log.Infof("[dry-run] baixar %s -> %s", url, dest)
		return nil
	}
	const tentativas = 3
	var err error
	for i := 1; i <= tentativas; i++ {
		if err = r.downloadOnce(url, dest); err == nil {
			return nil
		}
		if r.Ctx.Err() != nil {
			return r.Ctx.Err()
		}
		r.Log.Warnf("Falha ao baixar %s (tentativa %d/%d): %v", url, i, tentativas, err)
		time.Sleep(time.Duration(i) * time.Second)
	}
	return fmt.Errorf("não foi possível baixar %s após %d tentativas: %w", url, tentativas, err)
}

func (r *Runner) downloadOnce(url, dest string) error {
	req, err := http.NewRequestWithContext(r.Ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "post-install")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	r.Log.RawWrite(fmt.Sprintf("download %s -> %s\n", url, dest))
	return os.Rename(tmp, dest)
}

// DownloadVerified é Download conferindo o SHA-256 (hex) do arquivo baixado.
// Em DryRun só registra, como Download.
func (r *Runner) DownloadVerified(url, dest, sha string) error {
	if err := r.Download(url, dest); err != nil || r.DryRun {
		return err
	}
	if err := VerifySHA256(dest, sha); err != nil {
		os.Remove(dest)
		return err
	}
	return nil
}

// VerifySHA256 compara o SHA-256 de um arquivo com o esperado (hex).
func VerifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("SHA-256 de %s (%s) não confere com o esperado (%s)", filepath.Base(path), got, want)
	}
	return nil
}

// VerifyKey confere se todas as chaves primárias de um arquivo de chave
// OpenPGP (armored ou binário) estão entre os fingerprints esperados. O gpg
// roda com um --homedir temporário: como root numa instalação nova, o
// /root/.gnupg não existe. Em DryRun (o arquivo não foi baixado) não faz nada.
func (r *Runner) VerifyKey(path string, fingerprints ...string) error {
	if r.DryRun {
		return nil
	}
	home, err := os.MkdirTemp(r.WorkDir, "gnupg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	out, err := r.Output("gpg", "--homedir", home, "--batch", "--with-colons", "--show-keys", path)
	if err != nil {
		return err
	}
	primarias := primaryFingerprints(out)
	if len(primarias) == 0 {
		return fmt.Errorf("nenhuma chave encontrada em %s", filepath.Base(path))
	}
	for _, fp := range primarias {
		ok := false
		for _, want := range fingerprints {
			ok = ok || strings.EqualFold(fp, want)
		}
		if !ok {
			return fmt.Errorf("chave %s em %s não é nenhuma das esperadas (%s)", fp, filepath.Base(path), strings.Join(fingerprints, ", "))
		}
	}
	r.Log.Infof("Fingerprint da chave %s confere.", filepath.Base(path))
	return nil
}

// primaryFingerprints extrai, da saída "--with-colons" do gpg, o fingerprint
// de cada chave primária (o registro "fpr" logo após um "pub").
func primaryFingerprints(colons string) []string {
	var out []string
	aposPub := false
	for _, l := range strings.Split(colons, "\n") {
		campos := strings.Split(l, ":")
		switch campos[0] {
		case "pub":
			aposPub = true
		case "fpr":
			if aposPub && len(campos) > 9 {
				out = append(out, campos[9])
			}
			aposPub = false
		}
	}
	return out
}

// Temp devolve um caminho dentro do WorkDir.
func (r *Runner) Temp(name string) string {
	return filepath.Join(r.WorkDir, name)
}
