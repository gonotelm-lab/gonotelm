package main

import (
	"context"
	"flag"

	"github.com/gonotelm-lab/gonotelm/internal/api"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/event"
	wire "github.com/gonotelm-lab/gonotelm/internal/wire"
	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
)

func main() {
	configPath := flag.String("config", "./etc/gonotelm.toml.tpl", "config file path")
	flag.Parse()

	initConfig(*configPath)
	initLogger()

	run()
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

func run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	infras := infra.MustInit(conf.Global())
	wire.Init(infras)
	app := logic.MustNewLogic(
		ctx,
		infras,
		infras.ObjectStorage,
	)

	defer func() {
		// Close MQ consumers/producers before databases to avoid shutdown writes on closed DB.
		app.Close(ctx)
		infra.Close(ctx)
	}()

	event.Init(ctx, wire.GetWire())
	api.NewServer(app, infras, wire.GetWire()).Run()
}
