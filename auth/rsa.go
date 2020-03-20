package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

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

func loadPublicKey(dir, id string) (*rsa.PublicKey, error) {
	file := filepath.Join(dir, id+".pub")
	bytes, err := ioutil.ReadFile(file)
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
