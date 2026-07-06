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
	cfg, err := conf.Load(configPath)
	if err != nil {
		panic(err)
	}

	conf.SetGlobal(cfg)
}

func initLogger() {
	pkglog.Init()

	cfg := conf.Global()
	if cfg == nil {
		return
	}

	if err := pkglog.SetLevelText(cfg.Logging.Level); err != nil {
		panic(err)
	}
}

// run wires the application using bootstrap.NewApp.
// NOTE: bootstrap.NewApp is partially wired — Tasks 11-12 will complete the logic,
// event handler registration, and HTTP server wiring. The build may not fully
// compile until those tasks are done.
func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := bootstrap.NewApp(ctx, conf.Global())
	if err != nil {
		return err
	}
	defer app.Close()

	app.Server.Run()
	return nil
}
