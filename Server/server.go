package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SensorData struct { // Estrutura para armazenar os dados de um sensor, incluindo seu ID, valor, tipo e a hora em que os dados foram recebidos
	Id       string
	Value    float64
	Type     string
	Received time.Time
}

var (
	Sensors = make(map[string]SensorData) // Mapa para armazenar os sensores atualmente registrados, onde a chave é o ID do sensor e o valor é a estrutura SensorData com os dados mais recentes do sensor
	Clients = make(map[string]net.Conn)   // Mapa para armazenar os clientes TCP conectados, onde a chave é o endereço do cliente e o valor é a conexão TCP correspondente
	Bridges = make(map[string]string)     // Mapa para armazenar os bridges ativos entre clientes e sensores, onde a chave é o endereço do cliente e o valor é o ID do sensor que está sendo enviado para esse cliente

	Actors      = make(map[string]net.Conn) // Mapa para armazenar os atuadores TCP conectados, onde a chave é o endereço do atuador e o valor é a conexão TCP correspondente
	ActorStates = make(map[string]string)   // Mapa para armazenar o estado atual de cada atuador, onde a chave é o endereço do atuador e o valor é o estado atual (ex: "ON", "OFF", "UNKNOWN")
	ALock       = sync.RWMutex{}            // Mutex para proteger o acesso aos mapas de atuadores e seus estados

	Lock  = sync.RWMutex{} // Mutex para proteger o acesso ao mapa de sensores
	TLock = sync.RWMutex{} // Mutex para proteger o acesso aos mapas de clientes e bridges

	UDP_Types = map[string]string{ // Mapa para traduzir os tipos de sensores recebidos via UDP para uma abreviação
		"ANEMO": "A", //Anemômetro
		"FUEL":  "F", //Combustível
	}
	TCP_Types = []string{ // Lista dos tipos de mensagens TCP esperados, onde "USER" representa mensagens enviadas por clientes usuários e "ACTOR" representa mensagens enviadas por atuadores
		"USER",  //Usuário
		"ACTOR", //Atuador
	}
	Cmd_Types = []string{
		"HND", //Handshake
		"BYE", //Goodbye (desconexão)
		"LST", //Requisição de lista de sensor
		"GET", //Requisição de conexão com sensor (bridge)
		"DCN", //Disconexão do bridge
		"LSA", //Requisição de lista de atuadores
		"CKS", //Check State (ver estado atual do atuador)
		"SST", //Set State (Enviar comando para atuador)
		"FDB", //Feedback (do atuador para o servidor, repassado ao usuário)
	}
)

func main() {
	if len(os.Args) != 2 { // Verifica se a porta foi fornecida como argumento
		output("ERR: Incorrect command usage\nCorrect usage: go run server.go <port>")
		return
	}
	port := os.Args[1]

	output(fmt.Sprintf("Server starting on port %s...", port))

	go handleUDP(port) // Inicia o servidor UDP para receber dados dos sensores
	go listenTCP(port) // Inicia o servidor TCP para lidar com conexões de usuários e atuadores

	for { // Loop principal para monitorar sensores e imprimir status a cada 5 segundos
		time.Sleep(1 * time.Second)

		Lock.RLock()
		if len(Sensors) == 0 {
			Lock.RUnlock()
			continue
		}
		for id, data := range Sensors {
			output(fmt.Sprintf("\"%s\": [%s] %.2f (%s)", id, data.Type, data.Value, data.Received.Format("15:04:05")))
		}
		Lock.RUnlock()
	}
}

