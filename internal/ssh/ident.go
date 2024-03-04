package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

const (
	identityFile = "id_ed25519"
)

func GenerateIdentity(dirpath string) error {
	publicKey, privateKey, err := generateKeys()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dirpath, 0700); err != nil {
		return fmt.Errorf("create SSH key directory: %w", err)
	}
	if err := os.Chmod(dirpath, 0700); err != nil {
		return fmt.Errorf("chmod SSH key directory: %w", err)
	}

	if err := os.WriteFile(
		filepath.Join(dirpath, identityFile),
		privateKey,
		0600,
	); err != nil {
		return fmt.Errorf("write SSH private key to file: %w", err)
	}

	if err := os.WriteFile(
		filepath.Join(dirpath, identityFile+".pub"),
		publicKey,
		0600,
	); err != nil {
		return fmt.Errorf("write SSH public key to file: %w", err)
	}

	return nil
}

func RemoveIdentity(dirpath string) error {
	if err := os.Remove(filepath.Join(dirpath, identityFile)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove SSH private key: %w", err)
	}

	if err := os.Remove(filepath.Join(dirpath, identityFile+".pub")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove SSH public key: %w", err)
	}

	return nil
}

func ReadPublicKey(dirpath string) (string, error) {
	publicKey, err := os.ReadFile(filepath.Join(dirpath, identityFile+".pub"))
	if err != nil {
		return "", fmt.Errorf("read SSH public key: %w", err)
	}

	return string(publicKey), nil
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
