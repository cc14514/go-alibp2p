package alibp2p

import "context"

type Config struct {
	//ctx context.Context, homedir string, port int, bootnodes []string
	Ctx       context.Context
	Homedir   string
	Port      int
	Bootnodes []string
	Discover  bool
}