func handleUDP(port string) { // Inicia o servidor UDP para receber dados dos sensores
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+port)
	if err != nil {
		output(fmt.Sprintf("-!- Error resolving UDP address: %v", err))
		return
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		output(fmt.Sprintf("-!- Error starting UDP server: %v", err))
		return
	}
	defer udpConn.Close()

	output("Listening for UDP sensor data...")

	go heartbeatUDP() // Inicia uma goroutine para monitorar o status dos sensores a cada 5 segundos

	var buf [1024]byte

	for { // Loop para receber dados dos sensores
		n, addr, err := udpConn.ReadFromUDP(buf[:])
		if err != nil {
			output(fmt.Sprintf("-!- Error reading UDP: %v", err))
			continue
		}

		packetData := make([]byte, n)
		copy(packetData, buf[:n])

		go func(sender *net.UDPAddr, b []byte) { // Processa cada pacote recebido em uma goroutine separada
			msg := strings.TrimSpace(string(b))
			parts := strings.Split(msg, "/") // Espera formato "TYPE/VALUE"
			senderIp := sender.String()

			if len(parts) != 2 {
				output(fmt.Sprintf("-!- Invalid message format from %s: %s", senderIp, msg))
				return
			}

			sensorType := parts[0]
			if _, exists := UDP_Types[sensorType]; !exists {
				output(fmt.Sprintf("-!- Unknown sensor type from %s: %s", senderIp, sensorType))
				return
			}

			value, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				output(fmt.Sprintf("-!- Invalid numerical value from %s: %s", senderIp, parts[1]))
				return
			}

			deviceId := fmt.Sprintf("%s-%s", senderIp, UDP_Types[sensorType]) // Gera um ID único para o sensor baseado no IP e tipo

			Lock.Lock()
			if _, exists := Sensors[deviceId]; !exists { // Novo sensor, imprime mensagem de conexão
				output(fmt.Sprintf("Sensor \"%s\" (%s) CONNECTED", deviceId, sensorType))
			}

			Sensors[deviceId] = SensorData{ // Atualiza ou cria a entrada do sensor com os novos dados
				Id:       deviceId,
				Value:    value,
				Received: time.Now(),
				Type:     sensorType,
			}
			Lock.Unlock()

		}(addr, packetData)
	}
}

func heartbeatUDP() { // Loop para monitorar o status dos sensores a cada 5 segundos e remover os que estão inativos
	for {
		time.Sleep(5 * time.Second)
		Lock.Lock()
		for id, data := range Sensors {
			if time.Since(data.Received) > 5*time.Second { // Se o sensor não enviar dados por mais de 5 segundos, considera desconectado
				output(fmt.Sprintf("Sensor \"%s\" DISCONNECTED (last seen %.2f seconds ago)", id, time.Since(data.Received).Seconds()))
				delete(Sensors, id)
			}
		}
		Lock.Unlock()
	}
}

func listenTCP(port string) { // Inicia o servidor TCP para lidar com conexões de usuários e atuadores
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		output(fmt.Sprintf("-!- Error starting TCP server: %v", err))
		return
	}
	defer listener.Close()

	output(fmt.Sprintf("TCP server listening on port %s", port))

	for { // Loop para aceitar conexões TCP de usuários e atuadores
		conn, err := listener.Accept()
		if err != nil {
			output(fmt.Sprintf("-!- Error accepting TCP connection: %v", err))
			continue
		}

		output(fmt.Sprintf("TCP client connected from %s", conn.RemoteAddr()))
		go handleTCP(conn) // Lida com cada cliente em uma goroutine separada para permitir múltiplas conexões simultâneas
	}
}

