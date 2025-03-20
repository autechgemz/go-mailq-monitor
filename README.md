# mailq-monitor

`mailq-monitor` is a tool that connects to specified servers via SSH, executes specific commands, and monitors the results.  
If the results exceed the specified threshold, an alert email is sent.

## Features

- Connect to multiple servers via SSH
- Execute specified commands
- Compare command results with thresholds
- Send alert emails if thresholds are exceeded

## Requirements

- Go 1.16 or later
- Servers with SSH access configured
- SMTP server

## Installation

1. Clone the repository.

    ```sh
    git clone <repository>
    cd mailq-monitor
    ```

2. Install the required dependencies.

    ```sh
    go mod tidy
    ```

3. Build the program.

    ```sh
    go build -o mailq-monitor
    ```

    To generate a binary for Windows, use the following cross-compilation command:
    ```sh
    GOOS=windows GOARCH=amd64 go build -o mailq-monitor.exe
    ```

## Configuration

Create a [config.yaml](http://_vscodecontentref_/1) file and configure it as follows:

```yaml
# SSH command configuration
# Modify as needed
#
# user: Username for SSH connection
# password: Password for the SSH user (optional, uses ssh-agent if omitted)
# host: Hostname for SSH connection
# port: Port number for SSH connection
# commands: Command to execute (must return a numeric value for comparison with the threshold)
# threshold: Threshold value to trigger an alert
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

# Email alert configuration
# STARTTLS is not supported. Modify as needed.
#
# smtp_server: IP address or hostname of the SMTP server (single server only)
# smtp_port: Port number of the SMTP server
# from: Sender email address
# to: Recipient email addresses (multiple addresses allowed)
# cc: CC recipient email addresses (optional, multiple addresses allowed)
# bcc: BCC recipient email addresses (optional, multiple addresses allowed)
# subject: Email subject
# body_template: Email body (message attached to the alert, supports multiple lines)
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
    This is an alert email for the mail queue.
    The mail queue has exceeded the threshold.
```

## Usage

Place the config.yaml and binary in the same directory

```sh
./mailq-monitor
```

Alternatively, you can run it as follows:

```sh
PS C:\Users\mailqguys\work> .\mailq-monitor.exe
2025/03/20 10:00:20 Email alert sent successfully
```

## Sample

```
From: alerts@example.com
To: test1@example.com, test2@example.com
Cc: cc1@example.com, cc2@example.com
Subject: Mail Queue Alert

This is an alert email for the mail queue.
The mail queue has exceeded the threshold.

192.168.56.201: 58
192.168.56.202: 86 *
192.168.56.203: 46 *
```

## License

This project is licensed under the [MIT License](https://opensource.org/licenses/MIT).
