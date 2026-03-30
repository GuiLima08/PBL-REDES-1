package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run sensorFuel.go <server_ip> <port>")
		return
	}
	serverIP, port := os.Args[1], os.Args[2]

	var conn net.Conn
	var err error

	fmt.Printf("Connecting to %s:%s...\n", serverIP, port)
	
	for {
		conn, err = net.Dial("udp", serverIP+":"+port)
		if err != nil {
			fmt.Printf("Connection failed (%v). Retrying in 2 seconds...\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	defer conn.Close()
	fmt.Println("Successfully connected!")

	fuel := 100.0000
	
	for {
		fuel -= (fuel * 0.0001)
		if fuel < 1 {
			fuel = 1
		}
		
		msg := fmt.Sprintf("FUEL/%.2f", fuel)
		_, err := conn.Write([]byte(msg))
		if err != nil {
			fmt.Println("Error sending data, dropping packet:", err)
			time.Sleep(100 * time.Millisecond)
			continue 
		}
		
		time.Sleep(100 * time.Millisecond) // Send data every 0.1 seconds
	}
}