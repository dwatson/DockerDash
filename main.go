package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/gorilla/websocket"
	//"github.com/rcrowley/go-metrics"
	"log"
	"net/http"
	"time"
)

var connections map[*websocket.Conn]bool
var images = make([]docker.APIImages, 0)
var containers = make([]docker.Container, 0)

// DockerUpdates sent through channel from docker listener
type DockerUpdates struct {
	ID    string
	State string
}

//IDDataPacket only contains type and ID
type IDDataPacket struct {
	Type string `json:"Type"`
	ID   string `json:"ID"`
}

// DataPacket containing Images and Containers along with a type of update
type DataPacket struct {
	Type       string             `json:"Type"`
	Images     []docker.APIImages `json:",omitempty"`
	Containers []docker.Container `json:",omitempty"`
}

// JSONCmd is for commands sent from the web interface
type JSONCmd struct {
	Command string
	Data    string
}

// listener does see images removed but not added
func listenToDocker(c chan DockerUpdates, endpoint string) {
	client, err := docker.NewClient(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to docker")

	listener := make(chan *docker.APIEvents)
	err = client.AddEventListener(listener)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err = client.RemoveEventListener(listener)
		if err != nil {
			log.Fatal(err)
		}
	}()

	timeout := time.After(1 * time.Second)
	for {
		select {
		case msg := <-listener:
			var data = DockerUpdates{ID: msg.ID, State: msg.Status}
			log.Println(msg)
			c <- data
		case <-timeout:
			break
		}
	}
}

func sendAll(msg []byte) {
	for conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			delete(connections, conn)
			log.Println(err)
			return
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request, endpoint string) {
	// Taken from gorilla's website
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	}
	defer func() {
		conn.Close()
		delete(connections, conn)
	}()
	connections[conn] = true

	client, error := docker.NewClient(endpoint)
	if error != nil {
		log.Fatal(error)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		log.Println(string(msg))

		var command JSONCmd
		err = json.Unmarshal(msg, &command)
		if err != nil {
			fmt.Println("error:", err)
		}

		switch command.Command {
		case "init":
			var data = DataPacket{Type: "full", Images: images, Containers: containers}
			bytes, e := json.Marshal(data)
			if e != nil {
				http.Error(w, "Error marshalling JSON", http.StatusInternalServerError)
				return
			}
			sendAll(bytes)
		case "start":
			client.StartContainer(command.Data, nil)
		case "stop":
			client.StopContainer(command.Data, 15)
		case "remove":
			removeOptions := docker.RemoveContainerOptions{ID: command.Data, RemoveVolumes: true, Force: false}
			client.RemoveContainer(removeOptions)
		case "kill":
			killOptions := docker.KillContainerOptions{ID: command.Data}
			client.KillContainer(killOptions)
		}
	}
}

func serve(port int, dir string, endpoint string) {
	fs := http.Dir(dir)
	fileHandler := http.FileServer(fs)
	http.Handle("/", fileHandler)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(w, r, endpoint)
	})

	log.Printf("Serving files from %v at 127.0.0.1:%d", dir, port)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	err := http.ListenAndServe(addr, nil)
	log.Println(err.Error())
}

func initContainers(client *docker.Client) {
	containers = nil
	conoptions := docker.ListContainersOptions{All: true}
	cons, _ := client.ListContainers(conoptions)
	for _, con := range cons {
		fmt.Printf("%+v", con)
		info, _ := client.InspectContainer(con.ID)
		containers = append(containers, *info)
	}
}

func main() {
	// command line flags
	port := flag.Int("port", 8000, "port to serve on")
	dir := flag.String("directory", "web/", "directory of web files")
	endpoint := flag.String("endpoint", "unix:///var/run/docker.sock", "docker endpoint")
	flag.Parse()

	connections = make(map[*websocket.Conn]bool)

	go serve(*port, *dir, *endpoint)

	dockerChan := make(chan DockerUpdates)
	go listenToDocker(dockerChan, *endpoint)

	client, error := docker.NewClient(*endpoint)
	if error != nil {
		log.Fatal(error)
	}

	imgs, _ := client.ListImages(true)
	for _, img := range imgs {
		if img.RepoTags[0] != "<none>:<none>" {
			images = append(images, img)
		}
	}

	initContainers(client)

	for {
		select {
		case msg := <-dockerChan:

			info, _ := client.InspectContainer(msg.ID)
			switch msg.State {
			case "die":
				// Update local data
				initContainers(client)
				// Send remove event
				var diecontainers = make([]docker.Container, 0)
				diecontainers = append(diecontainers, *info)
				var data = DataPacket{Type: "remove", Containers: diecontainers}
				bytes, e := json.Marshal(data)
				if e != nil {
					return
				}
				sendAll(bytes)
			case "start":
				// Update local data
				containers = append(containers, *info)
				// Send add event

				var startcontainers = make([]docker.Container, 0)
				startcontainers = append(startcontainers, *info)
				var data = DataPacket{Type: "start", Containers: startcontainers}
				bytes, e := json.Marshal(data)
				if e != nil {
					return
				}
				sendAll(bytes)
			case "destroy":
				// Update local data
				for p, con := range containers {
					if msg.ID == con.ID {
						containers = append(containers[:p], containers[p+1:]...)
						break
					}
				}

				// Send destroy event

				var data = IDDataPacket{Type: "destroy", ID: msg.ID}
				bytes, e := json.Marshal(data)
				if e != nil {
					return
				}
				sendAll(bytes)
			}

		}
	}
}
