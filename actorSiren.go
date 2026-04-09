package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

var ( // Variáveis globais para manter a conexão TCP e o estado atual da sirene
	tcpConn net.Conn
	state   string = "OFF"
)

func main() {
	if len(os.Args) != 3 { // Verifica se os argumentos necessários foram fornecidos (IP do servidor e porta)
		fmt.Println("Uso: go run actorSiren.go <server_ip> <port>")
		return
	}
	serverIP, port := os.Args[1], os.Args[2]

	fmt.Printf("Conectando ao servidor em %s:%s...\n", serverIP, port)
	var err error
	tcpConn, err = net.Dial("tcp", serverIP+":"+port) // Tenta estabelecer uma conexão TCP com o servidor usando os argumentos fornecidos
	if err != nil {
		fmt.Println("-!- Erro ao conectar ao servidor:", err)
		return
	}
	defer tcpConn.Close()

	if !handshake() { // Realiza o handshake inicial com o servidor para se identificar como um ator e verificar se a conexão foi aceita
		return
	}

	readLoop() // Inicia a rotina de leitura para processar mensagens recebidas do servidor, como comandos para ligar/desligar a sirene ou solicitações de feedback
	fmt.Println("\n-!- Desconectado do servidor.")
}

func readLoop() { // Função que fica em loop lendo mensagens do servidor e respondendo de acordo com o comando recebido
	scanner := bufio.NewScanner(tcpConn) // Cria um scanner para ler linhas de texto da conexão TCP

	for scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())

		switch msg {
		case "ON": // Se o comando for "ON", atualiza o estado para "ON", imprime uma mensagem indicando que a sirene foi ligada e envia um feedback para o servidor
			state = "ON"
			fmt.Println("SIREN ON")
			feedback()

		case "OFF": // Se o comando for "OFF", atualiza o estado para "OFF", imprime uma mensagem indicando que a sirene foi desligada e envia um feedback para o servidor
			state = "OFF"
			fmt.Println("SIREN OFF")
			feedback()

		case "FEEDBACK": // Se o comando for "FEEDBACK", envia o estado atual da sirene de volta para o servidor
			feedback()

		case "BYE": // Se o comando for "BYE", imprime uma mensagem indicando que o servidor solicitou a desconexão e encerra a função, o que levará ao fechamento da conexão TCP
			fmt.Println("\n-!- O servidor solicitou a desconexão.")
			return

		default: // Para qualquer comando desconhecido, imprime uma mensagem de aviso no terminal
			fmt.Printf("-!- Mensagem desconhecida do servidor: %s\n", msg)
		}
	}
	
	if err := scanner.Err(); err != nil { // Verifica se houve algum erro durante a leitura da conexão TCP e imprime uma mensagem de erro caso isso ocorra
		fmt.Println("\n-!- Erro ao ler do servidor:", err)
	}
}

func handshake() bool { // Função para realizar o handshake inicial com o servidor, enviando uma mensagem de identificação e aguardando a resposta de aceitação
	_, err := tcpConn.Write([]byte("ACTOR/HND/--\n"))
	if err != nil {
		fmt.Println("-!- Erro ao enviar handshake: ", err)
		return false
	}
	
	scanner := bufio.NewScanner(tcpConn) // Cria um scanner para ler a resposta do servidor ao handshake, esperando uma mensagem de aceitação para confirmar que a conexão foi estabelecida corretamente
	if scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())
		if msg != "HND/ACCEPTED" { // Verifica se a resposta do servidor é "HND/ACCEPTED", indicando que o handshake foi bem-sucedido; caso contrário, imprime uma mensagem de rejeição e retorna false para indicar que a conexão não foi aceita
			fmt.Println("-!- Handshake rejeitado pelo servidor.")
			return false
		}
		fmt.Println("-!- Handshake bem-sucedido!")
		return true
	}
	
	fmt.Println("-!- Erro durante o handshake: servidor fechou a conexão.")
	return false
}

func feedback() { // Função para enviar o estado atual da sirene de volta para o servidor, formatando a mensagem de acordo com o protocolo definido e tratando possíveis erros de envio
	msg := fmt.Sprintf("ACTOR/FDB/%s\n", state)
	_, err := tcpConn.Write([]byte(msg))
	if err != nil {
		fmt.Println("-!- Erro ao enviar feedback: ", err)
	}
}