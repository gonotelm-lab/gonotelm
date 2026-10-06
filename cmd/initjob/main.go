// Command initjob runs one-shot business data initialization. It is not a
// migration of the database schema: schema migrations belong to cmd/migrate.
//
// It exits non-zero whenever a task failed or the run was interrupted, which is
// what lets a scheduler such as a Kubernetes Job retry it. Retries only execute
// tasks that have not succeeded yet.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gonotelm-lab/gonotelm/internal/bootstrap"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

func main() {
	if err := run(); err != nil {
		slog.Error("initjob failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "./etc/initjob.toml.tpl", "config file path")
	assetsDir := flag.String("assets-dir", "./assets", "assets directory path")
	flag.Parse()

	if _, err := conf.LoadInitJobConfig(*configPath); err != nil {
		return err
	}

	pkglog.Init()
	if err := pkglog.SetLevelText(conf.InitJobGlobal().Logging.Level); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewInitJob(ctx, conf.InitJobGlobal(), *assetsDir)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := app.Close(context.Background()); closeErr != nil {
			slog.Error("close initjob app failed", "err", closeErr)
		}
	}()

	return app.Run(ctx)
}
