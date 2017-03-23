package main

import (
	"fmt"
	"net/http"
)

type Authorization struct {
	denied   *HashTtl
	allowed  *HashTtl
	api_addr string
	api_key  string
}

func NewAuthorization(api_addr string, api_key string) *Authorization {
	return &Authorization{denied: NewHashTtl(60),
		allowed:  NewHashTtl(360),
		api_addr: api_addr,
		api_key:  api_key}
}

func (a *Authorization) auth_request(ip string, login string, password string) bool {
	uri := fmt.Sprintf("%v/ip/%v/can_login?proxy_login=%v&proxy_password=%v", a.api_addr, ip, login, password)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth("api", a.api_key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var body []byte = make([]byte, 3)
	_, err = resp.Body.Read(body)
	if string(body) == "yes" {
		return true
	} else {
		return false
	}
}

func (a *Authorization) Canlogin(ip string, login string, password string) bool {
	key := fmt.Sprintf("%v\x00%v\x00%v", ip, login, password)
	_, p := a.denied.get(key)
	if p {
		return false
	}
	_, p = a.allowed.get(key)
	if p {
		return true
	}
	can_login := a.auth_request(ip, login, password)
	if can_login {
		a.allowed.set(key, true)
	} else {
		a.denied.set(key, true)
	}
	return can_login
}
