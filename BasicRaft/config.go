package braft

type Config struct {
	Servers [] ServerLocation
	Addr string 
	StartPort int
	MaxPorts int
}