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
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/kms"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// SignatureAnnotation is the key of signature in annotation map
	SignatureAnnotation = "tekton.dev/signature"
)

// VerifyInterface get the checksum of json marshalled object and verify it.
func VerifyInterface(
	ctx context.Context,
	obj interface{},
	verifier signature.Verifier,
	signature []byte,
) error {
	ts, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(ts)

	if err := verifier.VerifySignature(bytes.NewReader(signature), bytes.NewReader(h.Sum(nil))); err != nil {
		return err
	}

	return nil
}

// VerifyTask verifies the signature and public key against task
func VerifyTask(ctx context.Context, taskObj v1beta1.TaskObject) error {
	tm, signature, err := prepareObjectMeta(taskObj.TaskMetadata())
	if err != nil {
		return err
	}
	task := v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: tm,
		Spec:       taskObj.TaskSpec(),
	}
	verifiers, err := getVerifiers(ctx)
	if err != nil {
		return err
	}
	for _, verifier := range verifiers {
		if err := VerifyInterface(ctx, task, verifier, signature); err == nil {
			return nil
		}
	}
	return fmt.Errorf("Task %v in namespace %v fails verification", task.Name, task.Namespace)
}

// VerifyPipeline verifies the signature and public key against pipeline
func VerifyPipeline(ctx context.Context, pipelineObj v1beta1.PipelineObject) error {
	pm, signature, err := prepareObjectMeta(pipelineObj.PipelineMetadata())
	if err != nil {
		return err
	}
	pipeline := v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline"},
		ObjectMeta: pm,
		Spec:       pipelineObj.PipelineSpec(),
	}
	verifiers, err := getVerifiers(ctx)
	if err != nil {
		return err
	}

	for _, verifier := range verifiers {
		if err := VerifyInterface(ctx, pipeline, verifier, signature); err == nil {
			return nil
		}
	}

	return fmt.Errorf("Pipeline %v in namespace %v fails verification", pipeline.Name, pipeline.Namespace)
}

// prepareObjectMeta will filter the objectmeta and extract the signature.
func prepareObjectMeta(in metav1.ObjectMeta) (metav1.ObjectMeta, []byte, error) {
	out := metav1.ObjectMeta{}

	// exclude the fields populated by system.
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace

	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for k, v := range in.Labels {
			out.Labels[k] = v
		}
	}

	out.Annotations = make(map[string]string)
	for k, v := range in.Annotations {
		out.Annotations[k] = v
	}

	// exclude the annotations added by other components
	// Task annotations are unlikely to be changed, we need to make sure other components
	// like resolver doesn't modify the annotations, otherwise the verification will fail
	delete(out.Annotations, "kubectl-client-side-apply")
	delete(out.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

	// signature should be contained in annotation
	sig, ok := in.Annotations[SignatureAnnotation]
	if !ok {
		return out, nil, fmt.Errorf("signature is missing")
	}
	// extract signature
	signature, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return out, nil, err
	}
	delete(out.Annotations, SignatureAnnotation)

	return out, signature, nil
}

// getVerifiers get all verifiers from configmap
func getVerifiers(ctx context.Context) ([]signature.Verifier, error) {
	cfg := config.FromContextOrDefaults(ctx)
	verifiers := []signature.Verifier{}

	for key := range cfg.TrustedResources.Keys {
		v, err := loadPublicKey(ctx, key)
		if err == nil {
			verifiers = append(verifiers, v)
		}
	}

	if len(verifiers) == 0 {
		return verifiers, fmt.Errorf("no public keys are founded for verification")
	}

	return verifiers, nil
}

// loadPublicKey is a wrapper for VerifierForKeyRef, hardcoding SHA256 as the hash algorithm
func loadPublicKey(ctx context.Context, keyRef string) (verifier signature.Verifier, err error) {
	return verifierForKeyRef(ctx, keyRef, crypto.SHA256)
}

// verifierForKeyRef parses the given keyRef, loads the key and returns an appropriate
// verifier using the provided hash algorithm
func verifierForKeyRef(ctx context.Context, keyRef string, hashAlgorithm crypto.Hash) (verifier signature.Verifier, err error) {
	// The key could be plaintext, in a file, at a URL, or in KMS.
	if kmsKey, err := kms.Get(ctx, keyRef, hashAlgorithm); err == nil {
		// KMS specified
		return kmsKey, nil
	}

	raw, err := os.ReadFile(filepath.Clean(keyRef))
	if err != nil {
		return nil, err
	}

	// PEM encoded file.
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("pem to public key: %w", err)
	}

	return signature.LoadVerifier(pubKey, hashAlgorithm)
}
