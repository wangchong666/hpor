package main

import (
	"os"
	"http-proxy-over-rabbitmq-rpc/rpc"
	"flag"
	log "github.com/sirupsen/logrus"
)




var t = flag.String("t", "RPC", "RPC or PROXY,PROXY内网,RPC外网")
var amqp = flag.String("url", "amqp://guest:guest@127.0.0.1:5672/", "amqp地址")
var port = flag.Int("p", 8000, "HTTP代理端口")
var count = flag.Int("c", 50, "消费者数量，并行连接数，与RPC一起使用")


func main() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)

	conf := &rpc.Config{
		Amqp:*amqp,
		Port:*port,
	}

	flag.Parse()

	log.Info("start app as ",*t)
	
	if(*t=="RPC"){
		rpc.Accept(conf,*count)
	}else{
		rpc.Proxy(conf)
	}


}
