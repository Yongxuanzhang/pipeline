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

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// SignInterface returns the encoded signature for the given object.
func SignInterface(signer signature.Signer, i interface{}) ([]byte, error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is nil")
	}
	b, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write(b)

	sig, err := signer.SignMessage(bytes.NewReader(h.Sum(nil)))
	if err != nil {
		return nil, err
	}

	return sig, nil
}

// GetSignedPipeline signed the given pipeline and rename it with given name
func GetSignedPipeline(unsigned *v1beta1.Pipeline, signer signature.Signer, name string) (*v1beta1.Pipeline, error) {
	signedPipeline := unsigned.DeepCopy()
	signedPipeline.Name = name
	if signedPipeline.Annotations == nil {
		signedPipeline.Annotations = map[string]string{}
	}
	signature, err := SignInterface(signer, signedPipeline)
	if err != nil {
		return nil, err
	}
	signedPipeline.Annotations[SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedPipeline, nil
}

// GetSignedTask signed the given task and rename it with given name
func GetSignedTask(unsigned *v1beta1.Task, signer signature.Signer, name string) (*v1beta1.Task, error) {
	signedTask := unsigned.DeepCopy()
	signedTask.Name = name
	if signedTask.Annotations == nil {
		signedTask.Annotations = map[string]string{}
	}
	signature, err := SignInterface(signer, signedTask)
	if err != nil {
		return nil, err
	}
	signedTask.Annotations[SignatureAnnotation] = base64.StdEncoding.EncodeToString(signature)
	return signedTask, nil
}
