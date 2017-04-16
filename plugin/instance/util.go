package instance

import (
	"math/rand"
	"time"

	"github.com/docker/machine/libmachine/ssh"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func getSSHClient(ipAddr string, port int, userName string, password string) (ssh.Client, error) {
	ssh.SetDefaultClient(ssh.Native)
	auth := ssh.Auth{
		Passwords: []string{password},
	}
	return ssh.NewClient(userName, ipAddr, port, &auth)
}
