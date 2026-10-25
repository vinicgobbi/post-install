package modules

// Código de terceiros e dos repositórios pessoais que este programa executa
// fica fixado numa versão conhecida: uma conta comprometida ou um push errado
// na branch principal não chega sozinho às próximas instalações. Para
// atualizar, troque o commit aqui (o scripts/atualizar-pins.sh faz isso).
const (
	dotfilesRepo   = "https://github.com/vinicgobbi/Dotfiles.git"
	dotfilesCommit = "1e9a97699b70582f87e768f1fe6aa1dde9b58fd6"

	vscodeNautilusRepo   = "https://github.com/vinicgobbi/vscode_nautilus.git"
	vscodeNautilusCommit = "77ed67c7b420a3a148c83ce8721f18b9e737cd44"

	copypathRepo   = "https://github.com/vinicgobbi/nautilus-copypath.git"
	copypathCommit = "f2e1c9d0a3b5ed2584539dd9d127cf6121fba599"
)

// clonarFixado devolve as linhas de shell que clonam repo em dir (expressão
// de shell, ex.: "$dir/x") e posicionam no commit ref.
func clonarFixado(repo, ref, dir string) string {
	return "git clone --quiet " + shQuote(repo) + " " + dir + "\n" +
		"git -C " + dir + " -c advice.detachedHead=false checkout --quiet " + shQuote(ref) + "\n"
}
