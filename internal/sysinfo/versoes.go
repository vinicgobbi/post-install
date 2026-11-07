// Versões suportadas — mantido pelo scripts/atualizar-versoes.sh, que o
// workflow versoes.yml roda toda semana, abrindo um PR quando algo muda.
// Pode ser editado à mão, mas o script regrava o arquivo inteiro.

package sysinfo

// ubuntuSuportadas são as LTS do Ubuntu com suporte padrão (sem contar o
// ESM), da mais antiga para a mais nova. MSRepo é a versão usada nas URLs do
// repositório da Microsoft (msodbcsql/mssql-tools): a própria, ou a da LTS
// anterior enquanto a Microsoft não publica a nova.
var ubuntuSuportadas = []versaoUbuntu{
	{Versao: "22.04", Codinome: "jammy", MSRepo: "22.04"},
	{Versao: "24.04", Codinome: "noble", MSRepo: "24.04"},
	{Versao: "26.04", Codinome: "resolute", MSRepo: "26.04"},
}

// fedoraSuportadas são as versões do Fedora em manutenção, da mais antiga
// para a mais nova. MSRepo é a versão do RHEL usada nas URLs do repositório
// da Microsoft (que não publica pacotes para o Fedora).
var fedoraSuportadas = []versaoFedora{
	{Versao: "43", MSRepo: "10"},
	{Versao: "44", MSRepo: "10"},
	{Versao: "45", MSRepo: "10"},
}
