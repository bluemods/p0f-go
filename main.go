package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/bluemods/p0f-go/p0f"
)

var (
	defaultPort = 38749
	defaultSock = "/tmp/p0f-mtu.sock"
)

func main() {
	sockFile := flag.String("s", defaultSock, fmt.Sprintf("p0f socket file, default is `%s`", defaultSock))
	port := flag.Int("p", defaultPort, fmt.Sprintf("HTTP API port, default is %d", defaultPort))
	flag.Parse()

	if len(*sockFile) == 0 {
		log.Fatal("-s is not a valid file name")
	}
	if *port < 0 || *port > 0xFFFF {
		log.Fatalf("invalid port (%d)", *port)
	}

	p, err := p0f.New(*sockFile)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Started with sock '%s' on port %d\n", *sockFile, *port)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensures that a new connection is attempted every time by a browser,
		// which results in faster verdict changes
		w.Header().Set("Connection", "close")

		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			http.Error(w, "", http.StatusMethodNotAllowed)
			return
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "invalid source address", http.StatusBadRequest)
			return
		}

		userIP := net.ParseIP(ip)
		if userIP == nil {
			http.Error(w, "invalid source address", http.StatusBadRequest)
			return
		}

		response, err := p.Query(userIP)
		if err != nil {
			http.Error(w, "query error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		enc := json.NewEncoder(w)

		// Pretty print (example: http://localhost:38749/?p=1)
		if r.URL.Query().Has("p") {
			enc.SetIndent("", " ")
		}
		enc.Encode(response)
	})))
}
