package mq

import "context"

type MessageHeader struct {
	Key   string
	Value []byte
}

type Message interface {
	// 消息所处主题
	Topic() string

	// 消息key
	Key() []byte

	// 消息value
	Value() []byte

	// 附加的消息header
	Headers() []MessageHeader
}

type ProducerSendRequest struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers []MessageHeader
}

type Producer interface {
	Send(ctx context.Context, req *ProducerSendRequest) error
	Close(ctx context.Context) error
}

type MessageHandler func(ctx context.Context, msg Message) error

type Consumer interface {
	// 订阅topic主题 handler在后台协程处理消息
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	Close(ctx context.Context) error
}
