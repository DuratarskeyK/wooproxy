package main

import (
	"github.com/duratarskeyk/goproxy"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"strconv"
)

var localToTransport map[string]*http.Transport
var localToTransportMu sync.Mutex
var remoteToLocal map[string]string
var remoteToLocalMu sync.RWMutex

func ConnState(c net.Conn, state http.ConnState) {
	host, _, _ := net.SplitHostPort(c.LocalAddr().String())
	if state == http.StateNew {
		remoteToLocalMu.Lock()
		remoteToLocal[c.RemoteAddr().String()] = host
		remoteToLocalMu.Unlock()
	} else if state == http.StateClosed || state == http.StateHijacked {
		remoteToLocalMu.Lock()
		delete(remoteToLocal, host)
		remoteToLocalMu.Unlock()
	}
}

func getTransportForLocalAddr(localAddr string) *http.Transport {
	localToTransportMu.Lock()
	defer localToTransportMu.Unlock()
	transport, ok := localToTransport[localAddr]
	if ok {
		return transport
	}

	transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(localAddr)},
		}).DialContext,
	}
	localToTransport[localAddr] = transport
	return transport
}

type ConnectDialError struct {}

func (err *ConnectDialError) Error() string {
	return "Something went wrong, please retry"
}

func ConnectDial(network string, addr string, ctx *goproxy.ProxyCtx) (net.Conn, error) {
	ret_err := &ConnectDialError{}
	remoteToLocalMu.RLock()
	localAddr := remoteToLocal[ctx.Req.RemoteAddr]
	remoteToLocalMu.RUnlock()
	host, port_str, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, ret_err
	}
	port, err := strconv.Atoi(port_str)
	if err != nil {
		return nil, ret_err
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, ret_err
	}
	for _, v := range addrs {
		conn, err := net.DialTCP(network, &net.TCPAddr{IP: net.ParseIP(localAddr)}, &net.TCPAddr{IP: net.ParseIP(v), Port: port})
		if err == nil {
				return conn, nil
		}
	}
	return nil, ret_err
}

func main() {
	api_addr := flag.String("api_addr", "", "Address for Proxy Api endpoint")
	api_key := flag.String("api_key", "", "Api key for the Proxy Api")
	bind_addr := flag.String("bind_port", "5432", "Port that the proxy will use")
	flag.Parse()
	if *api_addr == "" {
		fmt.Println("API address is required")
		return
	}
	if *api_key == "" {
		fmt.Println("API Key is required")
		return
	}
	localToTransport = make(map[string]*http.Transport)
	localToTransportMu = sync.Mutex{}
	remoteToLocal = make(map[string]string)
	remoteToLocalMu = sync.RWMutex{}
	proxy := goproxy.NewProxyHttpServer()
	proxy.ConnectDial = ConnectDial
	server := &http.Server{
		Addr:      ":" + *bind_addr,
		Handler:   proxy,
		ConnState: ConnState,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	auth := NewAuthorization(*api_addr, *api_key)
	proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		req := ctx.Req
		//delicious copypasta
		authHeader, ok := req.Header["Proxy-Authorization"]
		if !ok {
			return goproxy.RejectConnect, host
		}
		creds, err := base64.StdEncoding.DecodeString(strings.Split(authHeader[0], " ")[1])
		if err != nil {
			return goproxy.RejectConnect, host
		}
		remoteToLocalMu.RLock()
		localAddr := remoteToLocal[req.RemoteAddr]
		remoteToLocalMu.RUnlock()

		cred_arr := strings.Split(string(creds), ":")
		if !auth.Canlogin(localAddr, cred_arr[0], cred_arr[1]) {
			return goproxy.RejectConnect, host
		}
		return goproxy.OkConnect, host
	})
	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		deniedResponse := goproxy.NewResponse(req,
			goproxy.ContentTypeText, http.StatusProxyAuthRequired,
			"")
		deniedResponse.Header.Add("Proxy-Authenticate", "Basic realm=\"Proxy\"")
		authHeader, ok := req.Header["Proxy-Authorization"]
		if !ok {
			return req, deniedResponse
		}
		creds, err := base64.StdEncoding.DecodeString(strings.Split(authHeader[0], " ")[1])
		if err != nil {
			return req, deniedResponse
		}
		remoteToLocalMu.RLock()
		localAddr := remoteToLocal[req.RemoteAddr]
		remoteToLocalMu.RUnlock()

		cred_arr := strings.Split(string(creds), ":")
		if !auth.Canlogin(localAddr, cred_arr[0], cred_arr[1]) {
			return req, deniedResponse
		}
		tr := getTransportForLocalAddr(localAddr)
		ctx.RoundTripper = goproxy.RoundTripperFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (resp *http.Response, err error) {
			resp, err = tr.RoundTrip(req)
			return
		})
		return req, nil
	})
	log.Fatal(server.ListenAndServe())
}
