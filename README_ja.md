# mailq-monitor

`mailq-monitor` は、指定されたサーバーにSSHで接続し、特定のコマンドを実行してその結果を監視するツールです。  
結果が指定されたしきい値を超えた場合、アラートメールを送信します。

## 機能

- 複数のサーバーにSSHで接続
- 指定されたコマンドを実行
- コマンドの結果をしきい値と比較
- しきい値を超えた場合にアラートメールを送信

## 必要条件

- Go 1.16以上
- SSHアクセスが設定されたサーバー
- SMTPサーバー

## インストール

1. リポジトリをクローンします。

    ```sh
    git clone <repository>
    cd mailq-monitor
    ```

2. 必要な依存関係をインストールします。

    ```sh
    go mod tidy
    ```

3. プログラムをビルドします。

    ```sh
    go build -o mailq-monitor
    ```

    Windows用にバイナリを生成する場合は、以下のクロスコンパイルコマンドを使用します。
    ```sh
    GOOS=windows GOARCH=amd64 go build -o mailq-monitor.exe
    ```

## 設定

[config.yaml](http://_vscodecontentref_/0) ファイルを作成し、以下のように設定します。

```yaml
# SSHコマンド設定
# 必要に応じて変更してください
#
# user: SSH接続するユーザー名
# password: SSH接続するユーザーのパスワード (省略可能、省略時はssh-agentを使用)
# host: SSH接続するホスト名
# port: SSH接続するポート番号
# commands: 実行するコマンド (しきい値と比較するため、数値を返す必要があります)
# threshold: アラートを発生させるしきい値
#
servers:
  - user: "vagrant"
    host: "192.168.56.201"
    port: "22"
    commands: "sudo find /var/spool/postfix/{maildrop,incoming,active,deferred,hold} -type f 2>/dev/null | wc -l"
    threshold: 100
  - user: "vagrant"
    host: "192.168.56.202"
    port: "22"
    commands: "sudo find /var/spool/postfix/{maildrop,incoming,active,deferred,hold} -type f 2>/dev/null | wc -l"
    threshold: 10
  - user: "vagrant"
    host: "192.168.56.203"
    port: "22"
    commands: "sudo find /var/spool/postfix/{maildrop,incoming,active,deferred,hold} -type f 2>/dev/null | wc -l"
    threshold: 10

# メールアラート設定
# STARTTLSは未対応です。必要に応じて変更してください。
#
# smtp_server: SMTPサーバーのIPアドレスまたはホスト名 (単一サーバーのみ)
# smtp_port: SMTPサーバーのポート番号
# from: 送信元メールアドレス
# to: 送信先メールアドレス (複数指定可能)
# cc: CC送信先メールアドレス (省略可能、複数指定可能)
# bcc: BCC送信先メールアドレス (省略可能、複数指定可能)
# subject: メールの件名
# body_template: メール本文 (アラートに付与するメッセージ、複数行対応)
email:
  smtp_server: "192.168.56.201"
  smtp_port: "25"
  from: "alerts@example.com"
  to:
    - "test1@example.com"
    - "test2@example.com"
  cc:
    - "cc1@example.com"
    - "cc2@example.com"
  bcc:
    - "bcc1@example.com"
    - "bcc2@example.com"
  subject: "メールキューアラート"
  body_template: |
    こちらはメールキューのアラートメールです。
    メールキューがしきい値を超えました。