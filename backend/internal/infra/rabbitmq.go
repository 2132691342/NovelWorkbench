package infra

import (
	"fmt"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// NewRabbitMQ 建立 RabbitMQ 连接；拨号失败返回错误，由调用方降级处理。
func NewRabbitMQ(cfg *RabbitMQConfig) (*amqp091.Connection, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	return conn, nil
}
