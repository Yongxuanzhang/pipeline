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
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)



func init() {
	os.Setenv("SYSTEM_NAMESPACE", namespace)
}



func TestVerifyInterface_Task(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	// get signerverifer
	sv, err := GetSignerVerifier(password)
	if err != nil {
		t.Fatalf("Failed to get SignerVerifier %v", err)
	}

	unsignedTask := GetUnsignedTask("test-task")

	signedTask, err := GetSignedTask(unsignedTask, sv)
	if err != nil {
		t.Fatalf("Failed to get signed task %v", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Name = "tampered"

	tcs := []struct {
		name    string
		task    *v1beta1.Task
		wantErr bool
	}{{
		name:    "Signed Task Pass Verification",
		task:    signedTask,
		wantErr: false,
	}, {
		name:    "Unsigned Task Fail Verification",
		task:    unsignedTask,
		wantErr: true,
	}, {
		name:    "task Fail Verification with empty task",
		task:    nil,
		wantErr: true,
	}, {
		name:    "Tampered task Fail Verification",
		task:    tamperedTask,
		wantErr: true,
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

			errs := VerifyInterface(ctx, tc.task, sv, signature)
			if (errs != nil) != tc.wantErr {
				t.Fatalf("VerifyInterface() get err %v, wantErr %t", err, tc.wantErr)
			}
		})
	}

}

func TestVerifyTask(t *testing.T) {
	ctx := logging.WithLogger(context.Background(), zaptest.NewLogger(t).Sugar())

	// Get Signer
	signer, secretpath, err := GetSignerFromFile(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	ctx = SetupConfigMapinContext(ctx, secretpath)

	unsignedTask := GetUnsignedTask("test-task")

	signedTask, err := GetSignedTask(unsignedTask, signer)
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedTask := signedTask.DeepCopy()
	tamperedTask.Name = "tampered"

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

	// Get Signer
	signer, secretpath, err := GetSignerFromFile(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	ctx = SetupConfigMapinContext(ctx, secretpath)

	unsignedPipeline := GetUnsignedPipeline("test-pipeline")

	signedPipeline, err := GetSignedPipeline(unsignedPipeline, signer)
	if err != nil {
		t.Fatal("fail to sign task", err)
	}

	tamperedPipeline := signedPipeline.DeepCopy()
	tamperedPipeline.Name = "tampered"

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
	unsigned := GetUnsignedTask("test-task").ObjectMeta

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
			Annotations: map[string]string{"foo":"bar"},
		},
		wantErr: true,
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			task, signature, err := prepareObjectMeta(*tc.objectmeta)
			if (err != nil) != tc.wantErr {
				t.Fatalf("prepareTask() get err %v, wantErr %t", err, tc.wantErr)
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

