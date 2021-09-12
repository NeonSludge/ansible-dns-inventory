package inventory

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/pkg/errors"
)

func tlsCAPoolFromFile(ca string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	cert, err := os.ReadFile(ca)
	if err != nil {
		return nil, err
	}

	if ok := pool.AppendCertsFromPEM(cert); !ok {
		return nil, errors.New("invalid CA certificate")
	}

	return pool, nil
}

func tlsKeyPairFromFile(cert string, key string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(cert, key)
}

func tlsCAPoolFromPEM(ca string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()

	if ok := pool.AppendCertsFromPEM([]byte(ca)); !ok {
		return nil, errors.New("invalid CA certificate")
	}

	return pool, nil
}

func tlsKeyPairFromPEM(cert string, key string) (tls.Certificate, error) {
	return tls.X509KeyPair([]byte(cert), []byte(key))
}
