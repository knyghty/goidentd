// Copyright © 2013 Tom Carrick <tom@c0ck.org>
// This work is free. You can redistribute it and/or modify it under the
// terms of the Do What The Fuck You Want To Public License, Version 2,
// as published by Sam Hocevar. See the LICENSE file for more details.

package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"
)

type NoUserError struct {
	s string
}

type UnknownError struct {
	s string
}

func (e NoUserError) Error() string {
	return e.s
}

func (e UnknownError) Error() string {
	return e.s
}

func getUID(local, remote int) (u int, e error) {
	localh := strings.ToUpper(strconv.FormatInt(int64(local), 16))
	remoteh := strings.ToUpper(strconv.FormatInt(int64(remote), 16))
	file, err := os.Open("/proc/net/tcp")
	if err != nil {
		log.Print(err)
		file.Close()
		e = &UnknownError{err.Error()}
		return
	}
	r := bufio.NewReader(file)
	r.ReadString(10) // Skip header line. 10 = Newline.
	for {
		line, err := r.ReadString(10)
		if err != nil && err != io.EOF {
			log.Print(err)
			e = &UnknownError{err.Error()}
		}
		parts := strings.Split(line, " ")
		if len(parts) < 5 {
			file.Close()
			e = &NoUserError{"no user"}
			return
		}
		localf := parts[3]
		remotef := parts[4]
		localf = strings.ToUpper(strings.Split(localf, ":")[1])
		remotef = strings.ToUpper(strings.Split(remotef, ":")[1])
		if localh == localf && remoteh == remotef {
			uid, err := strconv.ParseInt(parts[10], 10, 0)
			if err != nil {
				log.Print(err)
				file.Close()
				e = &UnknownError{err.Error()}
				return
			}
			file.Close()
			return int(uid), nil
		}
	}
	e = &UnknownError{"unknown error"}
	return
}

func main() {
	ln, err := net.Listen("tcp", ":113")
	if err != nil {
		log.Fatalf("Could not start server: %s", err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Connection error: %s", err)
			continue
		}
		go func(c net.Conn) {
			c.SetReadDeadline(time.Now().Add(1 * time.Minute))
			data := make([]byte, 16)
			_, err := c.Read(data)
			if err != nil {
				log.Printf("Read error: %s", err)
				c.Close()
				return
			}
			data = bytes.Trim(data, "\r\n\x00")
			portPair := string(data)
			portPairs := strings.Split(portPair, ",")
			if len(portPairs) != 2 {
				c.Write([]byte("0 , 0 : ERROR : INVALID-PORT\n"))
				c.Close()
				return
			}
			locals := strings.Trim(portPairs[0], " ")
			local, err := strconv.Atoi(locals)
			if err != nil || local < 0 || local > 65535 {
				c.Write([]byte("0 , 0 : ERROR : INVALID-PORT\n"))
				c.Close()
				return
			}
			remotes := strings.Trim(portPairs[1], " ")
			remote, err := strconv.Atoi(remotes)
			if err != nil || remote < 0 || remote > 65535 {
				c.Write([]byte("0 , 0 : ERROR : INVALID-PORT\n"))
				c.Close()
				return
			}

			uid, err := getUID(int(local), int(remote))
			ports := locals + " , " + remotes
			if err != nil {
				if _, ok := err.(*NoUserError); ok {
					c.Write([]byte(ports + " : ERROR : NO-USER\n"))
					c.Close()
					return
				}
				if _, ok := err.(*UnknownError); ok {
					c.Write([]byte(ports + " : ERROR : UNKNOWN-ERROR\n"))
					c.Close()
					return
				}
			}
			u, err := user.LookupId(strconv.Itoa(uid))
			if err != nil {
				log.Print(err)
				c.Write([]byte(ports + " : ERROR : NO-USER\n"))
				c.Close()
				return
			}
			c.Write([]byte(ports + " : USERID : UNIX : " + u.Username + "\n"))
			c.Close()
		}(conn)
	}
}
