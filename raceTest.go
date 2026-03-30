package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Uso: go run racetest.go <server_ip> <port> <ator_id>")
		fmt.Println("Ex: go run racetest.go 127.0.0.1 8080 127.0.0.1:59000")
		return
	}
	serverIP, port, actorID := os.Args[1], os.Args[2], os.Args[3]

	var wg sync.WaitGroup
	var startingGun sync.WaitGroup 

	numClients := 10
	
	wg.Add(numClients)
	startingGun.Add(1)

	fmt.Printf("Conectando %d clientes ao servidor...\n", numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", serverIP+":"+port)
			if err != nil {
				fmt.Println("Erro ao conectar:", err)
				return
			}
			defer conn.Close()

			conn.Write([]byte("USER/HND/--\n"))
			scanner := bufio.NewScanner(conn)
			scanner.Scan() 

			comando := "OFF"
			if clientID%2 == 0 {
				comando = "ON"
			}
			// ADICIONADO \n NO DISPARO
			msg := fmt.Sprintf("USER/SST/%s|%s\n", actorID, comando)

			startingGun.Wait()

			conn.Write([]byte(msg))
			fmt.Printf(" [Robô %d] atirou: %s\n", clientID, comando)

		}(i)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("\n3... 2... 1... FOGO!")
	startingGun.Done() 

	wg.Wait()
	fmt.Println("\nTeste de estresse concluído! Verifique o log do Servidor e do Atuador.")
}