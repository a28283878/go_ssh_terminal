package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/jasonsoft/napnap"
	"golang.org/x/crypto/ssh"
)

type windowSize struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	var listen = flag.String("listen", ":8080", "Host:port to listen on")
	nap := napnap.New()
	flag.Parse()
	router := napnap.NewRouter()
	router.Get("/ssh", napnap.WrapHandler(http.HandlerFunc(sshWebsocket)))
	nap.Use(router)
	httpengine := napnap.NewHttpEngine(*listen)
	println("start")
	log.Fatal(nap.Run(httpengine))
}

func sshWebsocket(w http.ResponseWriter, r *http.Request) {
	//upgrade http to websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err)
		return
	}
	defer conn.Close()

	//get private key of user
	key, err := ioutil.ReadFile("/home/richard_lin/.ssh/id_rsa")
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: "richard_lin",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to the remote server and perform the SSH handshake.
	sshConn, err := ssh.Dial("tcp", "10.200.252.123:22", config)
	if err != nil {
		log.Fatalf("unable to connect: %v", err)
	}
	defer sshConn.Close()

	// Set up new Session between server and host terminal via ssh
	session, err := sshConn.NewSession()
	if err != nil {
		log.Fatal("unable to create session: ", err)
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // enable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err := session.RequestPty("xterm", 80, 30, modes); err != nil {
		log.Fatal("request for pseudo terminal failed: ", err)
	}

	//set io.Reader and io.Writer from terminal session
	sshReader, err := session.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	sshWriter, err := session.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	//read from terminal and write to frontend
	go func() {
		defer func() {
			conn.Close()
			sshConn.Close()
			session.Close()
		}()

		for {
			buf := make([]byte, 4096)
			n, err := sshReader.Read(buf)
			if err != nil {
				log.Print(err)
				return
			}
			err = conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				log.Print(err)
				return
			}
		}
	}()

	//read from frontend and write to terminal
	go func() {
		defer func() {
			conn.Close()
			sshConn.Close()
			session.Close()
		}()

		for {
			// set up io.Reader of websocket
			_, reader, err := conn.NextReader()
			if err != nil {
				log.Print(err)
				return
			}
			// read first byte to determine whether to pass data or resize terminal
			dataTypeBuf := make([]byte, 1)
			_, err = reader.Read(dataTypeBuf)
			if err != nil {
				log.Print(err)
				return
			}

			switch dataTypeBuf[0] {
			// when pass data
			case 0:
				buf := make([]byte, 1024)
				n, err := reader.Read(buf)
				if err != nil {
					log.Print(err)
					return
				}
				_, err = sshWriter.Write(buf[:n])
				if err != nil {
					log.Print(err)
					conn.WriteMessage(websocket.BinaryMessage, []byte(err.Error()))
					return
				}
			// when resize terminal
			case 1:
				decoder := json.NewDecoder(reader)
				resizeMessage := windowSize{}
				err := decoder.Decode(&resizeMessage)
				if err != nil {
					log.Print(err.Error())
					continue
				}
				err = session.WindowChange(resizeMessage.Rows, resizeMessage.Cols)
				if err != nil {
					log.Print(err.Error())
					conn.WriteMessage(websocket.BinaryMessage, []byte(err.Error()))
					return
				}
			// unexpected data
			default:
				log.Print("Unexpected data type")
			}
		}
	}()

	// Start remote shell
	if err := session.Shell(); err != nil {
		log.Println("failed to start shell: ", err)
	}

	if err := session.Wait(); err != nil {
		log.Println("failed to wait shell: ", err)
	}
}
