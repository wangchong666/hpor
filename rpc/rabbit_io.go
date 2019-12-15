package rpc

import (
	"time"
	"strings"
	"net/url"
	"bytes"
	"net"
	"math/rand"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"fmt"
	"io"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}



func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}


type RabbitIO interface {
	// Read reads data from the connection.
	// Read can be made to time out and return an Error with Timeout() == true
	// after a fixed time limit; see SetDeadline and SetReadDeadline.
	Read(b []byte) (n int, err error)

	// Write writes data to the connection.
	// Write can be made to time out and return an Error with Timeout() == true
	// after a fixed time limit; see SetDeadline and SetWriteDeadline.
	Write(b []byte) (n int, err error)

	Copy(dst io.Writer, src io.Reader) (written int64, err error) 

	GetConn() net.Conn
	SetConn(conn net.Conn) 

	Close() error
}



type rabbitIO struct {
	correlationId string
	ch * amqp.Channel
	q	 amqp.Queue
	msgs <- chan amqp.Delivery
	conn net.Conn
	sender bool
	replyTo  string
}


type Config struct{
	Amqp string
	Port int
}

func (c *rabbitIO)Read(b []byte) (n int, err error){
	d := <- c.msgs
	log.Debug("read",len(d.Body)," bytes") 
	if c.correlationId == d.CorrelationId && len(d.Body)>0 {
		// res, err = strconv.Atoi(string(d.Body))
		copy(b[0:len(d.Body)], d.Body[:])
		return len(d.Body),nil
	}else{
		return 0,io.EOF
	}
}

func (c *rabbitIO)Write(b []byte) (n int, err error){
	log.Debug("write ",len(b)," bytes")
	if(c.sender){
		err = c.ch.Publish(
			"",          // exchange
			"rpc_queue", // routing key
			false,       // mandatory
			false,       // immediate
			amqp.Publishing{
				ContentType:   "text/plain",
				CorrelationId: c.correlationId,
				ReplyTo:       c.q.Name,
				Body:          b,
			})
	}else{
		err = c.ch.Publish(
				"",        // exchange
				c.replyTo, // routing key
				false,     // mandatory
				false,     // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: c.correlationId,
					Body: b,
				})
	}
	return len(b),err
}


type writerOnly struct {
	io.Writer
}

// Fallback implementation of io.ReaderFrom's ReadFrom, when sendfile isn't
// applicable.
func (c *rabbitIO)Copy(w io.Writer, r io.Reader) (n int64, err error) {
	// Use wrapper to hide existing r.ReadFrom from io.Copy.
	log.Debug("copy")
	n,err = io.Copy(writerOnly{w}, r)
	log.Debug("copy",n,err)
	if(err != nil || n == 0){
		c.Close()
	}
	return n,err
}


func (c *rabbitIO)GetConn() net.Conn{
	return c.conn
}

func (c *rabbitIO)SetConn(conn net.Conn) {
	c.conn = conn
}


func (c *rabbitIO) Close() error{
	log.Debug("closing")
	if(c.sender){
		c.ch.Close()
	}else{
		c.Write(make([]byte, 0))
	}
	if(c.GetConn() != nil){
		return c.GetConn().Close()
	}else{
		return nil
	}
}

func Proxy(conf *Config){
	l, err := net.Listen("tcp",  fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Panic(err)
	}

	conn, err := amqp.Dial(conf.Amqp)
	failOnError(err, "Failed to connect to RabbitMQ")

	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		// client.SetReadDeadline(time.Now().Add(5 * time.Second))
		go handleClientRequest(client,conn,conf)
	}
}

func handleClientRequest(client net.Conn,conn *amqp.Connection,conf *Config) {
	if client == nil {
		return
	}
	// defer client.Close()

	rio,err := Dial(conn , conf)
	failOnError(err,"cannot connect to mq")

	rio.SetConn(client)
	defer rio.Close()


	wait := make(chan bool)
	//进行转发
	go func(){
		io.Copy(rio, client)
		log.Debug("1完成")
		wait<-true
	}()
	go func(){
		io.Copy(client, rio)
		log.Debug("2完成")
		wait<-true
	}()
	
	<-wait
	log.Debug("关闭。。。")

}

func Dial(conn *amqp.Connection,conf *Config) (RabbitIO, error) {

	// defer conn.Close()

	ch, err := conn.Channel()
	if(err != nil){
		log.Error("Failed to open a channel",err)
	}
	// failOnError(err, "Failed to open a channel")
	// defer ch.Close()

	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		true, // delete when unused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	corrId := randomString(32)

	io := &rabbitIO{
		correlationId:corrId,
		ch:ch,
		q:q,
		msgs: msgs,
		sender: true,
	}
	log.Info("new connection",corrId)
	return io,nil
}

var conn_map = make(map[string]RabbitIO) //连接复用

func Accept(conf *Config, count int){
	conn, err := amqp.Dial(conf.Amqp)
	failOnError(err, "Failed to open a amqp connection")
	for i:=0;i<count;i++ {
		go accept(conf,conn)
	}
	forever := make(chan bool)
	<-forever
}
func accept(conf *Config,conn *amqp.Connection) {
	ch, err := conn.Channel()
	defer ch.Close()
	failOnError(err, "Failed to open a channel")
	q, err := ch.QueueDeclare(
		"rpc_queue", // name
		false,       // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")
    
	log.Printf(" [*] Awaiting RPC requests")
	for d := range msgs {
		log.Debug("read",len(d.Body),"bytes")
		iox,ok := conn_map[d.CorrelationId]
		if(len(d.Body) == 0){
			d.Ack(false)
			if(ok){iox.Close()}
			continue
		}
		if(ok == false){
			io := &rabbitIO{
				correlationId:d.CorrelationId,
				ch:ch,
				q:q,
				msgs: msgs,
				sender:false,
				replyTo:d.ReplyTo,
			}


			var method, host, address string
			fmt.Sscanf(string(d.Body[:bytes.IndexByte(d.Body[:], '\n')]), "%s%s", &method, &host)
			hostPortURL, err := url.Parse(host)
			log.Info("hostPortURL="+host)
			failOnError(err,"cannot pass host")
				
			if hostPortURL.Opaque == "443" { //https访问
				address = hostPortURL.Scheme + ":443"
				// d.Ack(false)
				// continue
			} else { //http访问
				if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
					address = hostPortURL.Host + ":80"
				} else {
					address = hostPortURL.Host
				}
			}
				
			//获得了请求的host和port，就开始拨号吧
			server, err := net.Dial("tcp", address)
			io.conn = server
			// failOnError(err,"net.Dial "+host+ "failed")
			if(err != nil){
				io.Close()
				d.Ack(false)
				log.Warn("Dail err ",err,"closing")
				continue
			}
			server.SetReadDeadline(time.Now().Add(5 * time.Second))
			
			if method == "CONNECT" {
				// fmt.Fprint(client, )
				io.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
			} else {
				server.Write(d.Body[:])
			}
			conn_map[d.CorrelationId] = io
			//进行转发
			go io.Copy(io, server)
			// go io.Copy(server,io)
			d.Ack(false)
			// log.Debug("完成")
		}else{
			log.Debug("二次交互")
			iox.GetConn().Write(d.Body)
			d.Ack(false)
			log.Debug("二次交互完成")
		}
	}

	forever := make(chan bool)
	<-forever
}

