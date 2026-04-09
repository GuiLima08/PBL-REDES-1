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

	var wg sync.WaitGroup // WaitGroup para aguardar a conclusão de todas as goroutines dos clientes antes de finalizar o programa
	var startingGun sync.WaitGroup // WaitGroup para sincronizar o início simultâneo de todas as goroutines dos clientes, garantindo que todos enviem seus comandos ao mesmo tempo para criar uma situação de corrida no servidor

	numClients := 100
	
	wg.Add(numClients)
	startingGun.Add(1)

	fmt.Printf("Conectando %d clientes ao servidor...\n", numClients)

	for i := 0; i < numClients; i++ { // Para cada cliente, inicia uma goroutine que se conecta ao servidor
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", serverIP+":"+port)
			if err != nil {
				fmt.Println("Erro ao conectar:", err)
				return
			}
			defer conn.Close()

			conn.Write([]byte("USER/HND/--\n")) // Envia uma mensagem de handshake para o servidor para se identificar como um cliente
			scanner := bufio.NewScanner(conn)
			scanner.Scan() 

			comando := "OFF" // Define o comando a ser enviado, onde metade dos clientes enviará "ON" e a outra metade enviará "OFF" para criar uma situação de corrida no servidor
			if clientID%2 == 0 {
				comando = "ON"
			}

			msg := fmt.Sprintf("USER/SST/%s|%s\n", actorID, comando) 

			startingGun.Wait() // Aguarda o sinal do startingGun para garantir que todas as goroutines dos clientes enviem seus comandos ao mesmo tempo, criando uma situação de corrida no servidor

			conn.Write([]byte(msg)) // Envia o comando para o servidor, onde a ordem de chegada dos comandos pode variar devido à natureza concorrente das goroutines, criando uma situação de corrida no servidor
			fmt.Printf(" [Robô %d] atirou: %s\n", clientID, comando)

		}(i)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("\n3... 2... 1... FOGO!")
	startingGun.Done() // Sinaliza o startingGun para permitir que todas as goroutines dos clientes enviem seus comandos ao mesmo tempo, criando uma situação de corrida no servidor

	wg.Wait()
	fmt.Println("\nTeste de estresse concluído! Verifique o log do Servidor e do Atuador.")
}