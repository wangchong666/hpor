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

### start RPC Server
```
hpor -t RPC
```

### start Proxy Server
```
hpor -t PROXY
```

The proxy server will listen default port 8000