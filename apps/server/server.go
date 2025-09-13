package main

import (
	"flag"
	"net"
	"strings"
	"time"

	"fmt"

	"gitlab.com/donomii/frogpond"
)

func main() {
	//Get port as a command line option
	port := flag.Uint("port", 16002, "Port to listen on")
	//Provide a peer as a command line option
	peer := flag.String("peer", "", "Ip of known peer in the frogpond network")
	//Provide a name as a command line option
	name := flag.String("name", "Someone forgot to give me a name", "Name of this host")
	_ = flag.String("secret", "", "Secret to use for authentication")
	flag.Parse()
	//Start the server

	//Define time at zero
	var zero time.Time

	hs := &frogpond.HostService{GUID: fmt.Sprintf("%v:%v", *peer, *port), Ip: *peer, Name: "", Ports: []uint{*port}, Port: *port, LastSeen: zero}
	frogpond.AddHost(hs)
	frogpond.Info.Name = *name
	frogpond.Info.Services = []frogpond.Service{}

	//Get all local ip addresses
	ips, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	//Build /24 cidr addresses for each ip
	for _, ip := range ips {
		//Get ip as a string
		ipString := ip.String()
		if strings.HasPrefix(ipString, "127.") {
			continue
		}
		//If not ipv6
		if strings.Contains(ipString, ":") {
			continue
		}
		go frogpond.ScanNetwork(ipString, []uint{*port})

	}

	frogpond.StartServer(uint(*port))
}
