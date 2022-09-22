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
	"context"
	"path/filepath"
	"testing"

	cosignsignature "github.com/sigstore/cosign/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	namespace = "trusted-resources"
	password  = "hello"
)

var (
	// tasks for testing
	taskSpecTest = &v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{
			Image: "ubuntu",
			Name:  "echo",
		}},
	}

	trTypeMeta = metav1.TypeMeta{
		Kind:       pipeline.TaskRunControllerName,
		APIVersion: "tekton.dev/v1beta1"}

	trObjectMeta = metav1.ObjectMeta{
		Name:      "tr",
		Namespace: namespace,
	}
)

func GetUnsignedTask(name string) *v1beta1.Task {
	return &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: *taskSpecTest,
	}
}

func GetSignedTask(unsigned *v1beta1.Task, signer signature.Signer) (*v1beta1.Task, error) {
	signedTask := unsigned.DeepCopy()
	signedTask.Name = "signed"
	if signedTask.Annotations == nil {
		signedTask.Annotations = map[string]string{}
	}
	signature, err := SignInterface(signer, signedTask)
	if err != nil {
		return nil, err
	}
	signedTask.Annotations[SignatureAnnotation] = signature
	return signedTask, nil
}

func GetUnsignedPipeline(name string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Pipeline"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

func GetSignedPipeline(unsigned *v1beta1.Pipeline, signer signature.Signer) (*v1beta1.Pipeline, error) {
	signedPipeline := unsigned.DeepCopy()
	signedPipeline.Name = "signed"
	if signedPipeline.Annotations == nil {
		signedPipeline.Annotations = map[string]string{}
	}
	signature, err := SignInterface(signer, signedPipeline)
	if err != nil {
		return nil, err
	}
	signedPipeline.Annotations[SignatureAnnotation] = signature
	return signedPipeline, nil
}

func SetupConfigMapinContext(ctx context.Context, secretpath string) context.Context {
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
			config.CosignPubKey: secretpath,
		},
	}
	store.OnConfigChanged(cm)

	cm_featureflag := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "feature-flags",
		},
		Data: map[string]string{
			"enable-api-fields": config.AlphaAPIFields,
			"verification-policy": config.EnforceVerificationPolicy,
		},
	}
	store.OnConfigChanged(cm_featureflag)

	return store.ToContext(ctx)
}

// GetSignerFromFile generates key files to tmpdir, return signer and pubkey path
func GetSignerFromFile(t *testing.T, ctx context.Context) (signature.Signer, string, error) {
	t.Helper()

	tmpDir := t.TempDir()
	privateKeyPath, _, err := GenerateKeyFile(tmpDir, pass(password))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := cosignsignature.SignerFromKeyRef(ctx, privateKeyPath, pass(password))
	if err != nil {
		t.Fatal(err)
	}

	return signer, filepath.Join(tmpDir, "cosign.pub"), nil
}
