package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"sync/atomic"
	"time"

	"golang_snacks/invariant"
)

func main() {
	// In production, we use soft assertions.
	// During testing, we use hard assertions.
	invariant.AssertionFailureCallback = func(msg string) {
		atomic.AddInt32(&assertionFailureCount, 1)
		slog.Error(msg)
	}
	// Ensure any leftover assertions are announced.
	defer func() {
		if atomic.LoadInt32(&assertionFailureCount) > 0 {
			sendEmail()
			atomic.SwapInt32(&assertionFailureCount, 0)
		}
	}()

	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer server.Close()

	fmt.Printf("Listening on %s\n", server.Addr())
	connections := make(chan *net.TCPConn)
	go func() {
		for {
			conn, err := server.AcceptTCP()
			if err != nil {
				slog.Info("Stopped accepting connections", "err", err)
				close(connections)
				return
			}
			slog.Info("Accepted new connection")
			connections <- conn
		}
	}()
	messages := make(chan string)
	go func() {
		for connection := range connections {
			connection := connection
			go func() {
				buf := [256]byte{}
				for {
					n, err := connection.Read(buf[:])
					if err != nil {
						return
					}
					message := string(buf[:n-1]) // remove newline
					slog.Info("Received data from client", "data", message)
					messages <- message
				}
			}()
		}
	}()

	ticker := time.Tick(notifyFrequency)

loop:
	for {
		select {
		case message := <-messages:
			switch message {
			case "you gave me up":
				// To assign a person for each assertion, you can edit the signature to take a third string containing their email address.
				// I prefer to make it the third parameter so that the most relevant information (1) cond (2) msg are still read first.
				// invariant.Always(message != "you gave me up", "Never gonna give you up.", "firstlast@myorg.io")
				invariant.Always(false, "Never gonna give you up.")
			case "shutdown":
				// NOTE: this doesn't handle signal interrupts. You can still drop assertions with CTRL-C sending SIGINT for example.
				break loop
			}
		case <-ticker:
			if atomic.LoadInt32(&assertionFailureCount) > 0 {
				sendEmail()
				atomic.SwapInt32(&assertionFailureCount, 0)
			}
		}
	}
}

var (
	assertionFailureCount int32 = 0
	emailSentCount              = 0
)

const notifyFrequency = time.Second * 30

func sendEmail() {
	// Safety guard so we don't mistakenly send a million emails...
	const maxEmailsSent = 2
	if atomic.LoadInt32(&assertionFailureCount) == 0 || emailSentCount >= maxEmailsSent {
		return
	}
	err := smtp.SendMail(
		"smtp.gmail.com:587",
		smtp.PlainAuth("", USERNAME, PASSWORD, "smtp.gmail.com"),
		FROM,
		[]string{RECIPIENT},
		[]byte(fmt.Sprintf(
			"To: %s\r\nSubject: 🚨 ASSERTION FAILURE 🚨\r\n\r\nDetected %d assertion failures in the last %d seconds.",
			RECIPIENT,
			assertionFailureCount,
			notifyFrequency/time.Second,
		)),
	)
	if err != nil {
		slog.Error("Failed to announce assertion failure via email", "error", err)
		return
	}
	slog.Info("Assertion failures were announced via email.", "assertionFailureCount", assertionFailureCount)
}
