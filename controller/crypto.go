package controller

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"

	"go-web/resp"

	"github.com/gin-gonic/gin"
)

var (
	rsaPrivateKey   *rsa.PrivateKey
	rsaPublicKeyPEM string
)

func init() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("failed to generate rsa key: " + err.Error())
	}
	rsaPrivateKey = key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic("failed to marshal public key: " + err.Error())
	}
	rsaPublicKeyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}))
}

func decryptPassword(encryptedBase64 string) (string, error) {
	if encryptedBase64 == "" {
		return "", errors.New("empty password")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}
	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaPrivateKey, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func GetPublicKey(c *gin.Context) {
	resp.OK(c, gin.H{"public_key": rsaPublicKeyPEM})
}
