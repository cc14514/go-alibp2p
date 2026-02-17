# go-alibp2p

基于 `go-libp2p` 的 P2P 网络服务封装：

- 内置 Kademlia DHT（可做节点发现 / KV 存储）
- 支持私有网络（PSK）隔离（所有节点需要一致的 `networkid`）
- 支持普通 Stream 协议处理与“复用 Stream”的请求/响应模型
- 内置 GossipSub PubSub
- 适合做内容分发、区块链、局域网/私有网络 P2P 通讯；也可用于嵌入式环境

> 包名：`alibp2p`，模块：`github.com/cc14514/go-alibp2p`

## 安装

### Go Modules

```bash
go get github.com/cc14514/go-alibp2p
```

本仓库的 [go.mod](go.mod) 已包含对 `go-libp2p` 若干组件的 `replace`（用于锁定兼容版本）；如果你把它作为依赖引入，通常无需额外操作。

### Go 版本

`go.mod` 里声明的 `go 1.13` 表示该项目按较老的 Go / libp2p 依赖栈开发。

- 如果你使用很新的 Go 版本，可能会遇到依赖不兼容导致的编译/链接失败（需要降级 Go 工具链，或升级 libp2p 相关依赖）。

### Vendor

如果你使用 vendor，请参考下方“注意事项”。

## 快速开始

### 1) 创建并启动节点

`New` 会返回 `Alibp2pService`、最终生效的 `Config` 以及错误：

```go
package main

import (
	"log"

	alibp2p "github.com/cc14514/go-alibp2p"
)

func main() {
	srv, _, err := alibp2p.New(
		alibp2p.Homedir("./data/node1"),
		alibp2p.Network(30200, 0, 111),
		alibp2p.Discover(true),
		alibp2p.Relay(false),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 建议：先注册协议/事件，再 Start()
	srv.Start()

	id, addrs := srv.Myid()
	log.Println("peerID:", id)
	log.Println("listen addrs:", addrs)
}
```

说明：

- `Homedir(dir)`：用于持久化节点私钥（默认文件 `p2p.id`）。不传 `Homedir` 时必须 `Identity(priv)`。
- `Network(port, muxport, networkid)`：设置监听端口/自定义 mux 地址/私有网络标识。
- `Discover(true)`：开启定时 bootstrap（通常需要配置 `Bootnodes(...)`）。

### 2) 连接到其他节点

`Connect` 需要传入带 `/ipfs/<peerID>`（或 `/p2p/<peerID>`）的 multiaddr：

```go
_ = srv.Connect("/ip4/127.0.0.1/tcp/30200/ipfs/16Uiu2H...")
```

也可以直接设置引导节点列表：

```go
_ = srv.SetBootnode(
	"/ip4/1.2.3.4/tcp/30200/ipfs/16Uiu2H...",
)
```

### 3) 普通 Stream：一次请求/一次响应

普通模式下，`Request()` 会把请求 bytes 直接写入 stream，对端 handler 从 stream 读取请求、写回响应；handler 返回后 stream 会被关闭。

服务端：

```go
import (
	"crypto/ecdsa"
	"io"
	"io/ioutil"
)

const ProtoPing = "/demo/ping/1.0.0"

srv.SetHandlerWithProtocol(ProtoPing, func(sessionID, protocol string, pub *ecdsa.PublicKey, rw io.ReadWriter) error {
	req, _ := ioutil.ReadAll(rw)
	_ = req // 这里处理 req
	_, _ = rw.Write([]byte("pong"))
	return nil
})
```

客户端：

```go
resp, err := srv.Request(peerID, ProtoPing, []byte("ping"))
```

### 4) 复用 Stream：高频请求/响应（推荐用于 RPC 场景）

当你希望同一 peer 同一协议上复用连接/stream（减少频繁建连开销）时，使用：

- `SetHandlerReuseStreamWithProtocol` 注册处理器
- `Request/RequestWithTimeout` 发起请求
- `SendMsgAfterClose` 发送并等待 ack（复用模式下用于“只发不关”的场景）

