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

package testing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/theupdateframework/go-tuf/encrypted"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	namespace = "trusted-resources"
)

var (
	// tasks for testing
	taskSpecTest = &v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Image: "ubuntu",
			Name:  "echo",
		}},
	}
)

// GetUnsignedTask returns unsigned task with given name
func GetUnsignedTask(name string) *v1beta1.Task {
	return &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: *taskSpecTest,
	}
}

// GetUnsignedPipeline returns unsigned pipeline with given name
func GetUnsignedPipeline(name string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "task",
				},
			},
		},
	}
}

// SetupTrustedResourceConfig config the keys and feature flag for testing
func SetupTrustedResourceConfig(ctx context.Context, keypath string, resourceVerificationMode string) context.Context {
	store := config.NewStore(logging.FromContext(ctx).Named("config-store"))
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      config.TrustedTaskConfig,
		},
		Data: map[string]string{
			config.PublicKeys: keypath,
		},
	}
	store.OnConfigChanged(cm)

	featureflags := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "feature-flags",
		},
		Data: map[string]string{
			"enable-api-fields":          config.AlphaAPIFields,
			"resource-verification-mode": resourceVerificationMode,
		},
	}
	store.OnConfigChanged(featureflags)

	return store.ToContext(ctx)
}

// GetSignerFromFile generates key files to tmpdir, return signer and pubkey path
func GetSignerFromFile(ctx context.Context, t *testing.T) (signature.Signer, string, error) {
	t.Helper()

	tmpDir := t.TempDir()
	publicKeyFile := "ecdsa.pub"
	sv, err := GenerateKeyFile(tmpDir, publicKeyFile)
	if err != nil {
		t.Fatal(err)
	}

	return sv, filepath.Join(tmpDir, publicKeyFile), nil
}

// PassFunc is the function to be called to retrieve the signer password. If
// nil, then it assumes that no password is provided.
type PassFunc func(bool) ([]byte, error)

// GenerateKeyFile creates public key files, return the SignerVerifier
func GetKeyBytes(pf PassFunc) (signature.SignerVerifier, []byte,[]byte, error) {
	keys, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil,nil, err
	}

	x509Encoded, err := x509.MarshalPKCS8PrivateKey(keys)
	if err != nil {
		return nil, nil,nil, err
	}

	password := []byte{}
	if pf != nil {
		password, err = pf(true)
		if err != nil {
			return nil, nil,nil, err
		}
	}

	encBytes, err := encrypted.Encrypt(x509Encoded, password)
	if err != nil {
		return nil, nil,nil, err
	}

	privBytes := pem.EncodeToMemory(&pem.Block{
		Bytes: encBytes,
		Type:  "EC PRIVATE KEY",
	})
	if err != nil {
		return nil,nil,nil, err
	}

	// Now do the public key
	pubBytes, err := cryptoutils.MarshalPublicKeyToPEM(keys.Public())
	if err != nil {
		return  nil,nil,nil, err
	}

	sv, err := signature.LoadSignerVerifier(keys, crypto.SHA256)
	if err != nil {
		return nil,nil,nil, err
	}

	return sv, privBytes, pubBytes, nil
}

// GenerateKeyFile creates public key files, return the SignerVerifier
func GenerateKeyFile(dir string, pubkeyfile string) (signature.SignerVerifier, error) {
	keys, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Now do the public key
	pubBytes, err := cryptoutils.MarshalPublicKeyToPEM(keys.Public())
	if err != nil {
		return nil, err
	}

	pubKey := filepath.Join(dir, pubkeyfile)
	if err := os.WriteFile(pubKey, pubBytes, 0600); err != nil {
		return nil, err
	}

	sv, err := signature.LoadSignerVerifier(keys, crypto.SHA256)
	if err != nil {
		return nil, err
	}

	return sv, nil
}
