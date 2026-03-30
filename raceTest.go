package main

import (
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

	// Vamos simular 10 usuários atirando comandos ao mesmo tempo
	numClients := 10
	
	// Adicionamos os 10 na fila de espera geral, e 1 trava na "arma de largada"
	wg.Add(numClients)
	startingGun.Add(1)

	fmt.Printf("Conectando %d clientes ao servidor...\n", numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			// 1. Cada robô abre sua própria conexão TCP
			conn, err := net.Dial("tcp", serverIP+":"+port)
			if err != nil {
				fmt.Println("Erro ao conectar:", err)
				return
			}
			defer conn.Close()

			// 2. Faz o Handshake silenciosamente
			conn.Write([]byte("USER/HND/--"))
			buf := make([]byte, 1024)
			conn.Read(buf) 

			// 3. Define o comando. Metade vai mandar ON, metade vai mandar OFF
			comando := "OFF"
			if clientID%2 == 0 {
				comando = "ON"
			}
			msg := fmt.Sprintf("USER/SST/%s|%s", actorID, comando)

			// 4. PREPARAR... (Essa linha congela a goroutine até o startingGun dar o Done)
			startingGun.Wait()

			// 5. FOGO! (Todos chegam aqui no mesmo microssegundo)
			conn.Write([]byte(msg))
			fmt.Printf(" [Robô %d] atirou: %s\n", clientID, comando)

		}(i)
	}

	// Dá 1 segundo pro sistema operacional estabilizar as 10 conexões TCP
	time.Sleep(1 * time.Second)

	fmt.Println("\n3... 2... 1... FOGO!")
	// Isso libera os 10 robôs simultaneamente
	startingGun.Done() 

	// Espera os 10 robôs terminarem de enviar
	wg.Wait()
	fmt.Println("\nTeste de estresse concluído! Verifique o log do Servidor e do Atuador.")
}