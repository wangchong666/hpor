# hpor
http proxy over rabbitmq-rpc

### Usage
```
  -c int
        The number of consumers，work with -t RPC (default 50)
  -p int
        HTTP proxy port (default 8000)
  -t string
        RPC or PROXY (default "RPC")
  -url string
        amqp地址 (default "amqp://guest:guest@127.0.0.1:5672/")
```

### Start RPC Server
```
hpor -t RPC
```
with docker
```
docker --rm -it jcty/hpor -t RPC -url amqp://guest:guest@host:5672/
```

### Start Proxy Server
```
hpor -t PROXY
```
with docker
```
docker --rm -it jcty/hpor -t PROXY -url amqp://guest:guest@host:5672/
```

The proxy server will listen default port 8000