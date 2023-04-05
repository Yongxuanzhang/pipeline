//go:build e2e
// +build e2e

/*
Copyright 2019 The Tekton Authors

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

package test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	v1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	kind            = "Wait"
	betaAPIVersion  = "wait.testing.tekton.dev/v1beta1"
	betaWaitTaskDir = "./custom-task-ctrls/wait-task-beta"
)

var (
	filterTypeMeta          = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")
	filterObjectMeta        = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "UID", "CreationTimestamp", "Generation", "ManagedFields")
	filterCondition         = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time", "Message")
	filterCustomRunStatus   = cmpopts.IgnoreFields(v1beta1.CustomRunStatusFields{}, "StartTime", "CompletionTime")
	filterRunStatus         = cmpopts.IgnoreFields(v1alpha1.RunStatusFields{}, "StartTime", "CompletionTime")
	filterPipelineRunStatus = cmpopts.IgnoreFields(v1beta1.PipelineRunStatusFields{}, "StartTime", "CompletionTime")
)

func applyV1Beta1Controller(t *testing.T) {
	t.Helper()
	t.Log("Creating Wait v1beta1.CustomRun Custom Task Controller...")
	cmd := exec.Command("ko", "apply", "-f", "./config/controller.yaml")
	cmd.Dir = betaWaitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func cleanUpV1Beta1Controller(t *testing.T) {
	t.Helper()
	t.Log("Tearing down Wait v1beta1.CustomRun Custom Task Controller...")
	cmd := exec.Command("ko", "delete", "-f", "./config/controller.yaml")
	cmd.Dir = betaWaitTaskDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to tear down Wait Custom Task Controller: %s, Output: %s", err, out)
	}
}

func TestWaitCustomTask_PipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a custom task controller
	applyV1Beta1Controller(t)
	// Cleanup the controller after finishing the test
	defer cleanUpV1Beta1Controller(t)

	for _, tc := range []struct {
		name                  string
		customRunDuration     string
		customRunTimeout      *metav1.Duration
		customRunRetries      int
		prTimeout             *metav1.Duration
		prConditionAccessorFn func(string) ConditionAccessorFn
		wantPrCondition       apis.Condition
		wantCustomRunStatus   v1beta1.CustomRunStatus
		wantRetriesStatus     []v1beta1.CustomRunStatus
	}{{
		name:                  "Wait Task Has Succeeded",
		customRunDuration:     "1s",
		prTimeout:             &metav1.Duration{Duration: time.Minute},
		prConditionAccessorFn: Succeed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: "Succeeded",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
						Reason: "DurationElapsed",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Is Running",
		customRunDuration:     "2s",
		prTimeout:             &metav1.Duration{Duration: time.Second * 5},
		prConditionAccessorFn: Running,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: "Running",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
						Reason: "Running",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Failed When PipelineRun Is Timeout",
		customRunDuration:     "2s",
		prTimeout:             &metav1.Duration{Duration: time.Second},
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "PipelineRunTimeout",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "Cancelled",
					},
				},
				ObservedGeneration: 2,
			},
		},
	}, {
		name:                  "Wait Task Failed on Timeout",
		customRunDuration:     "2s",
		customRunTimeout:      &metav1.Duration{Duration: time.Second},
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "Failed",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TimedOut",
					},
				},
				ObservedGeneration: 1,
			},
		},
	}, {
		name:                  "Wait Task Retries on Timeout",
		customRunDuration:     "2s",
		customRunTimeout:      &metav1.Duration{Duration: time.Second},
		customRunRetries:      1,
		prConditionAccessorFn: Failed,
		wantPrCondition: apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: "Failed",
		},
		wantCustomRunStatus: v1beta1.CustomRunStatus{
			Status: v1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: "TimedOut",
					},
				},
				ObservedGeneration: 1,
			},
		},
		wantRetriesStatus: []v1beta1.CustomRunStatus{
			{
				Status: v1.Status{
					Conditions: []apis.Condition{
						{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
							Reason: "TimedOut",
						},
					},
					ObservedGeneration: 1,
				},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prTimeout == nil {
				tc.prTimeout = &metav1.Duration{Duration: time.Minute}
			}
			p := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "wait",
						Timeout: tc.customRunTimeout,
						Retries: tc.customRunRetries,
						TaskRef: &v1beta1.TaskRef{
							APIVersion: betaAPIVersion,
							Kind:       kind,
						},
						Params: v1beta1.Params{{Name: "duration", Value: v1beta1.ParamValue{Type: "string", StringVal: tc.customRunDuration}}},
					}},
				},
			}
			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      helpers.ObjectNameForTest(t),
					Namespace: namespace,
				},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{
						Name: p.Name,
					},
					Timeout: tc.prTimeout,
				},
			}
			if _, err := c.V1beta1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline %q: %v", p.Name, err)
			}
			if _, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Wait for the PipelineRun to the desired state
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, tc.prConditionAccessorFn(pipelineRun.Name), string(tc.wantPrCondition.Type), v1beta1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %q completion to be %s: %s", pipelineRun.Name, string(tc.wantPrCondition.Type), err)
			}

			// Get actual pipelineRun
			gotPipelineRun, err := c.V1beta1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun %q: %v", pipelineRun.Name, err)
			}

			// Start to compose expected PipelineRun
			wantPipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: pipelineRun.Namespace,
					Labels: map[string]string{
						"tekton.dev/pipeline": p.Name,
					},
				},
				Spec: v1beta1.PipelineRunSpec{
					ServiceAccountName: "default",
					PipelineRef:        &v1beta1.PipelineRef{Name: p.Name},
					Timeout:            tc.prTimeout,
				},
				Status: v1beta1.PipelineRunStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							tc.wantPrCondition,
						},
					},
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						PipelineSpec: &v1beta1.PipelineSpec{
							Tasks: []v1beta1.PipelineTask{
								{
									Name:    "wait",
									Timeout: tc.customRunTimeout,
									Retries: tc.customRunRetries,
									TaskRef: &v1beta1.TaskRef{
										APIVersion: betaAPIVersion,
										Kind:       kind,
									},
									Params: v1beta1.Params{{
										Name:  "duration",
										Value: v1beta1.ParamValue{Type: "string", StringVal: tc.customRunDuration},
									}},
								},
							},
						},
					},
				},
			}

			// Compose wantStatus and wantChildStatusReferences.
			// We will look in the PipelineRunStatus.ChildReferences for wantChildStatusReferences, and will look at
			// the actual Run's status for wantCustomRunStatus.
			if len(gotPipelineRun.Status.ChildReferences) != 1 {
				t.Fatalf("PipelineRun had unexpected .status.childReferences; got %d, want 1", len(gotPipelineRun.Status.ChildReferences))
			}
			wantCustomRunName := gotPipelineRun.Status.ChildReferences[0].Name
			wantPipelineRun.Status.PipelineRunStatusFields.ChildReferences = []v1beta1.ChildStatusReference{{
				TypeMeta: runtime.TypeMeta{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
				},
				Name:             wantCustomRunName,
				PipelineTaskName: "wait",
			}}

			wantStatus := tc.wantCustomRunStatus
			if tc.wantRetriesStatus != nil {
				wantStatus.RetriesStatus = tc.wantRetriesStatus
			}

			// Get the CustomRun.
			gotCustomRun, err := c.V1beta1CustomRunClient.Get(ctx, wantCustomRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get CustomRun %q: %v", wantCustomRunName, err)
			}

			if d := cmp.Diff(wantPipelineRun, gotPipelineRun,
				filterTypeMeta,
				filterObjectMeta,
				filterCondition,
				filterCustomRunStatus,
				filterPipelineRunStatus,
			); d != "" {
				t.Errorf("-want, +got: %v", d)
			}

			// Compare the CustomRun's status to what we're expecting.
			if d := cmp.Diff(wantStatus, gotCustomRun.Status, filterCondition, filterCustomRunStatus); d != "" {
				t.Errorf("CustomRun status differed. -want, +got: %v", d)
			}
		})
	}
}

// updateConfigMap updates the config map for specified @name with values. We can't use the one from knativetest because
// it assumes that Data is already a non-nil map, and by default, it isn't!
func updateConfigMap(ctx context.Context, client kubernetes.Interface, name string, configName string, values map[string]string) error {
	configMap, err := client.CoreV1().ConfigMaps(name).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	for key, value := range values {
		configMap.Data[key] = value
	}

	_, err = client.CoreV1().ConfigMaps(name).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}

func resetConfigMap(ctx context.Context, t *testing.T, c *clients, namespace, configName string, values map[string]string) {
	t.Helper()
	if err := updateConfigMap(ctx, c.KubeClient, namespace, configName, values); err != nil {
		t.Log(err)
	}
}