func handleTCP(conn net.Conn) { // Lida com a comunicação com um cliente TCP (usuário ou atuador)
	defer func() { // Garante que, ao desconectar, o cliente seja removido das listas e o estado seja limpo
		output(fmt.Sprintf("TCP client from %s disconnected", conn.RemoteAddr()))

		ALock.Lock()
		delete(Actors, conn.RemoteAddr().String())
		delete(ActorStates, conn.RemoteAddr().String())
		ALock.Unlock()

		TLock.Lock()
		delete(Clients, conn.RemoteAddr().String())
		delete(Bridges, conn.RemoteAddr().String())
		TLock.Unlock()

		conn.Close()
	}()
	go sensorBridge(conn) // Inicia uma goroutine para enviar dados do sensor para o cliente se ele solicitar um bridge

	scanner := bufio.NewScanner(conn) // Lê mensagens do cliente linha por linha

	for scanner.Scan() {
		msg := strings.TrimSpace(scanner.Text())
		parts := strings.Split(msg, "/") // Espera formato "TYPE/CMD/CONTENT"

		if len(parts) != 3 { // Verifica se a mensagem tem o formato correto
			output(fmt.Sprintf("-!- Invalid message format from %s: %s", conn.RemoteAddr(), msg))
			continue
		}
		msgType := parts[0]
		cmdType := parts[1]
		content := parts[2]

		msgTypeValid := false
		for _, t := range TCP_Types { // Verifica se o tipo da mensagem é válido (USER ou ACTOR)
			if t == msgType {
				msgTypeValid = true
				break
			}
		}
		if !msgTypeValid { // Se o tipo da mensagem for desconhecido, envia um erro de volta para o cliente e imprime uma mensagem de aviso
			output(fmt.Sprintf("-!- Unknown message type from %s: %s", conn.RemoteAddr(), msgType))
			conn.Write([]byte(fmt.Sprintf("ERR/Unknown message type: %s\n", msgType)))
			continue
		}

		cmdTypeValid := false
		for _, t := range Cmd_Types { // Verifica se o tipo do comando é válido (HND, BYE, LST, GET, DCN, LSA, CKS, SST, FDB)
			if t == cmdType {
				cmdTypeValid = true
				break
			}
		}
		if !cmdTypeValid { // Se o tipo do comando for desconhecido, envia um erro de volta para o cliente e imprime uma mensagem de aviso
			output(fmt.Sprintf("-!- Unknown command type from %s: %s", conn.RemoteAddr(), cmdType))
			conn.Write([]byte(fmt.Sprintf("ERR/Unknown command type: %s\n", cmdType)))
			continue
		}

		switch msgType { // Roteia a mensagem para a função de tratamento apropriada com base no tipo (USER ou ACTOR)
		case "USER":
			handleUser(conn, cmdType, content) // Lida com mensagens do tipo USER
		case "ACTOR":
			handleActor(conn, cmdType, content) // Lida com mensagens do tipo ACTOR
		default:
			output(fmt.Sprintf("-!- Unhandled message type from %s: %s", conn.RemoteAddr(), msgType))
		}
	}
}

