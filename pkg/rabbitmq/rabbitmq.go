package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

func NewClient(url, queueName string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// 1. 声明死信交换机 (DLX)
	dlxName := queueName + ".dlx"
	err = ch.ExchangeDeclare(
		dlxName, // name
		"direct", // kind
		true,    // durable
		false,   // auto-delete
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare dlx: %w", err)
	}

	// 2. 声明死信队列 (DLQ)
	dlqName := queueName + ".dlq"
	_, err = ch.QueueDeclare(
		dlqName, // name
		true,    // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare dlq: %w", err)
	}

	// 3. 绑定 DLQ 到 DLX
	err = ch.QueueBind(
		dlqName, // queue name
		queueName, // routing key (使用原队列名作为 key)
		dlxName, // exchange
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind dlq: %w", err)
	}

	// 4. 声明主队列，配置死信参数
	args := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": queueName,
	}

	_, err = ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		args,      // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	return &Client{
		conn:  conn,
		ch:    ch,
		queue: queueName,
	}, nil
}

func (c *Client) Publish(ctx context.Context, body []byte) error {
	return c.ch.PublishWithContext(ctx,
		"",      // exchange
		c.queue, // routing key
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
}

func (c *Client) Consume(prefetchCount int) (<-chan amqp.Delivery, error) {
	// 设置 QoS (Quality of Service)
	// prefetchCount: 限制服务器一次发送给消费者的未确认消息数量
	// global: false 表示该限制应用于每个 Channel，而不是连接
	if err := c.ch.Qos(
		prefetchCount, // prefetch count
		0,             // prefetch size
		false,         // global
	); err != nil {
		return nil, fmt.Errorf("failed to set qos: %w", err)
	}

	return c.ch.Consume(
		c.queue, // queue
		"",      // consumer
		false,   // auto-ack (we use manual ack)
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
}

func (c *Client) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
