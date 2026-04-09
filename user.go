package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	tcpConn net.Conn
	scanner *bufio.Scanner

	sensorList []string // Lista de sensores disponíveis, atualizada a partir das mensagens recebidas do servidor
	actorList  []string // Lista de atuadores disponíveis, atualizada a partir das mensagens recebidas do servidor
	latestData string // Armazena o dado mais recente recebido do servidor para o sensor atualmente monitorado
	actorState string // Armazena o estado atual do atuador que está sendo controlado

	listLock  sync.RWMutex // Mutex para proteger o acesso às listas de sensores e atuadores
	dataLock  sync.RWMutex // Mutex para proteger o acesso às variáveis de dados mais recentes e estado do atuador
	dataTypes = map[byte][2]string{ // Mapa para traduzir os tipos de sensores
		'A': {"Anemômetro", "Km/h"},
		'F': {"Combustível", "L"},
	}
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Uso: go run user.go <server_ip> <port>")
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

	if !handshake() { // Realiza o handshake inicial com o servidor para se identificar como um cliente e verificar se a conexão foi aceita
		return
	}

	// Inicia a rotina de leitura em background para processar mensagens recebidas do servidor
	go readLoop()

	// Inicializa o scanner para ler a entrada do usuário a partir do terminal
	scanner = bufio.NewScanner(os.Stdin)

	for {
		clearScreen()
		fmt.Println("======================================")
		fmt.Println("       PAINEL DE CONTROLE CENTRAL")
		fmt.Println("======================================")
		fmt.Println(" [1] Monitorar Sensores")
		fmt.Println(" [2] Gerenciar Atuadores")
		fmt.Println(" [Q] Sair")
		fmt.Print("\n Selecione uma opção: ")

		scanner.Scan()
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		if input == "1" {
			sensorID := showSensorMenu()
			if sensorID != "" {
				streamData(sensorID)
			}
		} else if input == "2" {
			actorID := showActorMenu()
			if actorID != "" {
				actorControl(actorID)
			}
		} else if input == "Q" {
			tcpConn.Write([]byte("USER/BYE/--\n"))
			clearScreen()
			fmt.Println("Desconectado. Adeus!")
			os.Exit(0)
		}
	}
}

func readLoop() { // Função que fica em loop lendo mensagens do servidor e respondendo de acordo com o comando recebido
	netScanner := bufio.NewScanner(tcpConn)

	for netScanner.Scan() {
		msg := strings.TrimSpace(netScanner.Text())
		parts := strings.Split(msg, "/")
		switch parts[0] {
		case "LST": // Atualiza a lista de sensores disponíveis com base na mensagem recebida do servidor
			listLock.Lock()
			if msg == "LST/" {
				sensorList = []string{}
			} else {
				sensorList = strings.Split(strings.TrimPrefix(msg, "LST/"), ",")
			}
			listLock.Unlock()

		case "LSA": // Atualiza a lista de atuadores disponíveis com base na mensagem recebida do servidor
			listLock.Lock()
			if msg == "LSA/" {
				actorList = []string{}
			} else {
				actorList = strings.Split(strings.TrimPrefix(msg, "LSA/"), ",")
			}
			listLock.Unlock()

		case "DATA": // Atualiza o dado mais recente recebido do servidor para o sensor atualmente monitorado
			dataLock.Lock()
			latestData = strings.TrimPrefix(msg, "DATA/")
			dataLock.Unlock()

		case "CKS": // Atualiza o estado do atuador com base na mensagem recebida do servidor
			if len(parts) == 3 {
				dataLock.Lock()
				actorState = parts[2]
				dataLock.Unlock()
			} else {
				fmt.Printf("-!- Received malformed CKS message: %s\n", msg)
			}
		}
	}

	fmt.Println("\n-!- Desconectado do servidor.")
	os.Exit(1)
}

func showSensorMenu() string { // Exibe a lista de sensores disponíveis e permite que o usuário selecione um para monitorar em tempo real
	for {
		tcpConn.Write([]byte("USER/LST/--\n")) // Envia uma mensagem para o servidor solicitando a lista de sensores disponíveis
		time.Sleep(200 * time.Millisecond)

		clearScreen()
		fmt.Println("======================================")
		fmt.Println("       MONITOR DE SENSORES")
		fmt.Println("======================================")

		listLock.RLock()
		sensors := make([]string, len(sensorList))
		copy(sensors, sensorList)
		listLock.RUnlock()

		if len(sensors) == 0 {
			fmt.Println("\n Nenhum sensor encontrado.")
		} else {
			fmt.Println("\n SENSORES DISPONÍVEIS:")
			for i, s := range sensors {
				fmt.Printf("  [%d] %s\n", i+1, s)
			}
		}

		fmt.Println("\n--------------------------------------")
		fmt.Println(" [R] Atualizar lista")
		fmt.Println(" [B] Voltar ao Menu Principal")
		fmt.Print("\n Selecione uma opção: ")

		scanner.Scan()
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		if input == "B" {
			return ""
		}
		if input == "R" || input == "" {
			continue
		}

		choice, err := strconv.Atoi(input)
		if err == nil && choice > 0 && choice <= len(sensors) {
			return sensors[choice-1]
		}
	}
}

