package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

func GenerateIdentity(identityFile string, passphrase string) error {
	publicKey, privateKey, err := generateKeys()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(identityFile), 0700); err != nil {
		return fmt.Errorf("create SSH key directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(identityFile), 0700); err != nil {
		return fmt.Errorf("chmod SSH key directory: %w", err)
	}

	if err := os.WriteFile(identityFile, privateKey, 0600); err != nil {
		return fmt.Errorf("write SSH private key to file: %w", err)
	}

	if err := os.WriteFile(identityFile+".pub", publicKey, 0644); err != nil {
		return fmt.Errorf("write SSH public key to file: %w", err)
	}

	if passphrase != "" {
		cmd := exec.Command("ssh-keygen",
			"-p",
			"-f", identityFile,
			"-N", passphrase,
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unlock SSH private key: %w", err)
		}
	}

	return nil
}

func RemoveIdentity(identityFile string) error {
	identityFile = strings.TrimSuffix(identityFile, ".pub")

	if err := os.Remove(identityFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove SSH private key: %w", err)
	}

	if err := os.Remove(identityFile + ".pub"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove SSH public key: %w", err)
	}

	return nil
}

func ReadPublicKey(identityFile string) (string, error) {
	if !strings.HasSuffix(identityFile, ".pub") {
		identityFile = identityFile + ".pub"
	}

	publicKey, err := os.ReadFile(identityFile)
	if err != nil {
		return "", fmt.Errorf("read SSH public key: %w", err)
	}

	return string(publicKey), nil
}

func readPrivateKey(identityFile string) (string, error) {
	privateKey, err := os.ReadFile(strings.TrimSuffix(identityFile, ".pub"))
	if err != nil {
		return "", fmt.Errorf("read SSH private key: %w", err)
	}

	return string(privateKey), nil
}

func generateKeys() ([]byte, []byte, error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ed25519 keys: %w", err)
	}

	publicKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create SSH public key: %w", err)
	}

	pemKey := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(privKey),
	}

	return ssh.MarshalAuthorizedKey(publicKey), pem.EncodeToMemory(pemKey), nil
}
