package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func generateCert() {
	pair, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		logrus.Error(err)
	}

	// 生成证书
	cert := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "ws-shell",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour * 24 * 365),
		DNSNames:    []string{"ws-shell", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &pair.PublicKey, pair)
	if err != nil {
		logrus.Error(err)
	}

	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	err = os.WriteFile("cert.pem", certPem, 0644)
	if err != nil {
		logrus.Error(err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(pair)
	if err != nil {
		logrus.Error(err)
	}

	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	err = os.WriteFile("key.pem", keyPem, 0644)
	if err != nil {
		logrus.Error(err)
	}
}
