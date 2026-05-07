package main

import (
	"context"
	"flag"

	"github.com/gonotelm-lab/gonotelm/internal/api"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	storageimpl "github.com/gonotelm-lab/gonotelm/internal/infra/storage/impl"
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

func initObjectStorage() storage.Storage {
	cfg := conf.Global()
	if cfg == nil {
		panic("config is not set")
	}

	storageCfg, err := cfg.ObjectStorageConfig()
	if err != nil {
		panic(err)
	}

	s, err := storageimpl.New(storageimpl.Type(cfg.Storage.Type), storageCfg)
	if err != nil {
		panic(err)
	}

	return s
}

func run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	infras := infra.MustInit(conf.Global())
	objectStorage := initObjectStorage()
	app := logic.MustNewLogic(
		ctx,
		infras,
		objectStorage,
	)

	defer func() {
		// Close MQ consumers/producers before databases to avoid shutdown writes on closed DB.
		app.Close(ctx)
		infra.Close(ctx)
	}()

	api.NewServer(app).Run()
}
