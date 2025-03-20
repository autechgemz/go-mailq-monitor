# mailq-monitor

`mailq-monitor`は、指定されたサーバーにSSHで接続し、特定のコマンドを実行してその結果を監視するツールです。
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

    もし、Windows用にバイナリを生成する場合は、下記のようにクロスコンパイルします。
    ```sh
    GOOS=windows GOARCH=amd64 go build -o mailq-monitor.exe
    ```

## 設定

  `config.yaml`ファイルを作成し、以下のように設定します。

```yaml
# SSHコマンドに関する設定です
# 必要に応じて変更してください
#
# user: SSH接続するユーザ名
# password: SSH接続するユーザのパスワード (省略可、省略するとssh-agentを使用します)
# host: SSH接続するホスト名
# port: SSH接続するポート番号
# commands: 実行するコマンド (コマンドは1つです。thresholdと比較するため数字で返ってくる必要があります)
# threshold: アラートを発生させるしきい値
#
servers:
  - user: "vagrant"
    host: "192.168.56.201"
    port: "22"
    commands: "sudo find /var/spool/postfix | wc -l"
    threshold: 100
  - user: "vagrant"
    host: "192.168.56.202"
    port: "22"
    commands: "sudo find /var/spool/postfix | wc -l"
    threshold: 10
  - user: "vagrant"
    host: "192.168.56.203"
    port: "22"
    commands: "sudo find /var/spool/postfix | wc -l"
    threshold: 10

# メール送信に関する設定です
# STARTTLSは未対応です。必要に応じて変更してください
#
# smtp_server: SMTPサーバのIPアドレス or ホスト名 (複数指定不可)
# smtp_port: SMTPサーバのポート番号
# from: 送信元メールアドレス
# to: 送信先メールアドレス (複数指定不可)
# subject: メールの件名
# body_template: メールの本文 (アラートに付与するメッセージです。複数行可)
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
  subject: "Mail Queue Alert"
  body_template: |
    こちらはメールキューのアラートメールです。
    メールキューがしきい値を超えました。
```

## 使い方

config.yamlとバイナリは同一ディレクトリに置いてください

```sh
./mailq-monitor
```

あるいは下記のように実行することができます。

```sh
PS C:\Users\mailqguys\work> .\mailq-monitor.exe
2025/03/20 10:00:20 Email alert sent successfully
```

## サンプル

```
From: alerts@example.com
To: test1@example.com, test2@example.com
Cc: cc1@example.com, cc2@example.com
Subject: Mail Queue Alert

こちらはメールキューのアラートメールです。

メールキューがしきい値を超えました。

192.168.56.201: 58
192.168.56.202: 86 *
192.168.56.203: 46 *
```
