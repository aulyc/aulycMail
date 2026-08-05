package smtp

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

type smtpTestAddr string

func (a smtpTestAddr) Network() string { return "test" }
func (a smtpTestAddr) String() string  { return string(a) }

type smtpRecordingConn struct {
	readData         []byte
	written          []byte
	readDeadline     time.Time
	writeDeadline    time.Time
	readDeadlineErr  error
	writeDeadlineErr error
}

func (c *smtpRecordingConn) Read(p []byte) (int, error) {
	if len(c.readData) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.readData)
	c.readData = c.readData[n:]
	return n, nil
}

func (c *smtpRecordingConn) Write(p []byte) (int, error) {
	c.written = append(c.written, p...)
	return len(p), nil
}

func (c *smtpRecordingConn) Close() error                { return nil }
func (c *smtpRecordingConn) LocalAddr() net.Addr         { return smtpTestAddr("local") }
func (c *smtpRecordingConn) RemoteAddr() net.Addr        { return smtpTestAddr("remote") }
func (c *smtpRecordingConn) SetDeadline(time.Time) error { return nil }
func (c *smtpRecordingConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return c.readDeadlineErr
}
func (c *smtpRecordingConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return c.writeDeadlineErr
}

func TestDeadlineConnAndDisconnectedClientBehavior(t *testing.T) {
	underlying := &smtpRecordingConn{readData: []byte("smtp")}
	conn := &deadlineConn{Conn: underlying, readTimeout: time.Second, writeTimeout: 2 * time.Second}
	buf := make([]byte, 4)
	if n, err := conn.Read(buf); err != nil || n != 4 || string(buf) != "smtp" || underlying.readDeadline.IsZero() {
		t.Fatalf("Read() = %d, %v, %q deadline=%v", n, err, buf, underlying.readDeadline)
	}
	if n, err := conn.Write([]byte("mail")); err != nil || n != 4 || string(underlying.written) != "mail" || underlying.writeDeadline.IsZero() {
		t.Fatalf("Write() = %d, %v, %q deadline=%v", n, err, underlying.written, underlying.writeDeadline)
	}
	readErr := errors.New("read deadline")
	underlying.readDeadlineErr = readErr
	if _, err := conn.Read(buf); !errors.Is(err, readErr) {
		t.Fatalf("Read() deadline error = %v", err)
	}
	writeErr := errors.New("write deadline")
	underlying.writeDeadlineErr = writeErr
	if _, err := conn.Write(nil); !errors.Is(err, writeErr) {
		t.Fatalf("Write() deadline error = %v", err)
	}

	config := DefaultConfig()
	if config.Port != 587 || config.Security != SecurityStartTLS || config.ConnectTimeout != 30*time.Second || config.ReadTimeout != 30*time.Second || config.WriteTimeout != 30*time.Second {
		t.Fatalf("DefaultConfig() = %+v", config)
	}
	client := NewClient(ClientConfig{Host: "smtp.example.com", Username: "user"})
	if client.config.Host != "smtp.example.com" || client.config.Username != "user" {
		t.Fatalf("NewClient() config = %+v", client.config)
	}
	if err := client.Login(); err == nil || err.Error() != "not connected" {
		t.Fatalf("Login() disconnected error = %v", err)
	}
	if err := client.SendMail("from@example.com", []string{"to@example.com"}, []byte("body")); err == nil || err.Error() != "not connected" {
		t.Fatalf("SendMail() disconnected error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() disconnected error = %v", err)
	}
}

func TestLoginAuthStateMachine(t *testing.T) {
	auth := LoginAuth("user@example.com", "secret")
	mechanism, initial, err := auth.Start(nil)
	if err != nil || mechanism != "LOGIN" || initial != nil {
		t.Fatalf("Start() = %q, %v, %v", mechanism, initial, err)
	}
	if response, err := auth.Next(nil, false); err != nil || response != nil {
		t.Fatalf("Next(done) = %q, %v", response, err)
	}
	if response, err := auth.Next([]byte("Username:"), true); err != nil || string(response) != "user@example.com" {
		t.Fatalf("Next(username) = %q, %v", response, err)
	}
	if response, err := auth.Next([]byte("PASSWORD:"), true); err != nil || string(response) != "secret" {
		t.Fatalf("Next(password) = %q, %v", response, err)
	}
	if _, err := auth.Next([]byte("OTP:"), true); err == nil || !strings.Contains(err.Error(), "unknown prompt") {
		t.Fatalf("Next(unknown) error = %v", err)
	}
}

