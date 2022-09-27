/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trustedresources

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/encrypted"
)

// signInterface returns the encoded signature for the given object.
func signInterface(signer signature.Signer, i interface{}) (string, error) {
	if signer == nil {
		return "", fmt.Errorf("signer is nil")
	}
	b, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(b)

	sig, err := signer.SignMessage(bytes.NewReader(h.Sum(nil)))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(sig), nil
}

// generateKeyFile creates cosign key files
func generateKeyFile(dir string, pf PassFunc) (string, string, error) {
	keys, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	priv, pub, err := marshalKeyPair(Keys{keys, keys.Public()}, pf)

	priKey := filepath.Join(dir, "cosign.key")
	if err := os.WriteFile(priKey, priv, 0600); err != nil {
		return "", "", err
	}

	pubKey := filepath.Join(dir, "cosign.pub")
	if err := os.WriteFile(pubKey, pub, 0600); err != nil {
		return "", "", err
	}

	return priKey, pubKey, nil
}

// getSignerVerifier creates SignerVerifier from given password
func getSignerVerifier() (signature.SignerVerifier, error) {
	keys, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	sv, err := signature.LoadSignerVerifier(keys, crypto.SHA256)
	if err != nil {
		return nil, err
	}
	return sv, nil
}

func marshalKeyPair(keypair Keys, pf PassFunc) ([] byte, [] byte, error) {
	x509Encoded, err := x509.MarshalPKCS8PrivateKey(keypair.private)
	if err != nil {
		return nil, nil, fmt.Errorf("x509 encoding private key: %w", err)
	}

	password := []byte{}
	if pf != nil {
		password, err = pf(true)
		if err != nil {
			return nil, nil, err
		}
	}

	encBytes, err := encrypted.Encrypt(x509Encoded, password)
	if err != nil {
		return nil, nil, err
	}

	// store in PEM format
	privBytes := pem.EncodeToMemory(&pem.Block{
		Bytes: encBytes,
		Type:  "ENCRYPTED COSIGN PRIVATE KEY",
	})

	// Now do the public key
	pubBytes, err := cryptoutils.MarshalPublicKeyToPEM(keypair.public)
	if err != nil {
		return nil, nil, err
	}

	return privBytes, pubBytes, nil
}

// PassFunc is the function to be called to retrieve the signer password. If
// nil, then it assumes that no password is provided.
type PassFunc func(bool) ([]byte, error)

type Keys struct {
	private crypto.PrivateKey
	public  crypto.PublicKey
}

func pass(s string) PassFunc {
	return func(_ bool) ([]byte, error) {
		return []byte(s), nil
	}
}
