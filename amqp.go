package epcache

import (
	"context"
	"sync"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"github.com/streadway/amqp"
	"google.golang.org/protobuf/proto"
)

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
		case data := <-gp.ch:
			body, _ := proto.Marshal(data)
			err = ch.Publish(
				gp.opts.Exchange,
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        body,
				},
			)
			if err != nil {
				gp.logger.Print("failed to publish a message:", err)
			}
		case <-ctx.Done():
			gp.logger.Println("Data sync sender stopped")
			return
		}
	}
} // TODO

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
	q, _err := ch.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)
	if _err != nil {
		gp.logger.Fatal("failed to declare a queue:", err)
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
	msgs, err1 := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err1 != nil {
		gp.logger.Fatal("failed to consume messages:", err1)
	}
	for {
		select {
		case msg := <-msgs:
			var data pb.SyncData
			proto.Unmarshal(msg.Body, &data)
			switch data.Method {
			case "U":
				go gp.node.update(data.Key, data.Value)
			case "R":
				go gp.node.remove(data.Key)
			}
		case <-ctx.Done():
			gp.logger.Println("Data sync receiver stopped")
			return
		}
	}
} // TODO
