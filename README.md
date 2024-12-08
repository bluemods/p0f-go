# p0f-go

## P0f client for Golang

Designed to make p0f easy to work with for all programming languages, it stands up a simple HTTP server that replies to HTTP GET requests with p0f data in JSON format.

It can also be used as a library if an HTTP server is undesired.

NOTE: This library makes use of p0f-mtu, which is NOT compatible with p0f. 
[Get p0f-mtu from here](https://github.com/ValdikSS/p0f-mtu/)

### Installing p0f-mtu

```bash
git clone https://github.com/ValdikSS/p0f-mtu/ && cd p0f-mtu && chmod +x ./build.sh && ./build.sh
```

### Preparing p0f-mtu

```bash
./p0f -s /tmp/p0f-mtu.sock -d
```

### Running p0f-go

```bash
go build && ./p0f-go -s /tmp/p0f-mtu.sock -p 38749
```

### Querying HTTP API externally

```bash
curl http://localhost:38749?p=1
```