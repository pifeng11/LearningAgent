package cli

import (
	"context"
	"fmt"
	"time"

	"learning-agent/internal/app"
	"learning-agent/internal/storage"

	"github.com/spf13/cobra"
)

func NewMigrateCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "执行 PostgreSQL 数据库迁移",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := app.LoadConfig()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			pool, err := storage.OpenPostgres(ctx, cfg.DatabaseURL)
			if err != nil {
				return err
			}
			defer pool.Close()

			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("ping postgres: %w", err)
			}
			if err := storage.ApplyMigrations(ctx, pool, dir); err != nil {
				return err
			}

			fmt.Printf("Applied migrations from %s\n", dir)
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "internal/storage/migrations", "迁移 SQL 文件目录")
	return cmd
}
