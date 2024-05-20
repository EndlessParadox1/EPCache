package epcache

import (
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"github.com/streadway/amqp"
	"google.golang.org/protobuf/proto"
)

func (gp *GrpcPool) producer() {
	conn, err := amqp.Dial(gp.msgBroker)
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
		data := <-gp.ch
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
	}
} // TODO

func (gp *GrpcPool) consumer() {
	conn, err := amqp.Dial(gp.msgBroker)
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
		gp.logger.Fatalf("failed to bind queue to exchange: %v", err)
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
		msg := <-msgs
		var data pb.SyncData
		proto.Unmarshal(msg.Body, &data)
		switch data.Method {
		case "U":
			go gp.node.update(data.Key, data.Value)
		case "R":
			go gp.node.remove(data.Key)
		}
	}
} // TODO
