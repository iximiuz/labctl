package ssh

// Based on https://gist.github.com/devinodaniel/8f9b8a4f31573f428f29ec0e884e6673

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	identityFile = "id_rsa"

	bitSize = 4096
)

func GenerateIdentity(dirpath string) error {
	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		return fmt.Errorf("generate SSH private key: %w", err)
	}

	publicKey, err := generatePublicKey(&privateKey.PublicKey)
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
		encodePrivateKeyToPEM(privateKey),
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

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, fmt.Errorf("generate SSH private key: %w", err)
	}

	if err = key.Validate(); err != nil {
		return nil, fmt.Errorf("validate SSH private key: %w", err)
	}

	return key, nil
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	return pem.EncodeToMemory(&privBlock)
}

func generatePublicKey(privatekey *rsa.PublicKey) ([]byte, error) {
	key, err := ssh.NewPublicKey(privatekey)
	if err != nil {
		return nil, fmt.Errorf("generate SSH public key: %w", err)
	}

	return ssh.MarshalAuthorizedKey(key), nil
}
