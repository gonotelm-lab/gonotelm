package main

import (
	"context"
	"flag"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/gonotelm-lab/gonotelm/internal/bootstrap"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

func main() {
	configPath := flag.String("config", "./etc/worker.toml.tpl", "config file path")
	flag.Parse()

	_, err := conf.LoadWorkerConfig(*configPath)
	if err != nil {
		panic(err)
	}

	pkglog.Init()
	if err := pkglog.SetLevelText(conf.WorkerGlobal().Logging.Level); err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewWorkerApp(ctx, conf.WorkerGlobal())
	if err != nil {
		slog.Error("new worker app failed", "err", err)
		return
	}
	defer app.Close(context.Background())

	if err := app.Run(ctx); err != nil {
		slog.Error("worker run failed", "err", err)
	}
}
