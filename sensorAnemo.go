package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 3 { // Verifica se os argumentos necessários foram fornecidos (IP do servidor e porta)
		fmt.Println("Usage: go run sensorAnemo.go <server_ip> <port>")
		return
	}
	serverIP, port := os.Args[1], os.Args[2]

	var conn net.Conn
	var err error

	fmt.Printf("Connecting to %s:%s...\n", serverIP, port)
	
	for { // Tenta se conectar ao servidor em um loop, caso haja falha na conexão, espera 2 segundos antes de tentar novamente
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

	var ( // Variáveis para simular a variação do vento de forma realista, com uma tendência de oscilação e mudanças de velocidade ao longo do tempo
		rnd   = rand.New(rand.NewSource(time.Now().UnixNano()))
		sig   = 1.0
		value = 20.0
		cnt   = 0
		speed = 0.01
	)

	for { // Em um loop infinito, simula a variação do vento e envia os dados para o servidor a cada 100 milissegundos
		value = value + sig*speed

		msg := fmt.Sprintf("ANEMO/%.2f", value)
		_, err := conn.Write([]byte(msg))
		if err != nil {
			fmt.Println("Error sending data, dropping packet:", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if value < 10 {
			value = 10.0
			sig = -1.0
		} else if value > 30 {
			value = 30.0
			sig = -1.0
		} else {
			sig = rnd90_10(rnd, sig)
		}
		
		cnt++
		if cnt == 100 {
			cnt = 0
			speed = float64(rnd.Intn(3)+1) * 0.01
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func rnd90_10(rnd *rand.Rand, sig float64) float64 { // Função para simular uma variação aleatória do vento, onde há uma probabilidade de 90% de manter a mesma direção (sinal) e 10% de inverter a direção, criando uma oscilação mais realista
	if rnd.Float64() < 0.9 {
		return sig
	}
	return -sig
}