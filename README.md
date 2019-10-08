# go-alibp2p

>此服务用 go-libp2p 实现的全连通 dht 网络，适合做 p2p 网络应用，如内容分享与区块链等，同时适合嵌入式环境，main 函数中简单演示了一些使用样例

## 编译

```
git clone https://github.com/cc14514/go-alibp2p.git
cd go-alibp2p/cmd
go build -o alibp2p main.go
```

## 启动

### 参数

```bash
$> alibp2p --help
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

>使用默认参数启动节点，将会连接到我提供的测试网络中，可以在控制台看到一些 peer ，如果想建立私有网络，请使用自己的 `networkid` 和 `bootnodes` ，其中 `bootnodes` 应该是一个使用了 `nodiscover` 参数启动的节点，启动成功后日志中会有对应的 `url` 信息

### 启动一个 bootnode

>可以启动多个 bootnode 用来增强可用性

```bash
$> alibp2p --nodiscover --networkid 2015061320170611
localhost:cmd liangc$ go run main.go --nodiscover --networkid 2015061320170611
2019/10/08 18:14:19 alibp2p.NewService {context.Background /tmp 10000 [] false 2015061320170611}
2019/10/08 18:14:19 keypath /tmp/p2p.id
2019/10/08 18:14:19 0 address /ip4/127.0.0.1/tcp/10000/ipfs/16Uiu2HAmNx99xhp6wao2hq3EWDWjazqXQWPXJ2aEK3z5M55E9Phe
2019/10/08 18:14:19 1 address /ip4/10.0.0.76/tcp/10000/ipfs/16Uiu2HAmNx99xhp6wao2hq3EWDWjazqXQWPXJ2aEK3z5M55E9Phe
2019/10/08 18:14:19 New Service end =========>
2019/10/08 18:14:19 >> Action on port = 0
2019/10/08 18:14:19 <<<< service_reg_map >>>> : shell
2019/10/08 18:14:19 StartDefaultServer port->8080 ; allow_method->[POST GET]
2019/10/08 18:14:19 host = :8080
```

此时我们的 `bootnode url` 为 `/ip4/10.0.0.76/tcp/10000/ipfs/16Uiu2HAmNx99xhp6wao2hq3EWDWjazqXQWPXJ2aEK3z5M55E9Phe`

### 使用 bootnode 组网

```bash
$> alibp2p --networkid 2015061320170611 --bootnodes /ip4/10.0.0.76/tcp/10000/ipfs/16Uiu2HAmNx99xhp6wao2hq3EWDWjazqXQWPXJ2aEK3z5M55E9Phe 
```

## 控制台

>启动节点后，可以在本地 attach 到控制台进行功能调试

```bash
$> alibp2p attach
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

cmd$> peers
{
	"success": true,
	"entity": {
		"Direct": [
			"/ip4/101.251.230.218/tcp/10000/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP"
		],
		"Relay": [
			"/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP/p2p-circuit/ipfs/16Uiu2HAm1tx8KP6cgbuD8ZbG8ca8eaVKusy6EmC3yA2CLCRAN6vn",
			"/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP/p2p-circuit/ipfs/16Uiu2HAm13zcweNavz7Jxbjujymgb9X1WN5MdrMjgod4fKmQVfiM",
			"/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP/p2p-circuit/ipfs/16Uiu2HAmUnYJQPeRJXoynqurHzCe8vASfArqXX58YyoihWpizxxF",
			"/ipfs/16Uiu2HAkzfSuviNuR7ez9BMkYw98YWNjyBNNmSLNnoX2XADfZGqP/p2p-circuit/ipfs/16Uiu2HAm9fLbABEV2R2NWE1qj13N4wx1r1KTff1ZYPEqgy11q5Px",
			......
		],
		"TimeUsed": "722.891µs",
		"Total": 37
	}
}

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
