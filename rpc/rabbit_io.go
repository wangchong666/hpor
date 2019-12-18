package rpc

import (
	"strings"
	"net/url"
	"bytes"
	"fmt"
	"net"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"io"
)

type RabbitIO interface {
	// Read reads data from the connection.
	// Read can be made to time out and return an Error with Timeout() == true
	// after a fixed time limit; see SetDeadline and SetReadDeadline.
	Read(b []byte) (n int, err error)

	// Write writes data to the connection.
	// Write can be made to time out and return an Error with Timeout() == true
	// after a fixed time limit; see SetDeadline and SetWriteDeadline.
	Write(b []byte) (n int, err error)

	Close() error
	CreateRequest(data []byte)

}



type rabbitIO struct {
	correlationId string
	ch * amqp.Channel
	q	 amqp.Queue
	msgs <- chan amqp.Delivery
	conn net.Conn
	sender bool
	replyTo  string
	queueName string
	buffer *bytes.Buffer
}


func (c *rabbitIO)Read(b []byte) (n int, err error){
	if(c.sender){
		d := <- c.msgs
		log.Debug("read ",len(d.Body)," bytes") 
		if c.correlationId == d.CorrelationId && len(d.Body)>0 {
			// res, err = strconv.Atoi(string(d.Body))
			copy(b[0:len(d.Body)], d.Body[:])
			return len(d.Body),nil
		}else{
			return 0,io.EOF
		}
	}else{
		n,err = c.buffer.Read(b)
		log.Debug(n,"---",err)
		return n,err
	}
}

func (c *rabbitIO)Write(b []byte) (n int, err error){
	log.Debug("write ",len(b)," bytes")
	if(c.sender){
		err = c.ch.Publish(
			"",          // exchange
			c.queueName, // routing key
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


func (c *rabbitIO) Close() error{
	log.Debug("closing")
	if(c.sender){
		c.ch.Close()
	}else{
		c.Write(make([]byte, 0))
		// delete(conn_map,c.correlationId)
	}
	if(c.conn != nil){
		return c.conn.Close()
	}else{
		return nil
	}
}

func (c *rabbitIO)CreateRequest(data []byte){
	var method, host, address string
	index := bytes.IndexByte(data[:], '\n')
	if(index < 0){
		log.Error("wrong package,drop")
		c.Close()
		return 
	}
	fmt.Sscanf(string(data[:]), "%s%s", &method, &host)
	hostPortURL, err := url.Parse(host)
	log.Info("hostPortURL="+host)
	if(err != nil){
		log.Error(err," cannot pass host")
		c.Close()
		return 
	}
		
	if hostPortURL.Opaque == "443" { //https访问
		address = hostPortURL.Scheme + ":443"
	} else { //http访问
		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			address = hostPortURL.Host + ":80"
		} else {
			address = hostPortURL.Host
		}
	}
		
	//获得了请求的host和port，就开始拨号吧
	server, err := net.Dial("tcp", address)
	if(err != nil){
		log.Warn("Dail err ",err,"closing")
		c.Close()
		return
	}
	c.conn = server
	// server.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	if method == "CONNECT" {
		// fmt.Fprint(client, )
		c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	} else {
		server.Write(data[:])
	}

	//进行转发
	go func(){
		n,err := io.Copy(c, server)
		log.Debug(n,"  ",err)
		log.Debug("1完成")
		c.Close()
	}()
	// go func(){
	// 	io.Copy(server, c)
	// 	log.Debug("2完成")
	// 	c.Close()
	// }()
}

