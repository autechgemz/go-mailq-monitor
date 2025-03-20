package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Servers []Server `yaml:"servers"`
	Email   Email    `yaml:"email"`
}

type Server struct {
	User      string `yaml:"user"`
	Password  string `yaml:"password,omitempty"`
	Host      string `yaml:"host"`
	Port      string `yaml:"port,omitempty"`
	Commands  string `yaml:"commands"`
	Threshold int    `yaml:"threshold"`
}

func (s *Server) GetPort() string {
	if s.Port == "" {
		return "22"
	}
	return s.Port
}

type Email struct {
	SMTPServer string   `yaml:"smtp_server"`
	SMTPPort   string   `yaml:"smtp_port"`
	From       string   `yaml:"from"`
	To         []string `yaml:"to"`
	Cc         []string `yaml:"cc,omitempty"`
	Bcc        []string `yaml:"bcc,omitempty"`
	Subject    string   `yaml:"subject"`
	Messages   string   `yaml:"messages"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}

func validateConfig(config Config) error {
	if len(config.Servers) == 0 {
		return errors.New("no servers defined in the configuration")
	}

	for i, server := range config.Servers {
		var validUsernameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
		if server.User == "" || !validUsernameRegex.MatchString(server.User) {
			return fmt.Errorf("server[%d]: 'user' is required", i)
		}
		if server.Host == "" {
			return fmt.Errorf("server[%d]: 'host' is required", i)
		}

		var validHostnameRegex = regexp.MustCompile(`^(([a-zA-Z0-9](-?[a-zA-Z0-9])*)\.)*([A-Za-z0-9](-?[A-Za-z0-9])*)$`)
		if !validHostnameRegex.MatchString(server.Host) {
			ip := net.ParseIP(server.Host)
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("server[%d]: 'host' must be a valid hostname or IPv4 address", i)
			}
		}
		if server.Commands == "" {
			return fmt.Errorf("server[%d]: 'commands' is required", i)
		}

		var validCommandRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-./\|\s><{},=]+$`)
		if !validCommandRegex.MatchString(server.Commands) {
			return fmt.Errorf("server[%d]: 'commands' contains invalid characters", i)
		}
		if server.Threshold < 0 {
			return fmt.Errorf("server[%d]: 'threshold' must be zero or a positive integer", i)
		}

		if server.Port == "" {
			server.Port = "22"
		}
	}

	email := config.Email
	if email.SMTPServer == "" {
		return errors.New("'smtp_server' is required in email configuration")
	}
	port, err := strconv.Atoi(email.SMTPPort)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("'smtp_port' must be a valid number between 1 and 65535")
	}
	if email.From == "" {
		return errors.New("'from' is required in email configuration")
	}
	var validEmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !validEmailRegex.MatchString(email.From) {
		return errors.New("'from' must be a valid email address")
	}
	for fieldName, recipients := range map[string][]string{
		"to":  email.To,
		"cc":  email.Cc,
		"bcc": email.Bcc,
	} {
		for i, recipient := range recipients {
			if !validEmailRegex.MatchString(recipient) {
				return fmt.Errorf("'%s[%d]' must be a valid email address", fieldName, i)
			}
		}
	}

	return nil
}

func runSSHCommand(server Server) (string, error) {
	authMethod, err := getSSHAuthMethod(server.Password)
	if err != nil {
		return "", err
	}
	clientConfig := &ssh.ClientConfig{
		User:            server.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	address := fmt.Sprintf("%s:%s", server.Host, server.GetPort())
	client, err := ssh.Dial("tcp", address, clientConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", server.Host, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session on %s: %w", server.Host, err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Run(server.Commands); err != nil {
		return "", fmt.Errorf("failed to run command on %s: %w, stderr: %s", server.Host, err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func getSSHAuthMethod(password string) (ssh.AuthMethod, error) {
	if password != "" {
		return ssh.Password(password), nil
	}

	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAgentSock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK variable is not found")
	}

	conn, err := net.Dial("unix", sshAgentSock)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)
	signers, err := agentClient.Signers()
	if err != nil {
		return nil, fmt.Errorf("failed to get signers from SSH agent: %w", err)
	}
	return ssh.PublicKeys(signers...), nil
}

func sendEmail(emailConfig Email, alerts []string) error {
	if len(alerts) == 0 {
		return nil
	}

	body := fmt.Sprintf("%s\n\n%s", emailConfig.Messages, strings.Join(alerts, "\n"))
	msg := fmt.Sprintf("From: %s\nTo: %s\n",
		emailConfig.From,
		strings.Join(emailConfig.To, ", "),
	)

	if len(emailConfig.Cc) > 0 {
		msg += fmt.Sprintf("Cc: %s\n", strings.Join(emailConfig.Cc, ", "))
	}

	msg += fmt.Sprintf("Subject: %s\n\n%s", emailConfig.Subject, body)

	recipients := append(emailConfig.To, emailConfig.Cc...)
	recipients = append(recipients, emailConfig.Bcc...)

	addr := fmt.Sprintf("%s:%s", emailConfig.SMTPServer, emailConfig.SMTPPort)
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	if err := client.Mail(emailConfig.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send email data: %w", err)
	}
	defer wc.Close()

	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	log.Println("Email alert sent successfully")
	return nil
}

func main() {
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if err := validateConfig(*config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	var alerts []string
	var alertMaker bool
	for _, server := range config.Servers {
		output, err := runSSHCommand(server)
		if err != nil {
			log.Printf("Error executing command on %s: %v", server.Host, err)
			continue
		}

		value, err := strconv.Atoi(output)
		if err != nil {
			log.Printf("Error parsing output on %s: %v", server.Host, err)
			continue
		}

		if value >= server.Threshold {
			alerts = append(alerts, fmt.Sprintf("%s: %d *", server.Host, value))
			alertMaker = true
		} else {
			alerts = append(alerts, fmt.Sprintf("%s: %d", server.Host, value))
		}
	}

	if alertMaker {
		if err := sendEmail(config.Email, alerts); err != nil {
			log.Printf("Error sending email: %v", err)
		}
	}
}
