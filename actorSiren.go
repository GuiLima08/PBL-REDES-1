package main

import (
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
	buf := make([]byte, 1024)
	for {
		n, err := tcpConn.Read(buf)
		if err != nil {
			fmt.Println("\n-!- Erro ao ler do servidor:", err)
			return
		}

		msg := strings.TrimSpace(string(buf[:n]))

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
}

func handshake() bool {
	_, err := tcpConn.Write([]byte("ACTOR/HND/--"))
	if err != nil {
		fmt.Println("-!- Erro ao enviar handshake: ", err)
		return false
	}
	buf := make([]byte, 1024)
	n, err := tcpConn.Read(buf)
	if err != nil {
		fmt.Println("-!- Erro durante o handshake:", err)
		return false
	} else if strings.TrimSpace(string(buf[:n])) != "HND/ACCEPTED" {
		fmt.Println("-!- Handshake rejeitado pelo servidor.")
		return false
	}
	fmt.Println("-!- Handshake bem-sucedido!")
	return true
}

func pulse() {
	time.Sleep(1 * time.Second)
	fmt.Println("Estado: ", state)
}

// --- NEW: Sends standard 3-part message (TYPE/COMMAND/CONTENT) ---
func feedback() {
	msg := fmt.Sprintf("ACTOR/FDB/%s", state)
	_, err := tcpConn.Write([]byte(msg))
	if err != nil {
		fmt.Println("-!- Erro ao enviar feedback: ", err)
	}
}