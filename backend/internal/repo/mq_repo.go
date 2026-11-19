// 本文件实现 RabbitMQ 索引任务的投递与消费。

package repo

import (
	"context"
	"encoding/json"
	"fmt"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// MQRepo RabbitMQ 消息仓储（知识库索引任务投递与消费）。
type MQRepo struct {
	conn  *amqp091.Connection // AMQP 连接
	queue string              // 队列名
}

// NewMQRepo 构造消息仓储。
func NewMQRepo(conn *amqp091.Connection, queueName string) *MQRepo {
	return &MQRepo{conn: conn, queue: queueName}
}

// available 判断 MQ 连接是否可用。
func (r *MQRepo) available() bool {
	return r != nil && r.conn != nil && !r.conn.IsClosed()
}

// DeclareQueue 声明持久化索引队列（幂等）。
func (r *MQRepo) DeclareQueue(ctx context.Context) error {
	if !r.available() {
		return fmt.Errorf("declare queue %s: rabbitmq unavailable", r.queue)
	}
	channel, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer channel.Close()
	if _, err := channel.QueueDeclare(r.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", r.queue, err)
	}
	return nil
}

// PublishIndexJob 投递索引任务消息（persistent 持久化）。
func (r *MQRepo) PublishIndexJob(ctx context.Context, job domain.KnowledgeIndexJobDTO) error {
	if !r.available() {
		return fmt.Errorf("publish index job: rabbitmq unavailable")
	}
	channel, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer channel.Close()
	if _, err := channel.QueueDeclare(r.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", r.queue, err)
	}
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal index job: %w", err)
	}
	if err := channel.PublishWithContext(ctx, "", r.queue, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		Body:         body,
	}); err != nil {
		return fmt.Errorf("publish index job: %w", err)
	}
	return nil
}

// ConsumeIndexJobs 阻塞消费索引任务直至 ctx 取消；handler 返回 nil 才 ack，
// 返回 error 则 requeue 重投（业务失败已在 handler 内部落库并返回 nil，不重投毒消息）。
func (r *MQRepo) ConsumeIndexJobs(ctx context.Context, handler func(ctx context.Context, job domain.KnowledgeIndexJobDTO) error) error {
	if !r.available() {
		return fmt.Errorf("consume index jobs: rabbitmq unavailable")
	}
	channel, err := r.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer channel.Close()
	if _, err := channel.QueueDeclare(r.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", r.queue, err)
	}
	deliveries, err := channel.Consume(r.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", r.queue, err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return nil // 通道/连接关闭
			}
			var job domain.KnowledgeIndexJobDTO
			if err := json.Unmarshal(delivery.Body, &job); err != nil {
				_ = delivery.Nack(false, false) // 载荷非法，丢弃
				continue
			}
			if err := handler(ctx, job); err != nil {
				_ = delivery.Nack(false, true) // 处理中断（如关停），requeue 待重投
			} else {
				_ = delivery.Ack(false)
			}
		}
	}
}
