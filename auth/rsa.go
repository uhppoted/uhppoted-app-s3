package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

func Sign(acl []byte, keyfile string) ([]byte, error) {
	key, err := loadPrivateKey(keyfile)
	if err != nil {
		return nil, err
	} else if key == nil {
		return nil, fmt.Errorf("invalid RSA signing key")
	}

	rng := rand.Reader
	hashed := sha256.Sum256(acl)

	return rsa.SignPKCS1v15(rng, key, crypto.SHA256, hashed[:])
}

func Verify(signedBy string, acl []byte, signature []byte, dir string) error {
	pubkey, err := loadPublicKey(dir, signedBy)
	if err != nil {
		return err
	} else if pubkey == nil {
		return fmt.Errorf("%s: no RSA public key", signedBy)
	}

	hash := sha256.Sum256(acl)
	err = rsa.VerifyPKCS1v15(pubkey, crypto.SHA256, hash[:], signature)
	if err != nil {
		return fmt.Errorf("%s: invalid RSA signature (%w)", signedBy, err)
	}

	return nil
}

func loadPrivateKey(filepath string) (*rsa.PrivateKey, error) {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(bytes)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("%s is not a valid RSA private key", filepath)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid RSA private key", filepath)
	}

	pk, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s is not a valid RSA private key", filepath)
	}

	return pk, nil
}

func loadPublicKey(dir, id string) (*rsa.PublicKey, error) {
	file := filepath.Join(dir, id+".pub")
	bytes, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(bytes)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("%s is not a valid RSA public key", file)
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid RSA public key (%w)", file, err)
	}

	pubkey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s is not a valid RSA public key", file)
	}

	return pubkey, nil
}
