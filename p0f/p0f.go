package p0f

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

const (
	requestChanSize = 1024 // Should be good enough for almost any use case

	requestSize  = 21
	responseSize = 44 + (32 * 6)

	ipv4Dword = 4
	ipv6Dword = 6

	resultBadQuery = 0x00
	resultOk       = 0x10
	resultNoMatch  = 0x20

	p0fStrMax = 32

	magicBytesSend = uint32(0x50304601)
	magicBytesRcv  = uint32(0x50304602)
)

type P0f struct {
	conn         net.Conn
	requestQueue chan *p0fRequest
	shutdown     *atomic.Bool
}

type p0fRequest struct {
	ip net.IP
	wg *sync.WaitGroup

	response P0fResponse
	err      error
}

type P0fResponse struct {
	Ip         string  `json:"ip"`         // IP address
	FirstSeen  uint32  `json:"firstSeen"`  // First seen (unix time)
	LastSeen   uint32  `json:"lastSeen"`   // Last seen (unix time)
	TotalCount uint32  `json:"totalCount"` // Total connections seen
	UptimeMin  uint32  `json:"uptimeMin"`  // Last uptime (minutes)
	UpModDays  uint32  `json:"upModDays"`  // Uptime modulo (days)
	LastNat    uint32  `json:"lastNat"`    // NAT / LB last detected (unix time)
	LastChg    uint32  `json:"lastChg"`    // OS chg last detected (unix time)
	Distance   uint16  `json:"distance"`   // System distance
	BadSw      byte    `json:"badSW"`      // Host is lying about U-A / Server
	OsMatchQ   byte    `json:"osMatchQ"`   // Match quality
	OsName     *string `json:"osName"`     // Name of detected OS
	OsFlavor   *string `json:"osFlavor"`   // Flavor of detected OS
	HttpName   *string `json:"httpName"`   // Name of detected HTTP app
	HttpFlavor *string `json:"httpFlavor"` // Flavor of detected HTTP app
	LinkMtu    uint16  `json:"linkMtu"`    // Link MTU value
	LinkType   *string `json:"linkType"`   // Link type
	Language   *string `json:"language"`   // Language
}

// unixSocketFile is the path to the UNIX socket file.
// This is opened when p0f is started (-s argument)
func New(unixSocketFile string) (*P0f, error) {
	conn, err := net.Dial("unix", unixSocketFile)
	if err != nil {
		return nil, err
	}
	p0f := &P0f{
		conn:         conn,
		requestQueue: make(chan *p0fRequest, requestChanSize),
		shutdown:     &atomic.Bool{},
	}
	go p0f.start()
	return p0f, nil
}

// Queries p0f for the given IP address.
// This function blocks the calling goroutine until completed.
func (p *P0f) Query(ip net.IP) (response P0fResponse, err error) {
	if p.shutdown.Load() {
		err = errors.New("P0f::Shutdown previously called")
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	request := &p0fRequest{ip: ip, wg: wg}

	select {
	case p.requestQueue <- request:
		wg.Wait() // wait for request to finish
		response, err = request.response, request.err
		return
	default:
		return response, errors.New("requestQueue at capacity")
	}
}

// Shut down p0f. After this, calls to Query will fail.
// This should only be called once. Subsequent calls to Shutdown have no effect.
func (p *P0f) Shutdown() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("error in Shutdown:", r)
		}
	}()
	if p.shutdown.CompareAndSwap(false, true) {
		close(p.requestQueue)
	}
}

// Long running background routine that processes requests
// and delivers them back to waiting goroutines.
func (p *P0f) start() {
	defer p.conn.Close()

	for !p.shutdown.Load() {
		request, ok := <-p.requestQueue
		if !ok {
			// Channel closed, exit
			return
		}

		func() {
			defer request.wg.Done()
			if err := p.writeRequest(request); err != nil {
				request.err = err
				return
			}
			request.response, request.err = p.readResponse(request.ip.String())
		}()
	}
}

func (p *P0f) writeRequest(request *p0fRequest) (err error) {
	buffer := [requestSize]byte{}
	binary.NativeEndian.PutUint32(buffer[0:4], magicBytesSend)

	if ip4 := request.ip.To4(); ip4 != nil {
		buffer[4] = ipv4Dword
		for i, b := range ip4 {
			buffer[5+i] = b
		}
	} else {
		buffer[4] = ipv6Dword
		for i, b := range request.ip.To16() {
			buffer[5+i] = b
		}
	}
	_, err = p.conn.Write(buffer[:])
	return
}

func (p *P0f) readResponse(ip string) (resp P0fResponse, err error) {
	// Temp struct to avoid returning Magic and Status (which are always the same on success),
	// as well as removing null terminators from the strings
	var r struct {
		Magic      uint32          // Must be magicBytesRcv
		Status     uint32          // result*
		FirstSeen  uint32          // First seen (unix time)
		LastSeen   uint32          // Last seen (unix time)
		TotalCount uint32          // Total connections seen
		UptimeMin  uint32          // Last uptime (minutes)
		UpModDays  uint32          // Uptime modulo (days)
		LastNat    uint32          // NAT / LB last detected (unix time)
		LastChg    uint32          // OS chg last detected (unix time)
		Distance   uint16          // System distance
		BadSw      byte            // Host is lying about U-A / Server
		OsMatchQ   byte            // Match quality
		OsName     [p0fStrMax]byte // Name of detected OS
		OsFlavor   [p0fStrMax]byte // Flavor of detected OS
		HttpName   [p0fStrMax]byte // Name of detected HTTP app
		HttpFlavor [p0fStrMax]byte // Flavor of detected HTTP app
		LinkMtu    uint16          // Link MTU value
		LinkType   [p0fStrMax]byte // Link type
		Language   [p0fStrMax]byte // Language
	}

	responseBytes := make([]byte, responseSize)

	if _, err = p.conn.Read(responseBytes); err != nil {
		return
	}
	if err = binary.Read(bytes.NewReader(responseBytes), binary.NativeEndian, &r); err != nil {
		return
	}
	if r.Magic != magicBytesRcv {
		err = errors.New("invalid magic bytes in response")
		return
	}
	switch r.Status {
	case resultOk:
		resp = P0fResponse{
			Ip:         ip,
			FirstSeen:  r.FirstSeen,
			LastSeen:   r.LastSeen,
			TotalCount: r.TotalCount,
			UptimeMin:  r.UptimeMin,
			UpModDays:  r.UpModDays,
			LastNat:    r.LastNat,
			LastChg:    r.LastChg,
			Distance:   r.Distance,
			BadSw:      r.BadSw,
			OsMatchQ:   r.OsMatchQ,
			OsName:     trstr(r.OsName),
			OsFlavor:   trstr(r.OsFlavor),
			HttpName:   trstr(r.HttpName),
			HttpFlavor: trstr(r.HttpFlavor),
			LinkMtu:    r.LinkMtu,
			LinkType:   trstr(r.LinkType),
			Language:   trstr(r.Language),
		}
		return
	case resultBadQuery:
		err = errors.New("bad query")
	case resultNoMatch:
		err = errors.New("no match")
	default:
		err = fmt.Errorf("unknown response code %d", r.Status)
	}
	return
}

func trstr(cStr [p0fStrMax]byte) *string {
	// if first byte is null the string is null
	if cStr[0] == 0 {
		return nil
	}
	for i := 1; i < len(cStr); i++ {
		if cStr[i] == 0 {
			// return new string with null bytes ignored
			goStr := string(cStr[0:i])
			return &goStr
		}
	}
	// All 32 bytes are not null, return full 32 char String
	goStr := string(cStr[:])
	return &goStr
}
