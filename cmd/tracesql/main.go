package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	"github.com/Felipe-Cavalca/TraceSQL/internal/db"
	"github.com/Felipe-Cavalca/TraceSQL/internal/export"
	"github.com/Felipe-Cavalca/TraceSQL/internal/prompt"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Default()
	logf := func(format string, args ...interface{}) {
		if !cfg.Log {
			return
		}
		fmt.Fprintf(os.Stderr, "[tracesql] "+format+"\n", args...)
	}

	rootCmd := &cobra.Command{
		Use:   "tracesql",
		Short: "Exporta registros relacionais para um arquivo .sql",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.BindFlags(cmd, &cfg); err != nil {
				return err
			}
			logf("configuração carregada: origem=%s destino=%s tabela=%s coluna=%s registro=%s new_ids=%t", cfg.Driver, cfg.OutputDriver, cfg.Table, cfg.Column, cfg.Record, cfg.NewIDs)

			if err := prompt.FillMissing(&cfg, os.Stdin, os.Stdout); err != nil {
				return err
			}
			logf("prompts concluídos")

			if err := cfg.Validate(); err != nil {
				return err
			}
			logf("configuração validada")

			database, err := db.Open(cfg.Driver, cfg.DSN)
			if err != nil {
				return err
			}
			defer database.Close()
			logf("conexão com banco aberta")

			ctx := context.Background()
			sqlDump, err := export.Run(ctx, database, cfg)
			if err != nil {
				return err
			}
			logf("exportação concluída, %d bytes gerados", len(sqlDump))

			outPath := cfg.OutPath()
			if err := os.WriteFile(outPath, []byte(sqlDump), 0o644); err != nil {
				return fmt.Errorf("gravando arquivo %s: %w", outPath, err)
			}
			logf("arquivo gravado em %s", outPath)

			fmt.Fprintf(os.Stdout, "Arquivo gerado: %s\n", outPath)
			return nil
		},
	}

	config.AttachFlags(rootCmd, cfg)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("erro: %v", err)
	}
}
