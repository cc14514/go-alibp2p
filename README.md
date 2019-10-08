# go-alibp2p

>用 go-libp2p 实现的全联通 dht 网络，适合做 p2p 网络应用，如内容分享与区块链等，同时适合嵌入式环境，main 函数中提供了一些使用样例，如题如下

## 参数说明

```bash
$> go run cmd/main.go --help
NAME:
   /var/folders/b7/q0wwxn550x3_mkt1glwzv7rc0000gn/T/go-build982463937/b001/exe/main - 用来演示 go-alibp2p 的组网和通信功能

USAGE:
   main [global options] command [command options] [arguments...]

VERSION:
   0.0.1

AUTHOR:
   liangc <cc14514@icloud.com>

COMMANDS:
   attach   得到一个 shell 用来操作本地已启动的节点
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --nodiscover               false to disable bootstrap
   --rpcport PORT             HTTP-RPC server listening PORT (default: 8080)
   --port value               service tcp port (default: 10000)
   --networkid value          network id (default: 1)
   --homedir value, -d value  home dir (default: "/tmp")
   --bootnodes value          bootnode list split by ','
   --help, -h                 show help
   --version, -v              print the version
```

## 控制台

>启动节点后，可以在本地 attach 到控制台进行功能调试

```bash
$> go run cmd/main.go attach
------------
hello world
------------
cmd$> help
------------------------------
echo [id/url] [msg]
put [key] [val]
get [key]
findpeer [id]
peers
myid
help , exit , quit
------------------------------
cmd$> findpeer 16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP
{
	"success": true,
	"entity": {
		"Data": {
			"Addrs": [
				"/ip4/192.168.18.233/tcp/10000",
				"/p2p-circuit/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP",
				"/ip4/101.251.230.218/tcp/10000",
				"/ip4/127.0.0.1/tcp/10000"
			],
			"ID": "16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP"
		},
		"TimeUsed": "41.226µs"
	}
}
cmd$> echo 16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP hello
{
	"success": true,
	"entity": {
		"Data": "hello\n",
		"TimeUsed": "6.727691ms"
	}
}
cmd$> exit
```