func handleUser(conn net.Conn, cmdType, content string) { // Lida com mensagens do tipo USER, processando comandos como handshake, lista de sensores, requisição de bridge, etc.
	switch cmdType {
	case "HND": // Handshake inicial para registrar o cliente como um usuário válido
		output(fmt.Sprintf("Handshake received from %s (USER)", conn.RemoteAddr()))
		TLock.Lock()
		Clients[conn.RemoteAddr().String()] = conn // Registra o cliente na lista de clientes conectados
		TLock.Unlock()
		conn.Write([]byte("HND/ACCEPTED\n")) // Envia uma resposta de handshake aceito de volta para o cliente
		
	case "LST": // Requisição de lista de sensores disponíveis, responde com os IDs dos sensores atualmente registrados
		Lock.RLock()
		var sensorList []string
		for id := range Sensors {
			sensorList = append(sensorList, id)
		}
		Lock.RUnlock()
		response := fmt.Sprintf("LST/%s\n", strings.Join(sensorList, ",")) // Formata a resposta com a lista de sensores separados por vírgula
		conn.Write([]byte(response))
		
	case "GET": // Requisição para estabelecer um bridge com um sensor específico
		output(fmt.Sprintf("Sensor connection requested by %s (USER) for sensor \"%s\"", conn.RemoteAddr(), content))
		Lock.RLock()
		sensor, exists := Sensors[content]
		Lock.RUnlock()
		if !exists { // Se o sensor solicitado não existir, envia um erro de volta para o cliente e imprime uma mensagem de aviso
			conn.Write([]byte(fmt.Sprintf("ERR/Sensor \"%s\" not found\n", content)))
			output(fmt.Sprintf("-!- Sensor \"%s\" requested by %s (USER) not found", content, conn.RemoteAddr()))
			return
		}
		go updateBridge(conn, sensor) // Estabelece o bridge para o sensor solicitado, permitindo que os dados do sensor sejam enviados para o cliente a cada segundo
		
	case "DCN": // Requisição para desconectar um bridge existente, remove o cliente da lista de bridges e imprime uma mensagem de desconexão
		output(fmt.Sprintf("Sensor disconnection requested by %s (USER)", conn.RemoteAddr()))
		TLock.Lock()
		delete(Bridges, conn.RemoteAddr().String()) // Remove o cliente da lista de bridges para parar de enviar dados do sensor para ele
		TLock.Unlock()
		
	case "BYE": // Requisição de desconexão do cliente, a função defer no handleClient já cuida de limpar as listas e fechar a conexão, aqui só imprimimos uma mensagem de desconexão
		output(fmt.Sprintf("Disconnect requested by %s (USER)", conn.RemoteAddr()))
		
	case "LSA": // Requisição de lista de atuadores disponíveis, responde com os IDs dos atuadores atualmente registrados
		ALock.RLock()
		var actorList []string
		for id := range Actors {
			actorList = append(actorList, id)
		}
		ALock.RUnlock()
		response := fmt.Sprintf("LSA/%s\n", strings.Join(actorList, ",")) // Formata a resposta com a lista de atuadores separados por vírgula
		conn.Write([]byte(response))
		
	case "CKS": // Requisição para verificar o estado atual de um atuador específico, responde com o estado atual do atuador se ele existir
		actorId := content
		ALock.RLock()
		state, exists := ActorStates[actorId]
		actorConn, actorExists := Actors[actorId]
		ALock.RUnlock()

		if !exists || !actorExists { // Se o atuador solicitado não existir, envia um erro de volta para o cliente e imprime uma mensagem de aviso
			_, err := conn.Write([]byte(fmt.Sprintf("CKS/%s/Not Found,%s\n", actorId, time.Now().Format("15:04:05"))))
			if err != nil {
				output(fmt.Sprintf("-!- Error sending CKS response for actor \"%s\": %v", actorId, err))
			}
			output(fmt.Sprintf("-!- Actor \"%s\" requested by %s (USER) not found", actorId, conn.RemoteAddr()))
			return
		}

		actorConn.Write([]byte("FEEDBACK\n")) // Solicita um feedback do atuador para garantir que temos o estado mais recente antes de responder ao cliente
		_, err := conn.Write([]byte(fmt.Sprintf("CKS/%s/%s,%s\n", actorId, state, time.Now().Format("15:04:05"))))
		if err != nil {
			output(fmt.Sprintf("-!- Error sending CKS response for actor \"%s\": %v", actorId, err))
		}

	case "SST": // Requisição para definir o estado de um atuador específico, extrai o ID do atuador e o novo estado do conteúdo da mensagem, e envia o comando para o atuador correspondente
		subparts := strings.Split(content, "|") // Espera formato "actorId|newState"
		if len(subparts) != 2 {
			return
		}
		actorId, newState := subparts[0], subparts[1]

		ALock.RLock()
		actorConn, exists := Actors[actorId]
		ALock.RUnlock()

		if !exists { // Se o atuador solicitado não existir, envia um erro de volta para o cliente e imprime uma mensagem de aviso
			output(fmt.Sprintf("-!- Actor \"%s\" requested by %s (USER) not found for SST command", actorId, conn.RemoteAddr()))
			conn.Write([]byte(fmt.Sprintf("ERR/Actor \"%s\" not found\n", actorId)))
			return
		}
		actorConn.Write([]byte(newState + "\n")) // Envia o novo estado para o atuador, que deve processar o comando e enviar um feedback de volta com o estado atualizado
		output(fmt.Sprintf("Forwarded %s command to actor %s", newState, actorId))
		
	default: // Se o comando for desconhecido, envia um erro de volta para o cliente e imprime uma mensagem de aviso
		output(fmt.Sprintf("-!- Unhandled command type from %s (USER): %s", conn.RemoteAddr(), cmdType))
	}
}

