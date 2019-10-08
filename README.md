# go-alibp2p
go-libp2p dht instance

```bash
$>go run cmd/main.go --help
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