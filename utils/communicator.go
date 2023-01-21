package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	glog "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// AuthMethodFromPrivateKeyFile read public key
func AuthMethodFromPrivateKeyFile(file string) (ssh.AuthMethod, error) {
	var buffer []byte
	var err error
	var key ssh.Signer

	if buffer, err = os.ReadFile(file); err != nil {
		glog.Errorf("Can't read key file:%s, reason:%v", file, err)
	} else if key, err = ssh.ParsePrivateKey(buffer); err != nil {
		glog.Errorf("Can't parse key file:%s, reason:%v", file, err)
	} else {
		return ssh.PublicKeys(key), nil
	}

	return nil, err
}

// AuthMethodFromPrivateKey read public key
func AuthMethodFromPrivateKey(key string) (ssh.AuthMethod, error) {
	var pub ssh.Signer
	var err error
	var signer ssh.Signer

	if pub, err = ssh.ParsePrivateKey([]byte(key)); err != nil {
		glog.Errorf("AuthMethodFromPublicKey fatal error:%v", err)
	} else if signer, err = ssh.NewSignerFromKey(pub); err != nil {
		glog.Errorf("AuthMethodFromPublicKey fatal error:%v", err)
	} else {
		return ssh.PublicKeys(signer), nil
	}

	return nil, err
}

// Shell execute local command ignore output
func Shell(args ...string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s, %s", err.Error(), strings.TrimSpace(stderr.String()))
	}

	return nil
}

// Scp copy file
func Scp(connect *types.AutoScalerServerSSH, host, src, dst string) error {

	if connect.TestMode {
		return nil
	}

	return Shell("scp",
		"-i", connect.GetAuthKeys(),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p",
		"-r",
		src,
		fmt.Sprintf("%s@%s:%s", connect.GetUserName(), host, dst))
}

// Sudo exec ssh command as sudo
func Sudo(connect *types.AutoScalerServerSSH, host string, timeoutInSeconds time.Duration, command ...string) (string, error) {
	var sshConfig *ssh.ClientConfig
	var err error
	var method ssh.AuthMethod

	if connect.TestMode {
		return "", nil
	}

	if len(connect.Password) > 0 {
		method = ssh.Password(connect.Password)
	} else if method, err = AuthMethodFromPrivateKeyFile(connect.GetAuthKeys()); err != nil {
		return "", err
	}

	sshConfig = &ssh.ClientConfig{
		Timeout:         timeoutInSeconds * time.Second,
		User:            connect.GetUserName(),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			method,
		},
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
	var out []byte

	for _, cmd := range command {
		glog.Debugf("Shell:%s", cmd)

		if out, err = session.CombinedOutput(fmt.Sprintf("sudo %s", cmd)); err != nil {
			if out != nil {
				stdout.Write(out)
			} else {
				stdout.Write([]byte(fmt.Sprintf("An error occured during su command: %s, error:%v", cmd, err)))
			}
			break
		} else {
			stdout.Write(out)
		}
	}

	if glog.GetLevel() == glog.DebugLevel && err != nil {
		glog.Debugf("sudo command:%s, output:%s, error:%v", strings.Join(command, ","), stdout.String(), err)
	}

	return strings.TrimSpace(stdout.String()), err
}