func handleActor(conn net.Conn, cmdType, content string) { // Lida com mensagens do tipo ACTOR, processando comandos como handshake e feedback de estado
	switch cmdType {
	case "HND": // Handshake inicial para registrar o cliente como um atuador válido, adiciona o atuador à lista de atuadores e inicializa seu estado como "UNKNOWN"
		output(fmt.Sprintf("Handshake received from %s (ACTOR)", conn.RemoteAddr()))

		ALock.Lock()
		Actors[conn.RemoteAddr().String()] = conn           // Registra o atuador na lista de atuadores conectados
		ActorStates[conn.RemoteAddr().String()] = "UNKNOWN" // Inicializa o estado do atuador como "UNKNOWN" até receber um feedback real do atuador sobre seu estado atual
		ALock.Unlock()

		conn.Write([]byte("HND/ACCEPTED\n")) // Envia uma resposta de handshake aceito de volta para o atuador

		time.Sleep(100 * time.Millisecond)
		conn.Write([]byte("FEEDBACK\n")) // Solicita um feedback do atuador para obter seu estado inicial assim que ele se conectar, o atuador deve responder com um comando FDB contendo seu estado atual

	case "FDB": // Feedback do atuador para atualizar seu estado no servidor, o conteúdo da mensagem deve conter o novo estado do atuador, que é salvo na estrutura ActorStates e impresso no terminal se houver uma mudança real de estado
		ALock.Lock()
		oldState := ActorStates[conn.RemoteAddr().String()] // Salva o estado antigo
		ActorStates[conn.RemoteAddr().String()] = content   // Atualiza pro novo
		ALock.Unlock()

		// Só imprime no terminal se houve uma mudança real
		if oldState != content {
			output(fmt.Sprintf("Actor %s updated state to: %s", conn.RemoteAddr(), content))
		}

	default: // Se o comando for desconhecido, envia um erro de volta para o atuador e imprime uma mensagem de aviso
		output(fmt.Sprintf("-!- Unhandled command type from %s (ACTOR): %s", conn.RemoteAddr(), cmdType))
	}
}

func sensorBridge(conn net.Conn) { // Continuamente envia dados do sensor para o cliente se ele tiver solicitado um bridge
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	user := conn.RemoteAddr().String()
	for range ticker.C { // A cada segundo, verifica se o cliente ainda tem um bridge ativo
		TLock.RLock()
		sensorId, exists := Bridges[user]
		TLock.RUnlock()

		if !exists { // Se o cliente não tiver um bridge ativo, simplesmente continua esperando sem fazer nada
			continue
		}
		Lock.RLock()
		sensor, exists := Sensors[sensorId]
		if !exists { // Se o sensor associado ao bridge não existir mais, envia uma mensagem de erro para o cliente
			response := "DATA/Sensor not found\n"
			conn.Write([]byte(response))
			Lock.RUnlock()
			continue
		}
		Lock.RUnlock()

		response := fmt.Sprintf("DATA/%s/%.2f\n", sensor.Type, sensor.Value)
		_, err := conn.Write([]byte(response)) // Envia os dados do sensor para o cliente no formato "DATA/TYPE/VALUE"
		if err != nil {
			return // Se houver um erro ao enviar os dados (por exemplo, o cliente desconectou), simplesmente retorna para encerrar a goroutine e limpar as listas no defer do handleClient
		}
	}
}

func updateBridge(conn net.Conn, sensor SensorData) { // Estabelece o bridge para o sensor solicitado
	TLock.Lock()
	Bridges[conn.RemoteAddr().String()] = sensor.Id
	TLock.Unlock()
	output(fmt.Sprintf("Bridge established for \"%s\" to sensor \"%s\"", conn.RemoteAddr(), sensor.Id))
}

func output(text string) { // Função auxiliar para imprimir mensagens no terminal com timestamp
	fmt.Printf("%s >> %s\n", time.Now().Format("2006-01-02 15:04:05"), text)
}
