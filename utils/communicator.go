package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	glog "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// AuthMethodFromPrivateKeyFile read public key
func AuthMethodFromPrivateKeyFile(file string) ssh.AuthMethod {
	if buffer, err := ioutil.ReadFile(file); err == nil {
		if key, err := ssh.ParsePrivateKey(buffer); err == nil {
			return ssh.PublicKeys(key)
		} else {
			glog.Fatalf("Can't parse key file:%s, reason:%v", file, err)
		}
	} else {
		glog.Fatalf("Can't read key file:%s, reason:%v", file, err)
	}

	return nil
}

// AuthMethodFromPrivateKey read public key
func AuthMethodFromPrivateKey(key string) ssh.AuthMethod {
	if pub, err := ssh.ParsePrivateKey([]byte(key)); err == nil {
		if signer, err := ssh.NewSignerFromKey(pub); err == nil {
			return ssh.PublicKeys(signer)
		} else {
			glog.Fatalf("AuthMethodFromPublicKey fatal error:%v", err)
		}
	} else {
		glog.Fatalf("AuthMethodFromPublicKey fatal error:%v", err)
	}

	return nil
}

// Sudo exec ssh command as sudo
func Sudo(connect *types.AutoScalerServerSSH, host string, command ...string) (string, error) {
	var sshConfig *ssh.ClientConfig
	var err error

	if len(connect.Password) > 0 {
		sshConfig = &ssh.ClientConfig{
			User:            connect.GetUserName(),
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Auth: []ssh.AuthMethod{
				ssh.Password(connect.Password),
			},
		}
	} else {
		sshConfig = &ssh.ClientConfig{
			User:            connect.GetUserName(),
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Auth: []ssh.AuthMethod{
				AuthMethodFromPrivateKeyFile(connect.GetAuthKeys()),
			},
		}
	}

	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), sshConfig)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %s", err)
	}

	session, err := connection.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %s", err)
	}

	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := session.RequestPty("xterm", 80, 200, modes); err != nil {
		return "", fmt.Errorf("request for pseudo terminal failed: %s", err)
	}

	var stdout bytes.Buffer

	for _, cmd := range command {
		glog.Debugf("Shell:%s", cmd)

		if out, err := session.CombinedOutput(fmt.Sprintf("sudo %s", cmd)); err != nil {
			return "", err
		} else {
			stdout.Write(out)
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}
