package typhoon

import (
	"bufio"
	"log"
	"math/rand"
	"net"
	"reflect"
	"time"
	"os"
	"syscall"
	"os/signal"
	"fmt"
	"strings"
)

var (
	started	= false
)

type Core struct {
	connCounter      int
	eventHandlers    map[reflect.Type][]EventCallback
	brand            string
	rootCommand      CommandNode
	compiledCommands []commandNode
}

func Init() *Core {
	initConfig()
	initPackets()
	initHacks()
	c := &Core{
		0,
		make(map[reflect.Type][]EventCallback),
		"typhoon",
		CommandNode{
			commandNodeTypeRoot,
			nil,
			nil,
			nil,
			"",
			nil,
		},
		nil,
	}
	c.compileCommands()
	return c
}

func (c *Core) Start() {
	if started {
		log.Fatal("Server already started!")
		return
	}
	addShutdownHook(func() {
		c.Stop()
	})
	ln, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		log.Fatal(err)
	}
	started = true
	log.Println("Server launched on port", config.ListenAddress)
	go c.initConsole()
	go c.keepAlive()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
		} else {
			c.connCounter += 1
			go c.handleConnection(conn, c.connCounter)
		}
	}
}

func (c *Core) Stop() {
	if !started {
		log.Fatal("Server is not started!")
		return
	}
	log.Println("Stopping server...")
	started = false
	c.CallEvent(&ServerStopEvent{})
	playersMutex.Lock()
	for _, player := range players {
		player.Kick("Server is closed")
	}
	playersMutex.Unlock()
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

func (c *Core) SetBrand(brand string) {
	c.brand = brand
}

func (c *Core) GetOnlinePlayers() map[int]*Player {
	return players
}

func (c *Core) keepAlive() {
	r := rand.New(rand.NewSource(15768735131534))
	keepalive := &PacketPlayKeepAlive{
		Identifier: 0,
	}
	for {
		playersMutex.Lock()
		for _, player := range players {
			if player.state == PLAY {
				if player.keepalive != 0 {
					fmt.Println("KICK DU JOUEUR POUR TIME OUT")
					player.Kick("Timed out")
				}

				id := int(r.Int31())
				keepalive.Identifier = id
				player.keepalive = id
				player.WritePacket(keepalive)
			}
		}
		playersMutex.Unlock()
		time.Sleep(5000000000)
	}
}

func (c *Core) handleConnection(conn net.Conn, id int) {
	log.Printf("%s(#%d) connected.", conn.RemoteAddr().String(), id)

	player := &Player{
		core:     c,
		id:       id,
		conn:     conn,
		state:    HANDSHAKING,
		protocol: V1_10,
		io: &ConnReadWrite{
			rdr: bufio.NewReader(conn),
			wtr: bufio.NewWriter(conn),
		},
		inaddr: InAddr{
			"",
			0,
		},
		name:        "",
		uuid:        "d979912c-bb24-4f23-a6ac-c32985a1e5d3",
		keepalive:   0,
		compression: false,
	}

	for {
		_, err := player.ReadPacket()
		if err != nil {
			break
		}
	}

	player.core.CallEvent(&PlayerQuitEvent{player})
	player.unregister()
	conn.Close()
	log.Printf("%s(#%d) disconnected.", conn.RemoteAddr().String(), id)
}

func (c *Core) initConsole() {
	for {
		consoleReader := bufio.NewReader(os.Stdin)
		fmt.Print("> ")
		input, _ := consoleReader.ReadString('\n')
		input = strings.ToLower(input)
		// ToDo: commands
		if strings.HasPrefix(input, "stop") {
			c.Stop()
			os.Exit(0)
		}
	}
}

func addShutdownHook(functions ...func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for {
			s := <-signals
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				for _, f := range functions {
					f()
				}
			}
		}
	}()
}
