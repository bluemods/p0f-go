package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/bluemods/p0f-go/p0f"
)

func main() {
	sockFile := flag.String("s", p0f.DefaultSock, fmt.Sprintf("p0f socket file, default is `%s`", p0f.DefaultSock))
	port := flag.Int("p", p0f.DefaultPort, fmt.Sprintf("HTTP API port, default is %d", p0f.DefaultPort))
	flag.Parse()

	if len(*sockFile) == 0 {
		log.Fatal("-s is not a valid file name")
	}
	if *port < 0 || *port > 0xFFFF {
		log.Fatalf("invalid port (%d)", *port)
	}
	log.Fatal(p0f.StartHttpWebServer(*sockFile, *port, p0f.DefaultIpResolver))
}
