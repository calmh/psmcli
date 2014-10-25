// psmcli
// Copyright (C) 2014 Procera Networks, Inc.

package main

import (
	"encoding/json"
	"net"
)

const (
	CodeAccessDenied = -20001
)

type command struct {
	ID     int           `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

type response struct {
	ID     int
	Result interface{}
	Error  struct {
		Code    int
		Message string
	}
}

type connection struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

func newConnection(addr string) (*connection, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	dec.UseNumber()

	return &connection{
		conn: conn,
		enc:  enc,
		dec:  dec,
	}, nil
}

func (c *connection) run(cmd command) (response, error) {
	err := c.enc.Encode(cmd)
	if err != nil {
		return response{}, err
	}

	var res response
	err = c.dec.Decode(&res)
	if err != nil {
		return response{}, err
	}

	return res, nil
}

type smdResponse struct {
	Result struct {
		Services map[string]smdService
	}
}

type smdService struct {
	Name       string
	Parameters []smdParameter
}

type smdParameter struct {
	Name     string
	Optional bool
	Type     string
}

func (c *connection) smd() (smdResponse, error) {
	err := c.enc.Encode(command{Method: "system.smd"})
	if err != nil {
		return smdResponse{}, err
	}

	var res smdResponse
	err = c.dec.Decode(&res)
	if err != nil {
		return smdResponse{}, err
	}

	return res, nil
}
