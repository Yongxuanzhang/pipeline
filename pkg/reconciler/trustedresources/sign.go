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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/signature"
)

// SignInterface returns the encoded signature for the given object.
func SignInterface(signer signature.Signer, i interface{}) (string, error) {
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

// GenerateKeyFile creates cosign key files
func GenerateKeyFile(dir string, pf cosign.PassFunc) (string, string, error) {
	keys, err := cosign.GenerateKeyPair(pf)
	if err != nil {
		return "", "", err
	}

	priKey := filepath.Join(dir, "cosign.key")
	if err := os.WriteFile(priKey, keys.PrivateBytes, 0600); err != nil {
		return "", "", err
	}

	pubKey := filepath.Join(dir, "cosign.pub")
	if err := os.WriteFile(pubKey, keys.PublicBytes, 0644); err != nil {
		return "", "", err
	}

	return priKey, pubKey, nil
}

// GetSignerVerifier creates SignerVerifier from given password
func GetSignerVerifier(password string) (signature.SignerVerifier, error) {
	keys, err := cosign.GenerateKeyPair(pass(password))
	if err != nil {
		return nil, err
	}
	sv, err := cosign.LoadPrivateKey(keys.PrivateBytes, []byte(password))
	if err != nil {
		return nil, err
	}
	return sv, nil
}

func pass(s string) cosign.PassFunc {
	return func(_ bool) ([]byte, error) {
		return []byte(s), nil
	}
}
