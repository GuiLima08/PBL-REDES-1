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
	latestData   string
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

	// --- MAIN APP LOOP ---
	for {
		// 1. Show Menu & Get Choice
		selectedSensor := showMenu()
		
		// 2. Stream Data (Blocks until user presses Enter)
		streamData(selectedSensor)
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
		} else if strings.HasPrefix(msg, "DATA/") {
			dataLock.Lock()
			latestData = strings.TrimPrefix(msg, "DATA/")
			dataLock.Unlock()
		}
	}
}

// --- SCREEN 1: THE MENU ---
func showMenu() string {
	for {
		// Ask the server for the latest list
		tcpConn.Write([]byte("USER/LST/--"))
		time.Sleep(200 * time.Millisecond) // Give the background loop a moment to receive it

		clearScreen()
		fmt.Println("======================================")
		fmt.Println("       MONITOR DE AERONAVE")
		fmt.Println("======================================")

		listLock.RLock()
		sensors := make([]string, len(sensorList))
		copy(sensors, sensorList)
		listLock.RUnlock()

		if len(sensors) == 0 {
			fmt.Println("\n Nenhum sensor encontrado. Tente atualizar a lista.")
		} else {
			fmt.Println("\n SENSORES DISPONÍVEIS:")
			for i, s := range sensors {
				fmt.Printf("  [%d] %s\n", i+1, s)
			}
		}

		fmt.Println("\n--------------------------------------")
		fmt.Println(" [R] Atualizar lista")
		fmt.Println(" [Q] Sair")
		fmt.Print("\n Selecione uma opção: ")

		// Wait for user input
		scanner.Scan()
		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		// Handle Special Commands
		if input == "Q" {
			tcpConn.Write([]byte("USER/BYE/--")) // Graceful disconnect
			clearScreen()
			fmt.Println("Desconectado. Adeus!")
			os.Exit(0)
		}
		if input == "R" || input == "" {
			continue // Loop restarts and redraws the menu
		}

		// Handle Number Selection
		choice, err := strconv.Atoi(input)
		if err == nil && choice > 0 && choice <= len(sensors) {
			return sensors[choice-1]
		}
	}
}

// --- SCREEN 2: THE LIVE STREAM ---
func streamData(sensorID string) {
	// Request connection
	tcpConn.Write([]byte("USER/GET/" + sensorID))
	
	// Reset the display data
	dataLock.Lock()
	latestData = "CONECTANDO..."
	dataLock.Unlock()

	// Setup an asynchronous listener for the "Enter" key
	stopChan := make(chan bool)
	go func() {
		scanner.Scan() // This blocks until the user presses Enter
		stopChan <- true
	}()

	// Setup a ticker to redraw the screen every 500ms
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			// User pressed Enter! Send the Disconnect command and exit the loop.
			tcpConn.Write([]byte("USER/DCN/--"))
			return 

		case <-ticker.C:
			// Time to redraw the screen
			dataLock.RLock()
			data := latestData
			dataLock.RUnlock()

			clearScreen()
			fmt.Println("======================================")
			fmt.Printf("SENSOR: \"%s\" (%s)\n", sensorID, dataTypes[sensorID][0])
			fmt.Println("======================================")
			
			// Format the DATA/A/15.5 string to look nice
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