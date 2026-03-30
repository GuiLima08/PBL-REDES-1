package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SensorData struct {
	Id       string
	Value    float64
	Type     string
	Received time.Time
}

var (
	Sensors   = make(map[string]SensorData)
	Clients   = make(map[string]net.Conn)
	Bridges   = make(map[string]string)
	Lock      = sync.RWMutex{}
	TLock     = sync.RWMutex{} // Separate lock for TCP Bridges
	UDP_Types = map[string]string{
		"ANEMO": "A", // Anemometer
		"FUEL":  "F", // Fuel level
	}
	TCP_Types = []string{
		"USER",
		"ACTOR",
	}
	Cmd_Types = []string{
		"HND", // Handshake
		"BYE", // Disconnect/Exit
		"LST", // Sensor List request
		"GET", // Connect to sensor request
		"DCN", // Disconnect from sensor request
	}
)

func main() {
	if len(os.Args) != 2 {
		output("ERR: Incorrect command usage\nCorrect usage: go run server.go <port>")
		return
	}
	port := os.Args[1]

	output(fmt.Sprintf("Server starting on port %s...", port))

	go handleUDP(port)
	go handleTCP(port)

	for {
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

func handleUDP(port string) {
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

	go statusUDP()

	var buf [1024]byte

	for {
		n, addr, err := udpConn.ReadFromUDP(buf[:])
		if err != nil {
			output(fmt.Sprintf("-!- Error reading UDP: %v", err))
			continue
		}

		packetData := make([]byte, n)
		copy(packetData, buf[:n])

		go func(sender *net.UDPAddr, b []byte) {
			msg := strings.TrimSpace(string(b))
			parts := strings.Split(msg, "/")
			senderIp := sender.IP.String()

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

			deviceId := fmt.Sprintf("%s-%s", senderIp, UDP_Types[sensorType])

			Lock.Lock()
			if _, exists := Sensors[deviceId]; !exists {
				output(fmt.Sprintf("Sensor \"%s\" (%s) CONNECTED", deviceId, sensorType))
			}

			Sensors[deviceId] = SensorData{
				Id:       deviceId,
				Value:    value,
				Received: time.Now(),
				Type:     sensorType,
			}
			Lock.Unlock()

		}(addr, packetData)
	}
}

func statusUDP() {
	for {
		time.Sleep(5 * time.Second)
		Lock.Lock()
		for id, data := range Sensors {
			if time.Since(data.Received) > 5*time.Second {
				output(fmt.Sprintf("Sensor \"%s\" DISCONNECTED (last seen %.2f seconds ago)", id, time.Since(data.Received).Seconds()))
				delete(Sensors, id)
			}
		}
		Lock.Unlock()
	}
}

func handleTCP(port string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		output(fmt.Sprintf("-!- Error starting TCP server: %v", err))
		return
	}
	defer listener.Close()

	output(fmt.Sprintf("TCP server listening on port %s", port))

	for {
		conn, err := listener.Accept()
		if err != nil {
			output(fmt.Sprintf("-!- Error accepting TCP connection: %v", err))
			continue
		}

		output(fmt.Sprintf("TCP client connected from %s", conn.RemoteAddr()))
		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer func() {
		output(fmt.Sprintf("TCP client from %s disconnected", conn.RemoteAddr()))
		conn.Close()
	}()
	go sensorBridge(conn)
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		msg := strings.TrimSpace(string(buf[:n]))
		parts := strings.Split(msg, "/")
		if len(parts) != 3 {
			output(fmt.Sprintf("-!- Invalid message format from %s: %s", conn.RemoteAddr(), msg))
			continue
		}
		msgType := parts[0]
		cmdType := parts[1]
		content := parts[2]

		msgTypeValid := false
		for _, t := range TCP_Types {
			if t == msgType {
				msgTypeValid = true
				break
			}
		}
		if !msgTypeValid {
			output(fmt.Sprintf("-!- Unknown message type from %s: %s", conn.RemoteAddr(), msgType))
			conn.Write([]byte(fmt.Sprintf("ERR/Unknown message type: %s", msgType)))
			continue
		}

		cmdTypeValid := false
		for _, t := range Cmd_Types {
			if t == cmdType {
				cmdTypeValid = true
				break
			}
		}
		if !cmdTypeValid {
			output(fmt.Sprintf("-!- Unknown command type from %s: %s", conn.RemoteAddr(), cmdType))
			conn.Write([]byte(fmt.Sprintf("ERR/Unknown command type: %s", cmdType)))
			continue
		}

		switch msgType {
		case "USER":
			handleUser(conn, cmdType, content)
		case "ACTOR":
			handleActor(conn, cmdType, content)
		default:
			output(fmt.Sprintf("-!- Unhandled message type from %s: %s", conn.RemoteAddr(), msgType))
		}
	}
}

func handleUser(conn net.Conn, cmdType, content string) {
	switch cmdType {
	case "HND":
		output(fmt.Sprintf("Handshake received from %s (USER)", conn.RemoteAddr()))
		TLock.Lock()
		Clients[conn.RemoteAddr().String()] = conn
		TLock.Unlock()
		response := fmt.Sprint("HND/ACCEPTED")
		conn.Write([]byte(response))
		break

	case "LST":
		output(fmt.Sprintf("Sensor list requested by %s (USER)", conn.RemoteAddr()))
		Lock.RLock()
		var sensorList []string
		for id := range Sensors {
			sensorList = append(sensorList, id)
		}
		Lock.RUnlock()
		response := fmt.Sprint("LST/", strings.Join(sensorList, ","))
		conn.Write([]byte(response))
		break

	case "GET":
		output(fmt.Sprintf("Sensor connection requested by %s (USER) for sensor \"%s\"", conn.RemoteAddr(), content))
		Lock.RLock()
		sensor, exists := Sensors[content]
		Lock.RUnlock()
		if !exists {
			conn.Write([]byte(fmt.Sprintf("ERR/Sensor \"%s\" not found", content)))
			output(fmt.Sprintf("-!- Sensor \"%s\" requested by %s (USER) not found", content, conn.RemoteAddr()))
			return
		}
		go updateBridge(conn, sensor)
		break

	case "DCN":
		output(fmt.Sprintf("Sensor disconnection requested by %s (USER)", conn.RemoteAddr()))
		TLock.Lock()
		delete(Bridges, conn.RemoteAddr().String())
		TLock.Unlock()
		break

	case "BYE":
		output(fmt.Sprintf("Disconnect requested by %s (USER)", conn.RemoteAddr()))
		TLock.Lock()
		delete(Clients, conn.RemoteAddr().String())
		delete(Bridges, conn.RemoteAddr().String())
		TLock.Unlock()
		conn.Close()
		break

	default:
		output(fmt.Sprintf("-!- Unhandled command type from %s (USER): %s", conn.RemoteAddr(), cmdType))
		break
	}
}

func handleActor(conn net.Conn, cmdType, content string) {
	switch cmdType {
	case "HND":
		output(fmt.Sprintf("Handshake received from %s (ACTOR)", conn.RemoteAddr()))
		Clients[conn.RemoteAddr().String()] = conn
		response := fmt.Sprint("HND/ACCEPTED")
		_, err := conn.Write([]byte(response))
		if err != nil {
			output(fmt.Sprintf("-!- Error sending handshake response to %s (ACTOR): %v", conn.RemoteAddr(), err))
		}

	default:
		output(fmt.Sprintf("-!- Unhandled command type from %s (ACTOR): %s", conn.RemoteAddr(), cmdType))
	}
}

func sensorBridge(conn net.Conn) {
	output(fmt.Sprintf("Starting sensor bridge for \"%s\"", conn.RemoteAddr()))
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	user := conn.RemoteAddr().String()
	for {
		select {
		case <-ticker.C:
			TLock.RLock()
			sensorId, exists := Bridges[user]
			TLock.RUnlock()

			if !exists {
				continue
			}
			Lock.RLock()
			sensor := Sensors[sensorId]
			Lock.RUnlock()

			response := fmt.Sprintf("DATA/%s/%.2f", sensor.Type, sensor.Value)
			_, err := conn.Write([]byte(response))
			if err != nil {
				output(fmt.Sprintf("-!- Error sending data to %s for sensor \"%s\": %v", conn.RemoteAddr(), sensorId, err))
				return
			}
		}
	}

}

func updateBridge(conn net.Conn, sensor SensorData) {
	TLock.Lock()
	Bridges[conn.RemoteAddr().String()] = sensor.Id
	TLock.Unlock()
	output(fmt.Sprintf("Bridge established for \"%s\" to sensor \"%s\"", conn.RemoteAddr(), sensor.Id))
}

func output(text string) {
	fmt.Printf("%s >> %s\n", time.Now().Format("2006-01-02 15:04:05"), text)
}
