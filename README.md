# hpor
http proxy over rabbitmq-rpc

### Usage
```
  -c int
        The number of consumers，work with -t RPC (default 50)
  -d    log debug info
  -p int
        HTTP proxy port (default 8000)
  -q string
        The queue name for rpc (default "rpc_queue")
  -t string
        RPC or PROXY (default "RPC")
  -url string
        amqp address (default "amqp://guest:guest@127.0.0.1:5672//vhost")
```

### Start RPC Server
```
hpor -t RPC
```
with docker
```
docker run --rm -it jcty/hpor -t RPC -url amqp://guest:guest@host:5672/
```

### Start Proxy Server
```
hpor -t PROXY
```
with docker
```
docker run --rm -it jcty/hpor -t PROXY -p 8000:8000 -url amqp://guest:guest@host:5672/
```

The proxy server will listen default port 8000