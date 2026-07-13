package main

import (
	"context"
	"flag"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/bootstrap"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

func main() {
	configPath := flag.String("config", "./etc/gonotelm.toml.tpl", "config file path")
	flag.Parse()

	initConfig(*configPath)
	initLogger()

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
	}
}

func initConfig(configPath string) {
	_, err := conf.LoadAppConfig(configPath)
	if err != nil {
		panic(err)
	}
}

func initLogger() {
	pkglog.Init()

	cfg := conf.AppGlobal()
	if cfg == nil {
		return
	}

	if err := pkglog.SetLevelText(cfg.Logging.Level); err != nil {
		panic(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.NewApp(ctx, conf.AppGlobal())
	if err != nil {
		return err
	}
	defer app.Close()

	app.Server.Run()
	return nil
}
