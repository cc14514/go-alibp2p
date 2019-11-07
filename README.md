# go-alibp2p

>此服务用 go-libp2p 实现的全连通 dht 网络，适合做 p2p 网络应用，如内容分享与区块链等，同时适合嵌入式环境

## 注意事项

* 使用 vendor 管理依赖时，请直接将 `go-alibp2p` 放入 `vendor/github.com/cc14514/go-alibp2p` 目录，不要直接使用 `govendor` 添加

* 使用 go mod 管理依赖时，需要在 `go.mod` 中替重定向一下几个库

```
// 需要将以下内容添加到 go.mod 尾部
replace github.com/libp2p/go-libp2p-kad-dht => github.com/cc14514/go-libp2p-kad-dht v0.0.0-20191107040323-2463a62af156
replace github.com/libp2p/go-libp2p => github.com/cc14514/go-libp2p v0.0.0-20191107035444-bf2343196cca
replace github.com/libp2p/go-libp2p-circuit => github.com/cc14514/go-libp2p-circuit v0.0.0-20191107035843-34ac9445e418
```

## example

* [使用示例](https://github.com/cc14514/go-alibp2p-example)


