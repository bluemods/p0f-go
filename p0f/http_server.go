package p0f

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
)

var (
	DefaultPort       = 38749
	DefaultSock       = "/tmp/p0f-mtu.sock"
	DefaultIpResolver = func(r *http.Request) string {
		return r.RemoteAddr
	}
)

// StartHttpWebServer
//
// Starts the web server that creates a p0f instance with the given sockFile.
// It listens to HTTP queries on the given port and
// uses ipResolver to determine what IP address is queried.
//
// For ipResolver, you should use DefaultIpResolver in almost all cases,
// as if you put this behind a CDN, you will be analyzing
// TCP signatures of the CDN itself instead of
// the connecting client, which is usually not what you want.
//
// If the p0f instance cannot be created, an error is returned.
//
// Otherwise, the HTTP webserver is opened on the given port
// and the function blocks until an error occurs.
//
// The error returned is always non-nil.
func StartHttpWebServer(sockFile string, port int, ipResolver func(r *http.Request) string) error {
	p, err := New(sockFile)
	if err != nil {
		return err
	}
	log.Printf("Started with sock '%s' on port %d\n", sockFile, port)

	return http.ListenAndServe(fmt.Sprintf(":%d", port), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensures that a new connection is attempted every time by a browser,
		// which results in faster verdict changes
		w.Header().Set("Connection", "close")

		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			http.Error(w, "", http.StatusMethodNotAllowed)
			return
		}

		ip, _, err := net.SplitHostPort(ipResolver(r))
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
	}))
}
