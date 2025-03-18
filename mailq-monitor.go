package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
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
	Password  string `yaml:"password"`
	Host      string `yaml:"host"`
	Port      string `yaml:"port"`
	Commands  string `yaml:"commands"`
	Threshold int    `yaml:"threshold"`
}

type Email struct {
	SMTPServer string   `yaml:"smtp_server"`
	SMTPPort   string   `yaml:"smtp_port"`
	From       string   `yaml:"from"`
	To         []string `yaml:"to"` // 複数のメールアドレスをサポート
	Subject    string   `yaml:"subject"`
	Message    string   `yaml:"messages"`
}

func loadConfig(filename string) (*Config, error) {

	// 設定ファイルを読み込む
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func runSSHCommand(server Server) (string, error) {

	// SSHクライアントを作成
	var authMethod ssh.AuthMethod

	// パスワードが設定されている場合はパスワード認証を使う
	if server.Password != "" {
		authMethod = ssh.Password(server.Password)
	} else {
		// パスワードが設定されてない場合は、SSHエージェントを使う
		sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
		if sshAgentSock == "" {
			return "", fmt.Errorf("SSH_AUTH_SOCK variable is not found")
		}

		// SSHエージェントに接続して秘密鍵を取得
		conn, err := net.Dial("unix", sshAgentSock)
		if err != nil {
			return "", fmt.Errorf("failed to connect to SSH agent: %v", err)
		}
		defer conn.Close()

		// SSHエージェントクライアントを作成
		agentClient := agent.NewClient(conn)
		signers, err := agentClient.Signers()
		if err != nil {
			return "", fmt.Errorf("failed to get signers from SSH agent: %v", err)
		}
		authMethod = ssh.PublicKeys(signers...)
	}

	config := &ssh.ClientConfig{
		User:            server.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// SSHサーバーに接続
	address := fmt.Sprintf("%s:%s", server.Host, server.Port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %v", server.Host, err)
	}
	defer client.Close()

	// SSHセッションを作成
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session on %s: %v", server.Host, err)
	}
	defer session.Close()

	// コマンドを実行して出力を取得
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Run(server.Commands); err != nil {
		return "", fmt.Errorf("failed to run command on %s: %v, stderr: %s", server.Host, err, stderr.String())
	}

	// 標準出力を返す
	return stdout.String(), nil
}

func sendEmail(emailConfig Email, alerts []string) error {

	// アラートがない場合は送信しない
	if len(alerts) == 0 {
		return nil // 送信不要
	}

	// メールのヘッダーとボディを作成
	body := fmt.Sprintf("%s\n%s", emailConfig.Message, strings.Join(alerts, "\n"))
	subject := emailConfig.Subject
	msg := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\n%s", emailConfig.From, strings.Join(emailConfig.To, ", "), subject, body)

	// SMTPサーバーに接続してメールを送信
	addr := fmt.Sprintf("%s:%s", emailConfig.SMTPServer, emailConfig.SMTPPort)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}
	defer conn.Close()

	// SMTPクライアントを作成してメールを送信
	client, err := smtp.NewClient(conn, emailConfig.SMTPServer)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Close()

	// MAIL FROMとRCPT TOコマンドを送信
	if err := client.Mail(emailConfig.From); err != nil {
		return fmt.Errorf("failed to set From address: %v", err)
	}
	for _, addr := range emailConfig.To {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("failed to set To address: %v", err)
		}
	}

	// DATAコマンドを送信してメールヘッダーとボディを送信
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start email body: %v", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write email body: %v", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close email body: %v", err)
	}

	// QUITコマンドを送信して接続を閉じる
	if err = client.Quit(); err != nil {
		return fmt.Errorf("failed to quit SMTP client: %v", err)
	}

	// ログにメール送信成功を出力
	log.Println("Email alert sent successfully")
	return nil
}

func main() {

	// 設定ファイルを読み込む
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 各サーバーに対してSSHコマンドを実行し、閾値を超えていたらアラートを送信する
	var alerts []string
	var sendAlert bool
	for _, server := range config.Servers {

		// SSHコマンドを実行し、出力を取得する
		output, err := runSSHCommand(server)
		if err != nil {
			log.Printf("SSH execution failed on %s: %v", server.Host, err)
			continue
		}

		// 出力を数値に変換する
		outputValue, err := strconv.Atoi(strings.TrimSpace(output))
		if err != nil {
			log.Printf("Failed to parse output on %s: %v", server.Host, err)
			continue
		}

		// もし閾値を超えていたらマークを付けて、sendAlertをtrueにする
		marker := ""
		if outputValue >= server.Threshold {
			marker = " *"
			sendAlert = true
		}
		alerts = append(alerts, fmt.Sprintf("%s: %d%s", server.Host, outputValue, marker))
	}

	// もしsendAlertがtrueなら、メールを送信する
	if sendAlert {
		sendEmail(config.Email, alerts)
	}
}