复用模式的数据帧使用 `amino` 的 length-prefixed 编解码，内部承载结构为 `RawData{Id, Err, Data}`；你在 handler 中只需要按普通 `io.ReadWriter` 读写业务 bytes。

```go
srv.SetHandlerReuseStreamWithProtocol("/demo/echo/1.0.0", func(sessionID, protocol string, pub *ecdsa.PublicKey, rw io.ReadWriter) error {
	req, _ := ioutil.ReadAll(rw)
	_, _ = rw.Write(append([]byte("echo:"), req...))
	return nil
})

resp, err := srv.Request(peerID, "/demo/echo/1.0.0", []byte("hello"))
```

注意：

- 对复用协议，`SendMsg()` 会直接返回错误（请用 `Request/SendMsgAfterClose`）。
- 非复用协议的响应读取依赖对端关闭 stream；因此 handler 必须在写完响应后返回。

### 5) DHT KV：Put / Get

```go
_ = srv.Put("foo", []byte("bar"))
v, err := srv.Get("foo")
```

库会把 key 写入固定 namespace（见源码常量 `NamespaceDHT`），调用方只传业务 key。

### 6) Provider 发现：Advertise / FindProviders

```go
srv.SetAdvertiseTTL("my-namespace", 3600) // 也可不设，走默认 TTL
srv.Advertise(ctx, "my-namespace")

ids, err := srv.FindProviders(ctx, "my-namespace", 50)
```

### 7) PubSub：Publish / Subscribe

```go
ps := srv.Pubsub()

_ = ps.Subscribe("topic-1", func(from, receivedFrom peer.ID, data []byte) {
	// handle msg
}, alibp2p.WithValidator(func(from, receivedFrom peer.ID, data []byte) bool {
	return len(data) > 0
}))

_ = ps.Publish("topic-1", []byte("hello"))
_ = ps.Unsubscribe("topic-1")
```

## 配置项（Option）速查

- `Homedir(dir)`：节点私钥持久化目录（生成/读取 `p2p.id`）
- `Identity(priv)`：直接指定节点私钥（`*ecdsa.PrivateKey`）
- `Network(port, muxport, networkid)`：监听端口、mux 地址、私有网络标识
- `Bootnodes(...)`：引导节点 multiaddr 列表
- `Discover(true/false)`：是否启动 bootstrap 循环
- `ConnectManger(bootstrapPeriod, connLow, connHi)`：bootstrap 周期与连接管理器阈值
- `Relay(true/false)`：是否启用 relay hop（并在部分场景添加 circuit 地址）
- `DisableInbound(true/false)`：DHT client 模式并拒绝入站连接
- `ClientProtocols(...)`：用于 bootstrap 时排除特定协议类型的 peers（避免连到不应连接的节点）
- `EnableMetric(true/false)`：开启 DHT metrics
- `Loglevel(3/4/5/...)`：日志级别（源码约定：3-4 为 Info，5 为 Debug）
- `MaxMsgSize(size)`：`amino` reader 最大包大小（最小 1MB）
- `Context(ctx)`：自定义 context

## 常见约定

- peer 标识：大多数 API 使用 peerID（例如 `16Uiu2H...`）；`Connect` 需要 multiaddr。
- 私有网络（PSK）：节点之间必须使用相同的 `networkid` 才能互通。

## 注意事项

* 使用 go mod 管理依赖时，需要在 `go.mod` 中替重定向一下几个库

```
// 需要将以下内容添加到 go.mod 尾部

replace github.com/libp2p/go-libp2p-discovery => github.com/cc14514/go-libp2p-discovery v0.0.0-20200509061928-ab91365d125c
replace github.com/libp2p/go-libp2p-kad-dht => github.com/cc14514/go-libp2p-kad-dht v0.0.3-rc0
replace github.com/libp2p/go-libp2p => github.com/cc14514/go-libp2p v0.0.3-rc0
replace github.com/libp2p/go-libp2p-circuit => github.com/cc14514/go-libp2p-circuit v0.0.3-rc0

```


