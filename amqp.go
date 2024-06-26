package epcache

import (
	"context"
	"sync"
	"sync/atomic"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
)

// produce publishes data-sync messages to MQ.
func (gp *GrpcPool) produce(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := amqp.Dial(gp.mqBroker)
	if err != nil {
		gp.logger.Fatal("failed to connect to MQ:", err)
	}
	defer conn.Close()
	ch, err_ := conn.Channel()
	if err_ != nil {
		gp.logger.Fatal("failed to open a channel:", err)
	}
	defer ch.Close()
	err = ch.ExchangeDeclare(
		gp.opts.Exchange,
		"fanout",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		gp.logger.Fatal("failed to declare an exchange:", err)
	}
	for {
		select {
		case data := <-gp.syncCh:
			body, _ := proto.Marshal(data)
			err = ch.Publish(
				gp.opts.Exchange,
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "application/x-protobuf",
					AppId:       gp.self,
					Body:        body,
				},
			)
			if err != nil {
				gp.logger.Println("failed to publish a message:", err)
			}
		case <-ctx.Done():
			gp.logger.Println("Data sync sender stopped")
			return
		}
	}
}

// consume consumes the data-sync messages from MQ and then tries to change the local cache.
func (gp *GrpcPool) consume(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := amqp.Dial(gp.mqBroker)
	if err != nil {
		gp.logger.Fatal("failed to connect to MQ:", err)
	}
	defer conn.Close()
	ch, err_ := conn.Channel()
	if err_ != nil {
		gp.logger.Fatal("failed to open a channel:", err_)
	}
	defer ch.Close()
	err = ch.ExchangeDeclare(
		gp.opts.Exchange,
		"fanout",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		gp.logger.Fatal("failed to declare an exchange:", err)
	}
	q, err1 := ch.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)
	if err1 != nil {
		gp.logger.Fatal("failed to declare a queue:", err1)
	}
	err = ch.QueueBind(
		q.Name,
		"",
		gp.opts.Exchange,
		false,
		nil,
	)
	if err != nil {
		gp.logger.Fatal("failed to bind queue to exchange:", err)
	}
	msgCh, err2 := ch.Consume(
		q.Name,
		"",
		true,
		true,
		false,
		false,
		nil,
	)
	if err2 != nil {
		gp.logger.Fatal("failed to consume messages:", err2)
	}
	for {
		select {
		case msg := <-msgCh:
			if msg.AppId != gp.self {
				atomic.AddInt64(&gp.node.Stats.PeerSyncs, 1)
				var data pb.SyncData
				proto.Unmarshal(msg.Body, &data)
				switch data.Method {
				case "U":
					gp.node.update(data.Key, data.Value)
				case "R":
					gp.node.remove(data.Key)
				}
			} // to ignore messages from self
		case <-ctx.Done():
			gp.logger.Println("Data sync receiver stopped")
			return
		}
	}
}
