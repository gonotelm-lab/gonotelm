package logic

import (
	"context"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq"
	mqimpl "github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl"
	"github.com/gonotelm-lab/gonotelm/internal/infra/mq/impl/kafka"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infra/vectordal"
)

type Logic struct {
	NotebookLogic *NotebookLogic
	SourceLogic   *SourceLogic
	ChatLogic     *ChatLogic
}

func MustNewLogic(
	ctx context.Context,
	dalImpl *dal.DAL,
	vectorDalImpl *vectordal.DAL,
	objectStorage storage.Storage,
) *Logic {
	// biz instances initialization
	var (
		notebookBiz = biznotebook.New(dalImpl.NotebookStore)
		chatBiz     = bizchat.New(dalImpl.ChatMessageStore)
	)

	sourceBiz, err := bizsource.New(
		objectStorage,
		dalImpl.SourceStore,
		vectorDalImpl.SourceDocStore,
	)
	if err != nil {
		panic(err)
	}

	notebookLogic := NewNotebookLogic(
		notebookBiz,
		sourceBiz,
	)

	sourceLogic := MustNewSourceLogic(
		ctx,
		objectStorage,
		notebookLogic.notebookBiz,
		sourceBiz,
	)

	chatLogic := NewChatLogic(
		notebookBiz,
		sourceBiz,
		chatBiz,
	)

	return &Logic{
		NotebookLogic: notebookLogic,
		SourceLogic:   sourceLogic,
		ChatLogic:     chatLogic,
	}
}

func (l *Logic) Close(ctx context.Context) {
	l.SourceLogic.Close(ctx)
	l.ChatLogic.Close(ctx)
}

func mustNewMsgQueueProducer() mq.Producer {
	switch conf.Global().MsgQueue.Type {
	case mqimpl.Kafka:
		return kafka.NewProducer(kafka.ProducerConfig{
			Brokers:  conf.Global().MsgQueue.Kafka.Brokers,
			Username: conf.Global().MsgQueue.Kafka.Username,
			Password: conf.Global().MsgQueue.Kafka.Password,
		})
	default:
		panic("unknown msg queue type")
	}
}

func mustNewMsgQueueConsumer(topic, groupId string) mq.Consumer {
	switch conf.Global().MsgQueue.Type {
	case mqimpl.Kafka:
		return kafka.NewConsumer(kafka.ConsumerConfig{
			Brokers:        conf.Global().MsgQueue.Kafka.Brokers,
			GroupID:        groupId,
			Topic:          topic,
			QueueCapacity:  conf.Global().MsgQueue.Kafka.ConsumerQueueCapacity,
			CommitInterval: conf.Global().MsgQueue.Kafka.ConsumerCommitInterval,
			Username:       conf.Global().MsgQueue.Kafka.Username,
			Password:       conf.Global().MsgQueue.Kafka.Password,
		})
	default:
		panic("unknown msg queue type")
	}
}