type smtpSessionResult struct {
	authPayload string
	commands    []string
	body        string
	err         error
}

func TestClientConnectLoginSendAndCloseAgainstSMTPServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	resultCh := make(chan smtpSessionResult, 1)

	go func() {
		result := smtpSessionResult{}
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			result.err = acceptErr
			resultCh <- result
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		writeResponse := func(response string) bool {
			if _, err := writer.WriteString(response); err != nil {
				result.err = err
				return false
			}
			if err := writer.Flush(); err != nil {
				result.err = err
				return false
			}
			return true
		}
		if !writeResponse("220 localhost ESMTP ready\r\n") {
			resultCh <- result
			return
		}

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				result.err = err
				break
			}
			line = strings.TrimSpace(line)
			result.commands = append(result.commands, line)
			switch {
			case strings.HasPrefix(line, "EHLO "):
				if !writeResponse("250-localhost\r\n250 AUTH PLAIN\r\n") {
					break
				}
			case strings.HasPrefix(line, "AUTH PLAIN "):
				encoded := strings.TrimPrefix(line, "AUTH PLAIN ")
				decoded, decodeErr := base64.StdEncoding.DecodeString(encoded)
				if decodeErr != nil {
					result.err = decodeErr
					break
				}
				result.authPayload = string(decoded)
				if !writeResponse("235 2.7.0 authenticated\r\n") {
					break
				}
			case strings.HasPrefix(line, "MAIL FROM:"):
				if !writeResponse("250 sender ok\r\n") {
					break
				}
			case strings.HasPrefix(line, "RCPT TO:"):
				if !writeResponse("250 recipient ok\r\n") {
					break
				}
			case line == "DATA":
				if !writeResponse("354 send data\r\n") {
					break
				}
				var body strings.Builder
				for {
					dataLine, readErr := reader.ReadString('\n')
					if readErr != nil {
						result.err = readErr
						break
					}
					if strings.TrimSpace(dataLine) == "." {
						break
					}
					body.WriteString(dataLine)
				}
				result.body = body.String()
				if result.err == nil && !writeResponse("250 queued\r\n") {
					break
				}
			case line == "QUIT":
				_ = writeResponse("221 bye\r\n")
				resultCh <- result
				return
			default:
				result.err = fmt.Errorf("unexpected SMTP command %q", line)
				resultCh <- result
				return
			}
			if result.err != nil {
				break
			}
		}
		resultCh <- result
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	config := DefaultConfig()
	config.Host = "localhost"
	config.Port = port
	config.Security = SecurityNone
	config.Username = "user@example.com"
	config.Password = "secret"
	config.ConnectTimeout = 2 * time.Second
	client := NewClient(config)
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect(local SMTP) error = %v", err)
	}
	if err := client.Login(); err != nil {
		t.Fatalf("Login(local SMTP) error = %v", err)
	}
	message := []byte("Subject: coverage\r\n\r\nhello SMTP\r\n")
	if err := client.SendMail("from@example.com", []string{"first@example.com", "second@example.com"}, message); err != nil {
		t.Fatalf("SendMail(local SMTP) error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close(local SMTP) error = %v", err)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("SMTP server error = %v; commands=%v", result.err, result.commands)
		}
		if result.authPayload != "\x00user@example.com\x00secret" {
			t.Fatalf("AUTH payload = %q", result.authPayload)
		}
		commands := strings.Join(result.commands, "\n")
		for _, expected := range []string{"MAIL FROM:<from@example.com>", "RCPT TO:<first@example.com>", "RCPT TO:<second@example.com>", "DATA", "QUIT"} {
			if !strings.Contains(commands, expected) {
				t.Fatalf("commands missing %q: %v", expected, result.commands)
			}
		}
		if !strings.Contains(result.body, "hello SMTP") {
			t.Fatalf("message body = %q", result.body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SMTP server result on port " + strconv.Itoa(port))
	}
}
