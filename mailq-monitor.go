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

// Config構造体: 設定ファイル全体の構造を定義
type Config struct {
	Servers []Server `yaml:"servers"` // サーバー設定のリスト
	Email   Email    `yaml:"email"`   // メール送信設定
}

// Server構造体: 各サーバーの設定を定義
type Server struct {
	User      string `yaml:"user"`      // SSH接続ユーザー名
	Password  string `yaml:"password"`  // SSH接続パスワード（省略可能）
	Host      string `yaml:"host"`      // サーバーのホスト名またはIPアドレス
	Port      string `yaml:"port"`      // SSH接続ポート番号（省略可能）
	Commands  string `yaml:"commands"`  // 実行するコマンド
	Threshold int    `yaml:"threshold"` // アラートを発生させるしきい値
}

// デフォルトポートを取得するヘルパー関数
func (s *Server) GetPort() string {
	if s.Port == "" {
		return "22" // デフォルトポート
	}
	return s.Port
}

// Email構造体: メール送信の設定を定義
type Email struct {
	SMTPServer string   `yaml:"smtp_server"`   // SMTPサーバーのホスト名またはIPアドレス
	SMTPPort   string   `yaml:"smtp_port"`     // SMTPサーバーのポート番号
	From       string   `yaml:"from"`          // 送信元メールアドレス
	To         []string `yaml:"to"`            // 送信先メールアドレスのリスト
	Cc         []string `yaml:"cc,omitempty"`  // Cc（カーボンコピー）宛先、省略可能
	Bcc        []string `yaml:"bcc,omitempty"` // Bcc（ブラインドカーボンコピー）宛先、省略可能
	Subject    string   `yaml:"subject"`       // メールの件名
	Message    string   `yaml:"messages"`      // メール本文のテンプレート
}

// 設定ファイルを読み込む
func loadConfig(filename string) (*Config, error) {
	// 設定ファイルを読み込む
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// YAMLデータをConfig構造体にデコード
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}

// SSHコマンドを実行する
func runSSHCommand(server Server) (string, error) {
	// SSH認証方法を取得
	authMethod, err := getSSHAuthMethod(server.Password)
	if err != nil {
		return "", err
	}

	// SSHクライアント設定を作成
	clientConfig := &ssh.ClientConfig{
		User:            server.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ホストキーの検証を無効化（本番環境では注意）
	}

	// SSH接続を確立
	address := fmt.Sprintf("%s:%s", server.Host, server.GetPort()) // GetPortでポートを取得
	client, err := ssh.Dial("tcp", address, clientConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", server.Host, err)
	}
	defer client.Close()

	// SSHセッションを作成
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session on %s: %w", server.Host, err)
	}
	defer session.Close()

	// コマンドの実行と結果の取得
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Run(server.Commands); err != nil {
		return "", fmt.Errorf("failed to run command on %s: %w, stderr: %s", server.Host, err, stderr.String())
	}

	// コマンドの出力を返す
	return strings.TrimSpace(stdout.String()), nil
}

// SSH認証方法を取得する
func getSSHAuthMethod(password string) (ssh.AuthMethod, error) {
	// パスワード認証を使用
	if password != "" {
		return ssh.Password(password), nil
	}

	// SSHエージェントを使用
	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAgentSock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK variable is not found")
	}

	// SSHエージェントに接続
	conn, err := net.Dial("unix", sshAgentSock)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	defer conn.Close()

	// SSHエージェントから署名者を取得
	agentClient := agent.NewClient(conn)
	signers, err := agentClient.Signers()
	if err != nil {
		return nil, fmt.Errorf("failed to get signers from SSH agent: %w", err)
	}
	return ssh.PublicKeys(signers...), nil
}

// メールを送信する
func sendEmail(emailConfig Email, alerts []string) error {
	if len(alerts) == 0 {
		return nil
	}

	// メール本文を作成
	body := fmt.Sprintf("%s\n\n%s", emailConfig.Message, strings.Join(alerts, "\n"))
	msg := fmt.Sprintf("From: %s\nTo: %s\n",
		emailConfig.From,
		strings.Join(emailConfig.To, ", "),
	)

	// Ccが設定されている場合のみヘッダーに追加
	if len(emailConfig.Cc) > 0 {
		msg += fmt.Sprintf("Cc: %s\n", strings.Join(emailConfig.Cc, ", "))
	}

	// メール件名と本文を追加
	msg += fmt.Sprintf("Subject: %s\n\n%s", emailConfig.Subject, body)

	// 宛先リストにTo、Cc、Bccを含める
	recipients := append(emailConfig.To, emailConfig.Cc...)
	recipients = append(recipients, emailConfig.Bcc...)

	// SMTPサーバーに接続
	addr := fmt.Sprintf("%s:%s", emailConfig.SMTPServer, emailConfig.SMTPPort)
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// STARTTLSをスキップしてTLSを無効化
	// SMTPサーバーに送信元を設定
	if err := client.Mail(emailConfig.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// 宛先を設定
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// メールデータを送信
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send email data: %w", err)
	}
	defer wc.Close()

	// メール本文を送信
	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	log.Println("Email alert sent successfully")
	return nil
}

// メイン処理
func main() {
	// 設定ファイルを読み込む
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	var alerts []string
	// 各サーバーに対してコマンドを実行
	for _, server := range config.Servers {
		output, err := runSSHCommand(server)
		if err != nil {
			log.Printf("Error executing command on %s: %v", server.Host, err)
			continue
		}

		// コマンドの出力を数値に変換
		value, err := strconv.Atoi(output)
		if err != nil {
			log.Printf("Error parsing output on %s: %v", server.Host, err)
			continue
		}

		// しきい値を超えた場合にマーカーを付ける
		marker := ""
		if value >= server.Threshold {
			marker = " *"
		}
		alerts = append(alerts, fmt.Sprintf("%s: %d%s", server.Host, value, marker))
	}

	// アラートメールを送信
	if err := sendEmail(config.Email, alerts); err != nil {
		log.Printf("Error sending email: %v", err)
	}
}
