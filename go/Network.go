package main

import (
	"net"
	"strconv"
	"log"
	"io"
	"github.com/golang/protobuf/proto"
)

type UUID string

type NodeDescription struct {
	IP string
	port string
	isNAT bool
}

type RoutingTable struct {
	hosts map[UUID]net.Conn
}

var nodeDesc NodeDescription
var routingTable RoutingTable

func fillRoutingTable(connection net.Conn) {
}

func clientRoutine() {
	nodeDesc.isNAT, _ = checkNAT()
	nodeDesc.IP, _ = getRemoteIP()
	nodeDesc.port = strconv.Itoa(defaultPort)

	var connection net.Conn
	for _, ip := range KnownHosts {
		addr := ip + ":" + strconv.Itoa(defaultPort)
		conn, err := net.Dial("tcp4", addr)
		if err != nil {
			log.Println(err)
			continue
		}
		connection = conn
		break
	}
	log.Printf("NAT: %t\n", nodeDesc.isNAT)
	fillRoutingTable(connection)

	message := &Message{
		TYPE: Message_JOIN,
		Payload: &Message_Join_{
			&Message_Join{
				IP: nodeDesc.IP,
				IsNAT: nodeDesc.isNAT,
				Port: nodeDesc.port,
			},
		},
	}
	data, _ := proto.Marshal(message)
	connection.Write(data)

	defer connection.Close()
}

func serverRoutine(port int, terminate chan struct{}) {
	listener, err := net.Listen("tcp4", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Listeninig at port %d failed, %s", port, err)
		return
	}
	defer listener.Close()
	log.Printf("Listeninig at port: %d", port)
	newConnection := make(chan net.Conn)

	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			newConnection <- c
		}
	}()

	terminateClients := make(chan struct{})

	for {
		select {
		case <-terminate:
			log.Println("Terminating listener")
			close(terminateClients)
			return
		case conn := <-newConnection:
			log.Println("New connection at:", conn.RemoteAddr().String())
			go clientHandler(conn, terminateClients)
		}
	}
}

func clientHandler(conn net.Conn, done chan struct{}) {
	defer conn.Close()
	buffer := make([]byte, 12000)
	messageChannel := make(chan Message)
	go func() {
		for {
			message := &Message{}
			n, err := conn.Read(buffer)
			if err == io.EOF {
				return
			}
			if err := proto.Unmarshal(buffer[:n], message); err != nil {
				log.Fatalln("Unable to read message.", err)
				continue
			}
			messageChannel <- *message
		}
	}()

	for {
		select {
		case <-done:
			log.Println("Terminating connection with client:", conn.RemoteAddr().String())
			return
		case message := <-messageChannel:
			log.Printf("Received message of type %v: %v\n", message.TYPE.String(), message.String())
		}
	}
}
