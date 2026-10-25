package modules

import (
	"bufio"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

var mirrors = Module{
	ID:    "mirrors",
	Grupo: GrupoSistema,
	Title: "Otimizar mirrors",
	Desc:  "Aponta o APT para o mirror do Ubuntu mais rápido medido nesta rede",
	Run: func(c *Ctx) error {
		c.Log.Infof("Otimizando a seleção de espelhos (mirrors) do APT...")
		return otimizarMirrorsUbuntu(c)
	},
}

func otimizarMirrorsUbuntu(c *Ctx) error {
	mirror, ok := mirrorUbuntuMaisRapido(c)
	if !ok {
		c.Log.Warnf("Não foi possível testar a velocidade dos mirrors (sem rede?). Mantendo o redirecionador automático (mirror://) do Ubuntu.")
		mirror = "mirror://mirrors.ubuntu.com/mirrors.txt"
	} else {
		c.Log.Successf("Mirror mais rápido nos testes: %s", mirror)
	}

	repl := strings.NewReplacer(
		"http://archive.ubuntu.com/ubuntu/", mirror,
		"http://security.ubuntu.com/ubuntu/", mirror,
		"mirror://mirrors.ubuntu.com/mirrors.txt", mirror,
	)
	for _, arquivo := range []string{"/etc/apt/sources.list", "/etc/apt/sources.list.d/ubuntu.sources"} {
		info, err := os.Stat(arquivo)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(arquivo)
		if err != nil {
			return err
		}
		if err := c.WriteFile(arquivo, repl.Replace(string(data)), info.Mode().Perm()); err != nil {
			return err
		}
	}
	c.Log.Successf("APT otimizado: repositórios apontando para o mirror com melhor tempo de resposta medido agora.")
	return nil
}

var (
	reHrefUbuntu = regexp.MustCompile(`href="(https?://[^"]+/ubuntu/)"`)
	reRSSUbuntu  = regexp.MustCompile(`<link>(https?://[^<]+/ubuntu/)</link>`)
)

// candidatosMirrorUbuntu monta a lista de candidatos a testar:
//  1. o pick do GeoIP da Canonical (http:// simples: o https:// desse host
//     não responde). Devolve UMA linha só — serve de baseline. Um atacante na
//     rede pode trocar a resposta, mas no máximo escolhe o mirror: o apt
//     confere a assinatura do Release de qualquer mirror.
//  2. os mirrors do próprio país do IP público (país consultado por HTTPS no
//     ifconfig.co, no formato em inglês que o Launchpad usa), extraídos da página de
//     mirrors do Launchpad (organizada em seções por país). É essencial: uma
//     amostra aleatória global tem grande chance de nunca incluir o mirror
//     local de fato mais rápido.
//  3. uma amostra aleatória global do feed RSS do Launchpad, como
//     complemento caso a seção do país não exista ou venha vazia.
func candidatosMirrorUbuntu(c *Ctx) []string {
	var cands []string

	if body, err := c.Get("http://mirrors.ubuntu.com/mirrors.txt", 5*time.Second); err == nil {
		cands = append(cands, strings.Fields(string(body))...)
	}

	if body, err := c.Get("https://ifconfig.co/country", 5*time.Second); err == nil {
		if pais := strings.TrimSpace(string(body)); pais != "" {
			if page, err := c.Get("https://launchpad.net/ubuntu/+archivemirrors", 12*time.Second); err == nil {
				cands = append(cands, mirrorsDoPais(string(page), pais)...)
			}
		}
	}

	if rss, err := c.Get("https://launchpad.net/ubuntu/+archivemirrors-rss", 12*time.Second); err == nil {
		var globais []string
		for _, m := range reRSSUbuntu.FindAllStringSubmatch(string(rss), -1) {
			globais = append(globais, m[1])
		}
		slices.Sort(globais)
		globais = slices.Compact(globais)
		rand.Shuffle(len(globais), func(i, j int) { globais[i], globais[j] = globais[j], globais[i] })
		cands = append(cands, globais[:min(8, len(globais))]...)
	}

	slices.Sort(cands)
	return slices.Compact(cands)
}

// mirrorsDoPais extrai os mirrors da seção de um país na página
// +archivemirrors: do cabeçalho <th colspan="2">País</th> até a próxima
// linha <tr class="head">.
func mirrorsDoPais(page, pais string) []string {
	var bloco strings.Builder
	dentro := false
	sc := bufio.NewScanner(strings.NewReader(page))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		linha := sc.Text()
		if !dentro {
			if strings.Contains(linha, `<th colspan="2">`+pais+`</th>`) {
				dentro = true
			}
			continue
		}
		if strings.Contains(linha, `<tr class="head">`) {
			break
		}
		bloco.WriteString(linha + "\n")
	}
	var out []string
	for _, m := range reHrefUbuntu.FindAllStringSubmatch(bloco.String(), -1) {
		out = append(out, m[1])
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// mirrorUbuntuMaisRapido testa o tempo de download do Release de cada
// candidato em paralelo e fica com o menor tempo medido de verdade nesta
// rede, em vez de confiar em GeoIP de terceiros.
func mirrorUbuntuMaisRapido(c *Ctx) (string, bool) {
	const timeoutCandidato = 4 * time.Second
	var (
		mu     sync.Mutex
		melhor string
		menor  time.Duration
		wg     sync.WaitGroup
	)
	client := &http.Client{Timeout: timeoutCandidato}

	for _, cand := range candidatosMirrorUbuntu(c) {
		wg.Add(1)
		go func(cand string) {
			defer wg.Done()
			url := strings.TrimSuffix(cand, "/") + "/dists/" + c.Sys.Codename + "/Release"
			req, err := http.NewRequestWithContext(c.Ctx, http.MethodGet, url, nil)
			if err != nil {
				return
			}
			inicio := time.Now()
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				return
			}
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				return
			}
			d := time.Since(inicio)
			mu.Lock()
			if melhor == "" || d < menor {
				melhor, menor = cand, d
			}
			mu.Unlock()
		}(cand)
	}
	wg.Wait()
	return melhor, melhor != ""
}
