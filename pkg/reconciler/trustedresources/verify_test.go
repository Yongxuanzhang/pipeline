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
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

const (
	namespace = "trusted-resources"
)

func TestVerifyInterface_Task(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	// get signerverifer
	sv, _, err := signature.NewDefaultECDSASignerVerifier()
	if err != nil {
		t.Fatalf("failed to get signerverifier %v", err)
	}

	unsignedTask := getUnsignedTask("test-task")

	signedTask, err := getSignedTask(unsignedTask, sv)
	if err != nil {
		t.Fatalf("Failed to get signed task %v", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Name = "tampered"

	tcs := []struct {
		name        string
		task        *v1beta1.Task
		expectedErr error
	}{{
		name:        "Signed Task Pass Verification",
		task:        signedTask,
		expectedErr: nil,
	}, {
		name:        "Unsigned Task Fail Verification",
		task:        unsignedTask,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	}, {
		name:        "task Fail Verification with empty task",
		task:        nil,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	}, {
		name:        "Tampered task Fail Verification",
		task:        tamperedTask,
		expectedErr: fmt.Errorf("invalid signature when validating ASN.1 encoded signature"),
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			signature := []byte{}

			if tc.task != nil {
				if sig, ok := tc.task.Annotations[SignatureAnnotation]; ok {
					delete(tc.task.Annotations, SignatureAnnotation)
					signature, err = base64.StdEncoding.DecodeString(sig)
					if err != nil {
						t.Fatal(err)
					}
				}
			}

			err := VerifyInterface(ctx, tc.task, sv, signature)
			if (err != nil) && (err.Error() != tc.expectedErr.Error()) {
				t.Fatalf("VerifyInterface() get err %v, wantErr %t", err, tc.expectedErr)
			}
		})
	}

}

func TestVerifyTask(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := ttesting.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	ctx = ttesting.SetupConfigMapinContext(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedTask := ttesting.GetUnsignedTask("test-task")

	signedTask, err := getSignedTask(unsignedTask, signer)
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Annotations["random"] = "attack"

	tcs := []struct {
		name    string
		task    v1beta1.TaskObject
		wantErr bool
	}{{
		name:    "Signed Task Passes Verification",
		task:    signedTask,
		wantErr: false,
	}, {
		name:    "Tampered Task Fails Verification with tampered content",
		task:    tamperedTask,
		wantErr: true,
	}, {
		name:    "Unsigned Task Fails Verification without signature",
		task:    unsignedTask,
		wantErr: true,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyTask(ctx, tc.task)
			if (err != nil) != tc.wantErr {
				t.Fatalf("verifyTaskRun() get err %v, wantErr %t", err, tc.wantErr)
			}
		})
	}

}

func TestVerifyPipeline(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	signer, keypath, err := ttesting.GetSignerFromFile(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	ctx = ttesting.SetupConfigMapinContext(ctx, keypath, config.EnforceResourceVerificationMode)

	unsignedPipeline := ttesting.GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := getSignedPipeline(unsignedPipeline, signer)
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Annotations["random"] = "attack"

	tcs := []struct {
		name     string
		pipeline v1beta1.PipelineObject
		wantErr  bool
	}{{
		name:     "Signed Pipeline Passes Verification",
		pipeline: signedPipeline,
		wantErr:  false,
	}, {
		name:     "Tampered Pipeline Fails Verification with tampered content",
		pipeline: tamperedPipeline,
		wantErr:  true,
	}, {
		name:     "Unsigned Pipeline Fails Verification without signature",
		pipeline: unsignedPipeline,
		wantErr:  true,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPipeline(ctx, tc.pipeline)
			if (err != nil) != tc.wantErr {
				t.Fatalf("VerifyPipeline() get err %v, wantErr %t", err, tc.wantErr)
			}
		})
	}

}

func TestPrepareObjectMeta(t *testing.T) {
	unsigned := ttesting.GetUnsignedTask("test-task").ObjectMeta

	signed := unsigned.DeepCopy()
	signed.Annotations = map[string]string{SignatureAnnotation: "tY805zV53PtwDarK3VD6dQPx5MbIgctNcg/oSle+MG0="}

	signedWithLabels := signed.DeepCopy()
	signedWithLabels.Labels = map[string]string{"label": "foo"}

	signedWithExtraAnnotations := signed.DeepCopy()
	signedWithExtraAnnotations.Annotations["kubectl-client-side-apply"] = "client"
	signedWithExtraAnnotations.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = "config"

	tcs := []struct {
		name       string
		objectmeta *metav1.ObjectMeta
		expected   metav1.ObjectMeta
		wantErr    bool
	}{{
		name:       "Prepare signed objectmeta without labels",
		objectmeta: signed,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		wantErr: false,
	}, {
		name:       "Prepare signed objectmeta with labels",
		objectmeta: signedWithLabels,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Labels:      map[string]string{"label": "foo"},
			Annotations: map[string]string{},
		},
		wantErr: false,
	}, {
		name:       "Prepare signed objectmeta with extra annotations",
		objectmeta: signedWithExtraAnnotations,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{},
		},
		wantErr: false,
	}, {
		name:       "Fail preparation without signature",
		objectmeta: &unsigned,
		expected: metav1.ObjectMeta{
			Name:        "test-task",
			Namespace:   namespace,
			Annotations: map[string]string{"foo": "bar"},
		},
		wantErr: true,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			task, signature, err := prepareObjectMeta(*tc.objectmeta)
			if (err != nil) != tc.wantErr {
				t.Fatalf("prepareObjectMeta() get err %v, wantErr %t", err, tc.wantErr)
			}
			if d := cmp.Diff(task, tc.expected); &tc.expected != nil && d != "" {
				t.Error(diff.PrintWantGot(d))
			}

			if tc.wantErr {
				return
			}
			if signature == nil {
				t.Fatal("signature is not extracted")
			}

		})
	}

}

func getUnsignedTask(name string) *v1beta1.Task {
	return &v1beta1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "tekton-pipelines",
			Annotations: map[string]string{"foo": "bar"},
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image: "ubuntu",
				Name:  "echo",
			}},
		},
	}
}

func getSignedTask(unsigned *v1beta1.Task, signer signature.Signer) (*v1beta1.Task, error) {
	signedTask := unsigned.DeepCopy()
	signedTask.Name = "signed"
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

func getSignedPipeline(unsigned *v1beta1.Pipeline, signer signature.Signer) (*v1beta1.Pipeline, error) {
	signedPipeline := unsigned.DeepCopy()
	signedPipeline.Name = "signed"
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
