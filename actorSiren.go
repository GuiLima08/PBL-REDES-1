package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

var (
	tcpConn net.Conn
	state   string = "OFF"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Uso: go run actorSiren.go <server_ip> <port>")
		return
	}
	serverIP, port := os.Args[1], os.Args[2]

	fmt.Printf("Conectando ao servidor em %s:%s...\n", serverIP, port)
	var err error
	tcpConn, err = net.Dial("tcp", serverIP+":"+port)
	if err != nil {
		fmt.Println("-!- Erro ao conectar ao servidor:", err)
		return
	}
	defer tcpConn.Close()

	if !handshake() {
		return
	}

	readLoop()
	fmt.Println("\n-!- Desconectado do servidor.")
}

func readLoop() {
	scanner := bufio.NewScanner(tcpConn)

	for scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())

		switch msg {
		case "ON":
			state = "ON"
			fmt.Println("SIREN ON")
			feedback()

		case "OFF":
			state = "OFF"
			fmt.Println("SIREN OFF")
			feedback()

		case "FEEDBACK":
			feedback()

		case "BYE":
			fmt.Println("\n-!- O servidor solicitou a desconexão.")
			return

		default:
			fmt.Printf("-!- Mensagem desconhecida do servidor: %s\n", msg)
		}
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Println("\n-!- Erro ao ler do servidor:", err)
	}
}

func handshake() bool {
	_, err := tcpConn.Write([]byte("ACTOR/HND/--\n"))
	if err != nil {
		fmt.Println("-!- Erro ao enviar handshake: ", err)
		return false
	}
	
	scanner := bufio.NewScanner(tcpConn)
	if scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())
		if msg != "HND/ACCEPTED" {
			fmt.Println("-!- Handshake rejeitado pelo servidor.")
			return false
		}
		fmt.Println("-!- Handshake bem-sucedido!")
		return true
	}
	
	fmt.Println("-!- Erro durante o handshake: servidor fechou a conexão.")
	return false
}

func pulse() {
	time.Sleep(1 * time.Second)
	fmt.Println("Estado: ", state)
}

func feedback() {
	msg := fmt.Sprintf("ACTOR/FDB/%s\n", state)
	_, err := tcpConn.Write([]byte(msg))
	if err != nil {
		fmt.Println("-!- Erro ao enviar feedback: ", err)
	}
}