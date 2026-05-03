package main

import (
	"context"
	"flag"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/api"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	dalimpl "github.com/gonotelm-lab/gonotelm/internal/infra/dal/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	storageimpl "github.com/gonotelm-lab/gonotelm/internal/infra/storage/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
	vectordalimpl "github.com/gonotelm-lab/gonotelm/internal/infra/vectordal/impl"
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

func initDal() *dal.DAL {
	cfg := conf.Global()
	d, err := dalimpl.New(dalimpl.Type(cfg.Database.Type), cfg.SQLConfig())
	if err != nil {
		panic(err)
	}

	slog.Info("initialized dal", "type", cfg.Database.Type)

	return d
}

func initVectorDal() *vectordal.DAL {
	cfg := conf.Global()
	vd, err := vectordalimpl.New(&cfg.VectorDB)
	if err != nil {
		panic(err)
	}

	slog.Info("initialized vector dal", "type", cfg.VectorDB.Type)

	return vd
}

func run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dal := initDal()
	vdal := initVectorDal()
	objectStorage := initObjectStorage()
	app := logic.NewLogic(ctx, dal, vdal, objectStorage)
	
	defer func() {
		// Close MQ consumers/producers before databases to avoid shutdown writes on closed DB.
		app.Close(ctx)
		if err := vdal.Close(ctx); err != nil {
			slog.ErrorContext(ctx, "close vector dal failed", slog.Any("err", err))
		}
		if err := dal.Close(ctx); err != nil {
			slog.ErrorContext(ctx, "close dal failed", slog.Any("err", err))
		}
	}()

	api.NewServer(app).Run()
}
