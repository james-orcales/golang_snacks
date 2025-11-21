package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"sync/atomic"
	"time"

	"golang_snacks/invariant"
)

func main() {
	invariant.AssertionFailureCallback = func(msg string) {
		atomic.AddInt32(&assertion_failure_count, 1)
		panic(msg)
	}
	// Ensure any leftover assertions are announced.
	defer func() {
		if atomic.LoadInt32(&assertion_failure_count) > 0 {
			send_email()
			atomic.SwapInt32(&assertion_failure_count, 0)
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

	ticker := time.Tick(notify_frequency)

	// Log assertion failures and continue; let all other panics crash the program.
	database := make(map[string]int)
	for {
		should_shutdown := func() bool {
			defer func() {
				if err := recover(); err != nil {
					if strErr, ok := err.(string); ok && strings.HasPrefix(strErr, invariant.AssertionFailureMsgPrefix) {
						slog.Error(strErr)
					}
				}
			}()
			select {
			case message := <-messages:
				switch message {
				default:
					// To assign a person for each assertion, modify the signature to take a third string
					// containing their email address. I prefer to make it the third parameter so that the most relevant
					// information (1) cond (2) msg are still read first.
					// invariant.Always(message != "you gave me up", "Never gonna give you up.", "firstlast@myorg.io")
					invariant.Always(message != "you gave me up", "Never gonna give you up.")
					database[message] += 1
				case "shutdown":
					// NOTE: this doesn't handle signal interrupts. You can still drop assertions with CTRL-C sending
					// SIGINT for example.
					return true
				}
			case <-ticker:
				if atomic.LoadInt32(&assertion_failure_count) > 0 {
					send_email()
					atomic.SwapInt32(&assertion_failure_count, 0)
				}
			}
			return false
		}()
		if should_shutdown {
			break
		}
	}
	for key, val := range database {
		fmt.Println(key, val)
	}
}

var (
	assertion_failure_count int32 = 0
	email_sent_count              = 0
)

const notify_frequency = time.Second * 30

func send_email() {
	// Safety guard so we don't mistakenly send a million emails...
	const max_emails_sent = 2
	if atomic.LoadInt32(&assertion_failure_count) == 0 || email_sent_count >= max_emails_sent {
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
			assertion_failure_count,
			notify_frequency/time.Second,
		)),
	)
	if err != nil {
		slog.Error("Failed to announce assertion failure via email", "error", err)
		return
	}
	slog.Info("Assertion failures were announced via email.", "assertion_failure_count", assertion_failure_count)
}
