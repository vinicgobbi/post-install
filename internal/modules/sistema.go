package modules

var atualizacao = Module{
	ID:    "atualizacao",
	Grupo: GrupoSistema,
	Title: "Atualizar sistema",
	Desc:  "Roda o upgrade completo de pacotes do sistema",
	Run: func(c *Ctx) error {
		c.Log.Infof("Atualizando pacotes do sistema...")
		err := step(
			func() error { return c.Run("apt", "update") },
			func() error { return c.Run("apt", "upgrade", "-y") },
		)
		if err != nil {
			return err
		}
		c.Log.Successf("Sistema atualizado.")
		return nil
	},
}

var limpeza = Module{
	ID:    "limpeza",
	Grupo: GrupoFinalizacao,
	Title: "Limpeza final",
	Desc:  "Remove pacotes órfãos e limpa o cache do APT",
	Run: func(c *Ctx) error {
		c.Log.Infof("Limpando o sistema...")
		err := step(
			func() error { return c.Run("apt", "autoremove", "-y") },
			func() error { return c.Run("apt", "clean") },
		)
		if err != nil {
			return err
		}

		// Os clones feitos como usuário usam mktemp e se apagam sozinhos; aqui
		// só sobra o diretório de trabalho desta execução (o main também o
		// remove ao sair, isto só libera o espaço mais cedo).
		if err := c.RemoveAll(c.WorkDir); err != nil {
			c.Log.Warnf("Não foi possível remover %s: %v", c.WorkDir, err)
		}
		c.Log.Successf("Instalação finalizada com sucesso!")
		c.Log.Infof("Recomenda-se reiniciar a máquina para aplicar as mudanças de grupo e kernel.")
		return nil
	},
}
