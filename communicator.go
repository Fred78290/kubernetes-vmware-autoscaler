package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/golang/glog"
	"golang.org/x/crypto/ssh"
)

// PublicKeyFile read public key
func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func pipe(args ...string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	glog.V(5).Infof("Shell:%v", args)

	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		s := strings.TrimSpace(stderr.String())

		return s, fmt.Errorf("%s, %s", err.Error(), s)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func shell(args ...string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	glog.V(5).Infof("Shell:%v", args)

	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s, %s", err.Error(), strings.TrimSpace(stderr.String()))
	}

	return nil
}

func scp(host, src, dst string) error {
	return shell(fmt.Sprintf("scp %s %s@:%s", src, host, dst))
}

func sudo(host string, command ...string) (string, error) {
	var sshConfig *ssh.ClientConfig
	var err error

	connect := phAutoScalerServer.Configuration.SSH

	if len(connect.Password) > 0 {
		sshConfig = &ssh.ClientConfig{
			User: connect.UserName,
			Auth: []ssh.AuthMethod{
				ssh.Password(connect.Password),
			},
		}
	} else {
		sshConfig = &ssh.ClientConfig{
			User: connect.UserName,
			Auth: []ssh.AuthMethod{
				PublicKeyFile(connect.AuthKeys),
			},
		}
	}

	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), sshConfig)
	if err != nil {
		return "", fmt.Errorf("Failed to dial: %s", err)
	}

	session, err := connection.NewSession()
	if err != nil {
		return "", fmt.Errorf("Failed to create session: %s", err)
	}

	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		return "", fmt.Errorf("request for pseudo terminal failed: %s", err)
	}

	var stdout bytes.Buffer

	for _, cmd := range command {
		glog.V(5).Infof("Shell:%s", cmd)

		if out, err := session.CombinedOutput(fmt.Sprintf("sudo %s", cmd)); err != nil {
			return "", err
		} else {
			stdout.Write(out)
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}
