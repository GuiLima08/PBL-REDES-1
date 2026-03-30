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
	tcpConn    net.Conn
	scanner    *bufio.Scanner
	
	// App State
	sensorList   []string
	actorList    []string
	latestData   string
	actorState   string // Holds the fetched state of the current actor

	listLock     sync.RWMutex
	dataLock     sync.RWMutex
	dataTypes	 = map[string][2]string{
		"ANEMO": {"Anemômetro", "Km/h"},
		"FUEL":  {"Combustível", "L"},
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

	if !handshake() {
		return
	}

	// Start the background network reader
	go readLoop()

	// Initialize the keyboard scanner
	scanner = bufio.NewScanner(os.Stdin)

	// --- THE MOTHER MENU LOOP ---
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
			tcpConn.Write([]byte("USER/BYE/--"))
			clearScreen()
			fmt.Println("Desconectado. Adeus!")
			os.Exit(0)
		}
	}
}

// --- BACKGROUND NETWORK LISTENER ---
func readLoop() {
	buf := make([]byte, 1024)
	for {
		n, err := tcpConn.Read(buf)
		if err != nil {
			fmt.Println("\n-!- Desconectado do servidor.")
			os.Exit(1)
		}

		msg := strings.TrimSpace(string(buf[:n]))

		// Update state safely
		if strings.HasPrefix(msg, "LST/") {
			listLock.Lock()
			if msg == "LST/" {
				sensorList = []string{}
			} else {
				sensorList = strings.Split(strings.TrimPrefix(msg, "LST/"), ",")
			}
			listLock.Unlock()

		} else if strings.HasPrefix(msg, "LSA/") {
			listLock.Lock()
			if msg == "LSA/" {
				actorList = []string{}
			} else {
				actorList = strings.Split(strings.TrimPrefix(msg, "LSA/"), ",")
			}
			listLock.Unlock()

		} else if strings.HasPrefix(msg, "DATA/") {
			dataLock.Lock()
			latestData = strings.TrimPrefix(msg, "DATA/")
			dataLock.Unlock()

		} else if strings.HasPrefix(msg, "CKS/") {
			// Expected: CKS/actorIP/ON
			parts := strings.Split(msg, "/")
			if len(parts) == 3 {
				dataLock.Lock()
				actorState = parts[2]
				dataLock.Unlock()
			}
		}
	}
}

// --- SCREEN 1A: SENSOR LIST ---
func showSensorMenu() string {
	for {
		tcpConn.Write([]byte("USER/LST/--"))
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

// --- SCREEN 1B: ACTOR LIST ---
func showActorMenu() string {
	for {
		tcpConn.Write([]byte("USER/LSA/--"))
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

// --- SCREEN 2A: SENSOR LIVE STREAM ---
func streamData(sensorID string) {
	tcpConn.Write([]byte("USER/GET/" + sensorID))
	
	dataLock.Lock()
	latestData = "CONECTANDO..."
	dataLock.Unlock()

	stopChan := make(chan bool)
	go func() {
		scanner.Scan()
		stopChan <- true
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			tcpConn.Write([]byte("USER/DCN/--"))
			return 

		case <-ticker.C:
			dataLock.RLock()
			data := latestData
			dataLock.RUnlock()

			clearScreen()
			fmt.Println("======================================")
			fmt.Printf("SENSOR: \"%s\" (%s)\n", sensorID, dataTypes[sensorID][0])
			fmt.Println("======================================")
			
			parts := strings.Split(data, "/")
			if len(parts) == 2 {
				fmt.Printf("    %s%s\n", parts[1], dataTypes[sensorID][1])
			} else {
				fmt.Printf("\n    %s\n", data)
			}

			fmt.Println("\n======================================")
			fmt.Println(" [Enter] Voltar ao menu")
		}
	}
}

// --- SCREEN 2B: ACTOR CONTROL PANEL ---
func actorControl(actorID string) {
	for {
		// Ask the server for the current state
		tcpConn.Write([]byte("USER/CKS/" + actorID))
		time.Sleep(200 * time.Millisecond) // Wait for server to fetch state

		clearScreen()
		fmt.Println("======================================")
		fmt.Printf(" ATUADOR: %s\n", actorID)
		
		dataLock.RLock()
		st := actorState
		dataLock.RUnlock()
		
		fmt.Printf(" ESTADO ATUAL: %s\n", st)
		fmt.Println("======================================")
		fmt.Println(" [1] Ligar (ON)")
		fmt.Println(" [2] Desligar (OFF)")
		fmt.Println("--------------------------------------")
		fmt.Println(" [R] Atualizar Estado")
		fmt.Println(" [B] Voltar")
		fmt.Print("\n Selecione uma opção: ")

		scanner.Scan()
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		if input == "1" {
			// The pipe '|' acts as a delimiter so the server knows which command to route
			tcpConn.Write([]byte("USER/SST/" + actorID + "|ON"))
			time.Sleep(200 * time.Millisecond) 
		} else if input == "2" {
			tcpConn.Write([]byte("USER/SST/" + actorID + "|OFF"))
			time.Sleep(200 * time.Millisecond)
		} else if input == "B" {
			return
		}
		// If 'R' or empty, the loop just restarts and fetches the new state!
	}
}

func handshake() bool {
	tcpConn.Write([]byte("USER/HND/--"))
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