func showActorMenu() string { // Exibe a lista de atuadores disponíveis e permite que o usuário selecione um para controlar
	for {
		tcpConn.Write([]byte("USER/LSA/--\n")) // Envia uma mensagem para o servidor solicitando a lista de atuadores disponíveis
		time.Sleep(200 * time.Millisecond)

		clearScreen()
		fmt.Println("======================================")
		fmt.Println("       GERENCIADOR DE ATUADORES")
		fmt.Println("======================================")

		listLock.RLock()
		actors := make([]string, len(actorList))
		copy(actors, actorList)
		listLock.RUnlock()

		if len(actors) == 0 {
			fmt.Println("\n Nenhum atuador encontrado.")
		} else {
			fmt.Println("\n ATUADORES DISPONÍVEIS:")
			for i, a := range actors {
				fmt.Printf("  [%d] %s\n", i+1, a)
			}
		}

		fmt.Println("\n--------------------------------------")
		fmt.Println(" [R] Atualizar lista")
		fmt.Println(" [B] Voltar ao Menu Principal")
		fmt.Print("\n Selecione uma opção: ")

		scanner.Scan()
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		if input == "B" {
			return ""
		}
		if input == "R" || input == "" {
			continue
		}

		choice, err := strconv.Atoi(input)
		if err == nil && choice > 0 && choice <= len(actors) {
			return actors[choice-1]
		}
	}
}

func streamData(sensorID string) { // Envia uma mensagem para o servidor solicitando o início do streaming de dados para o sensor selecionado
	tcpConn.Write([]byte("USER/GET/" + sensorID + "\n"))

	dataLock.Lock()
	latestData = "CONECTANDO..."
	dataLock.Unlock()

	stopChan := make(chan bool)
	go func() {
		scanner.Scan()
		stopChan <- true
	}()

	senType := dataTypes[sensorID[len(sensorID)-1]] // Extrai o tipo do sensor a partir do ID do sensor e usa o mapa dataTypes para obter uma descrição legível

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for { // Loop para atualizar a tela com os dados mais recentes do sensor a cada 500 milissegundos
		select {
		case <-stopChan: // Se o usuário sinalizar para parar o monitoramento, envia uma mensagem para o servidor solicitando o fim do streaming de dados
			tcpConn.Write([]byte("USER/DCN/--\n"))
			return

		case <-ticker.C: // A cada 500 milissegundos, atualiza a tela com os dados mais recentes do sensor
			dataLock.RLock()
			data := latestData
			dataLock.RUnlock()

			clearScreen()
			fmt.Println("======================================")
			fmt.Printf("SENSOR: \"%s\" (%s)\n", sensorID, senType[0])
			fmt.Println("======================================")

			parts := strings.Split(data, "/")
			if len(parts) == 2 { // Se o dado recebido tiver o formato esperado (valor/tipo), exibe o valor do sensor
				fmt.Printf("    %s %s\n", parts[1], senType[1])
			} else { // Caso contrário, exibe o dado recebido como está
				fmt.Printf("\n    %s\n", data)
			}

			fmt.Println("\n======================================")
			fmt.Println(" [Enter] Voltar ao menu")
		}
	}
}

func actorControl(actorID string) { // Loop para exibir o menu de controle do atuador selecionado
	for {
		tcpConn.Write([]byte("USER/CKS/" + actorID + "\n"))
		time.Sleep(200 * time.Millisecond)

		clearScreen()
		fmt.Println("======================================")
		fmt.Printf(" ATUADOR: %s\n", actorID)

		dataLock.RLock()
		st := strings.Split(actorState, ",") // Lê o estado atual do atuador a partir da variável protegida por mutex
		dataLock.RUnlock()

		if len(st) == 2 {
			fmt.Printf(" ESTADO ATUAL: %s\n", st[0])
			fmt.Printf(" Última atualização: %s\n", st[1])
			fmt.Println("======================================")
			if st[0] == "ON" || st[0] == "OFF" {
				fmt.Println(" [1] Ligar (ON)")
				fmt.Println(" [2] Desligar (OFF)")
				fmt.Println("--------------------------------------")
			}
		} else {
			fmt.Println("Erro. Pressione R para atualizar ou tente novamente mais tarde.")
			fmt.Println("--------------------------------------")
		}

		fmt.Println(" [R] Atualizar Estado")
		fmt.Println(" [B] Voltar")
		fmt.Print("\n Selecione uma opção: ")

		scanner.Scan()
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		if input == "1" {
			tcpConn.Write([]byte("USER/SST/" + actorID + "|ON\n"))
			time.Sleep(200 * time.Millisecond)
		} else if input == "2" {
			tcpConn.Write([]byte("USER/SST/" + actorID + "|OFF\n"))
			time.Sleep(200 * time.Millisecond)
		} else if input == "B" {
			return
		}
	}
}

func handshake() bool {
	tcpConn.Write([]byte("USER/HND/--\n"))

	netScanner := bufio.NewScanner(tcpConn)
	if netScanner.Scan() {
		msg := strings.TrimSpace(netScanner.Text())
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

// --- CROSS-PLATFORM CLEAR SCREEN ---
func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}
