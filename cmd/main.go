// For test and demo alibp2p mod
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cc14514/go-alibp2p"
	"time"
)

var (
	homedir = flag.String("d", "/tmp", "home dir")
	port    = flag.Int("p", 10000, "tcp listen")
	target  = flag.String("t", "", "connect to")
)

func main() {
	flag.Parse()
	if homedir == nil || *homedir == "" {
		panic("homedir can not empty.")
	}
	p2pservice := alibp2p.NewService(context.Background(), *homedir, *port, nil)
	go p2pservice.Start()
	if target != nil && *target != "" {
		//TODO 连接
		fmt.Println("try connect ready ->", *target)
		<- time.After(3 * time.Second)
		fmt.Println("try connect start ->", *target)
		<- time.After(3 * time.Second)
		fmt.Println("target", target, "err", p2pservice.Connect(*target))
	}
	select {}
}
