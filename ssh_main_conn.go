package main

import (
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
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

	session, err := sshConn.NewSession()
	if err != nil {
		log.Fatal("unable to create session: ", err)
	}
	defer session.Close()

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err := session.RequestPty("xterm", 80, 30, modes); err != nil {
		log.Fatal("request for pseudo terminal failed: ", err)
	}
	// Start remote shell
	if err := session.Shell(); err != nil {
		log.Fatal("failed to start shell: ", err)
	}

	if err := session.Wait(); err != nil {
		log.Fatal("failed to wait shell: ", err)
	}
}
