package main

import "github.com/navaz-alani/concord/server"

// Cipher is a ServerExtension which performs encyrption on outgoing packets.
type Cipher interface {
	server.ServerExtension
}
