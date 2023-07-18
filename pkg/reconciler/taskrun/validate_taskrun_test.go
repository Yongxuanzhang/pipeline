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

package taskrun

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestValidateResolvedTask_ValidParams(t *testing.T) {
	ctx := context.Background()
	task := &pipeline.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: pipeline.TaskSpec{
			Steps: []pipeline.Step{{
				Image:   "myimage",
				Command: []string{"mycmd"},
			}},
			Params: []pipeline.ParamSpec{
				{
					Name: "foo",
					Type: pipeline.ParamTypeString,
				}, {
					Name: "bar",
					Type: pipeline.ParamTypeString,
				}, {
					Name: "zoo",
					Type: pipeline.ParamTypeString,
				}, {
					Name: "matrixParam",
					Type: pipeline.ParamTypeString,
				}, {
					Name: "include",
					Type: pipeline.ParamTypeString,
				}, {
					Name: "arrayResultRef",
					Type: pipeline.ParamTypeArray,
				}, {
					Name: "myObjWithoutDefault",
					Type: pipeline.ParamTypeObject,
					Properties: map[string]pipeline.PropertySpec{
						"key1": {},
						"key2": {},
					},
				}, {
					Name: "myObjWithDefault",
					Type: pipeline.ParamTypeObject,
					Properties: map[string]pipeline.PropertySpec{
						"key1": {},
						"key2": {},
						"key3": {},
					},
					Default: &pipeline.ParamValue{
						Type: pipeline.ParamTypeObject,
						ObjectVal: map[string]string{
							"key1": "val1-default",
							"key2": "val2-default", // key2 is also provided and will be overridden by taskrun
							// key3 will be provided by taskrun
						},
					},
				},
			},
		},
	}
	rtr := &resources.ResolvedTask{
		TaskSpec: &task.Spec,
	}
	p := pipeline.Params{{
		Name:  "foo",
		Value: *pipeline.NewStructuredValues("somethinggood"),
	}, {
		Name:  "bar",
		Value: *pipeline.NewStructuredValues("somethinggood"),
	}, {
		Name:  "arrayResultRef",
		Value: *pipeline.NewStructuredValues("$(results.resultname[*])"),
	}, {
		Name: "myObjWithoutDefault",
		Value: *pipeline.NewObject(map[string]string{
			"key1":      "val1",
			"key2":      "val2",
			"extra_key": "val3",
		}),
	}, {
		Name: "myObjWithDefault",
		Value: *pipeline.NewObject(map[string]string{
			"key2": "val2",
			"key3": "val3",
		}),
	}}
	m := &pipeline.Matrix{
		Params: pipeline.Params{{
			Name:  "zoo",
			Value: *pipeline.NewStructuredValues("a", "b", "c"),
		}, {
			Name: "matrixParam", Value: pipeline.ParamValue{Type: pipeline.ParamTypeArray, ArrayVal: []string{}},
		}},
		Include: []pipeline.IncludeParams{{
			Name: "build-1",
			Params: pipeline.Params{{
				Name: "include", Value: pipeline.ParamValue{Type: pipeline.ParamTypeString, StringVal: "string-1"},
			}},
		}},
	}
	if err := ValidateResolvedTask(ctx, p, m, rtr); err != nil {
		t.Errorf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}
}
func TestValidateResolvedTask_ExtraValidParams(t *testing.T) {
	ctx := context.Background()
	tcs := []struct {
		name   string
		task   pipeline.Task
		params pipeline.Params
		matrix *pipeline.Matrix
	}{{
		name: "extra-str-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeString,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "foo",
			Value: *pipeline.NewStructuredValues("string"),
		}, {
			Name:  "extrastr",
			Value: *pipeline.NewStructuredValues("extra"),
		}},
	}, {
		name: "extra-arr-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeString,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "foo",
			Value: *pipeline.NewStructuredValues("string"),
		}, {
			Name:  "extraArr",
			Value: *pipeline.NewStructuredValues("extra", "arr"),
		}},
	}, {
		name: "extra-obj-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeString,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "foo",
			Value: *pipeline.NewStructuredValues("string"),
		}, {
			Name: "myObjWithDefault",
			Value: *pipeline.NewObject(map[string]string{
				"key2": "val2",
				"key3": "val3",
			}),
		}},
	}, {
		name: "extra-param-matrix",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "include",
					Type: pipeline.ParamTypeString,
				}},
			},
		},
		params: pipeline.Params{{}},
		matrix: &pipeline.Matrix{
			Params: pipeline.Params{{
				Name:  "extraArr",
				Value: *pipeline.NewStructuredValues("extra", "arr"),
			}},
			Include: []pipeline.IncludeParams{{
				Name: "build-1",
				Params: pipeline.Params{{
					Name: "include", Value: *pipeline.NewStructuredValues("string"),
				}},
			}},
		},
	}}
	for _, tc := range tcs {
		rtr := &resources.ResolvedTask{
			TaskSpec: &tc.task.Spec,
		}
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResolvedTask(ctx, tc.params, tc.matrix, rtr); err != nil {
				t.Errorf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
			}
		})
	}
}

func TestValidateResolvedTask_InvalidParams(t *testing.T) {
	ctx := context.Background()
	tcs := []struct {
		name    string
		task    pipeline.Task
		params  pipeline.Params
		matrix  *pipeline.Matrix
		wantErr string
	}{{
		name: "missing-params-string",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeString,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "missing",
			Value: *pipeline.NewStructuredValues("somethingfun"),
		}},
		matrix:  &pipeline.Matrix{},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [foo]",
	}, {
		name: "missing-params-arr",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "missing",
			Value: *pipeline.NewStructuredValues("array", "param"),
		}},
		matrix:  &pipeline.Matrix{},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [foo]",
	}, {
		name: "invalid-string-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeString,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "foo",
			Value: *pipeline.NewStructuredValues("array", "param"),
		}},
		matrix:  &pipeline.Matrix{},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [foo]",
	}, {
		name: "invalid-arr-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{
			Name:  "foo",
			Value: *pipeline.NewStructuredValues("string"),
		}},
		matrix:  &pipeline.Matrix{},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [foo]",
	}, {name: "missing-param-in-matrix",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "bar",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{}},
		matrix: &pipeline.Matrix{
			Params: pipeline.Params{{
				Name:  "missing",
				Value: *pipeline.NewStructuredValues("foo"),
			}}},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [bar]",
	}, {
		name: "missing-param-in-matrix-include",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "bar",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{}},
		matrix: &pipeline.Matrix{
			Include: []pipeline.IncludeParams{{
				Name: "build-1",
				Params: pipeline.Params{{
					Name: "missing", Value: pipeline.ParamValue{Type: pipeline.ParamTypeString, StringVal: "string"},
				}},
			}},
		},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [bar]",
	}, {
		name: "invalid-arr-in-matrix-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "bar",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{}},
		matrix: &pipeline.Matrix{
			Params: pipeline.Params{{
				Name:  "bar",
				Value: *pipeline.NewStructuredValues("foo"),
			}}},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [bar]",
	}, {
		name: "invalid-str-in-matrix-include-param",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "bar",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{}},
		matrix: &pipeline.Matrix{
			Include: []pipeline.IncludeParams{{
				Name: "build-1",
				Params: pipeline.Params{{
					Name: "bar", Value: pipeline.ParamValue{Type: pipeline.ParamTypeString, StringVal: "string"},
				}},
			}},
		},
		wantErr: "invalid input params for task : param types don't match the user-specified type: [bar]",
	}, {
		name: "missing-params-obj",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "foo",
					Type: pipeline.ParamTypeArray,
				}},
			},
		},
		params: pipeline.Params{{
			Name: "missing-obj",
			Value: *pipeline.NewObject(map[string]string{
				"key1":    "val1",
				"misskey": "val2",
			}),
		}},
		matrix:  &pipeline.Matrix{},
		wantErr: "invalid input params for task : missing values for these params which have no default values: [foo]",
	}, {
		name: "missing object param keys",
		task: pipeline.Task{
			ObjectMeta: metav1.ObjectMeta{Name: "foo"},
			Spec: pipeline.TaskSpec{
				Params: []pipeline.ParamSpec{{
					Name: "myObjWithoutDefault",
					Type: pipeline.ParamTypeObject,
					Properties: map[string]pipeline.PropertySpec{
						"key1": {},
						"key2": {},
					},
				}},
			},
		},
		params: pipeline.Params{{
			Name: "myObjWithoutDefault",
			Value: *pipeline.NewObject(map[string]string{
				"key1":    "val1",
				"misskey": "val2",
			}),
		}},
		matrix:  &pipeline.Matrix{},
		wantErr: "invalid input params for task : missing keys for these params which are required in ParamSpec's properties map[myObjWithoutDefault:[key2]]",
	}}
	for _, tc := range tcs {
		rtr := &resources.ResolvedTask{
			TaskSpec: &tc.task.Spec,
		}
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResolvedTask(ctx, tc.params, tc.matrix, rtr)
			if d := cmp.Diff(tc.wantErr, err.Error()); d != "" {
				t.Errorf("Did not get the expected Error  %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestValidateOverrides(t *testing.T) {
	tcs := []struct {
		name    string
		ts      *pipeline.TaskSpec
		trs     *v1.TaskRunSpec
		wantErr bool
	}{{
		name: "valid stepOverrides",
		ts: &pipeline.TaskSpec{
			Steps: []pipeline.Step{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1.TaskRunSpec{
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "step1",
			}},
		},
	}, {
		name: "valid sidecarOverrides",
		ts: &pipeline.TaskSpec{
			Sidecars: []pipeline.Sidecar{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1.TaskRunSpec{
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				Name: "step1",
			}},
		},
	}, {
		name: "invalid stepOverrides",
		ts:   &pipeline.TaskSpec{},
		trs: &v1.TaskRunSpec{
			StepSpecs: []v1.TaskRunStepSpec{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}, {
		name: "invalid sidecarOverrides",
		ts:   &pipeline.TaskSpec{},
		trs: &v1.TaskRunSpec{
			SidecarSpecs: []v1.TaskRunSidecarSpec{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOverrides(tc.ts, tc.trs)
			if (err != nil) != tc.wantErr {
				t.Errorf("expected err: %t, but got err %s", tc.wantErr, err)
			}
		})
	}
}

func TestValidateResult(t *testing.T) {
	tcs := []struct {
		name    string
		tr      *v1.TaskRun
		rtr     *pipeline.TaskSpec
		wantErr bool
	}{{
		name: "valid taskrun spec results",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "valid taskspec results",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1.ResultsTypeArray,
						},
						{
							Name: "object-result",
							Type: v1.ResultsTypeObject,
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "invalid taskrun spec results types",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results types",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeArray,
							Value: *v1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{
				{
					Name: "string-result",
					Type: pipeline.ResultsTypeString,
				},
				{
					Name: "array-result",
					Type: pipeline.ResultsTypeArray,
				},
				{
					Name:       "object-result",
					Type:       pipeline.ResultsTypeObject,
					Properties: map[string]pipeline.PropertySpec{"hello": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results object properties",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"world": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results object properties",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{
				{
					Name:       "object-result",
					Type:       pipeline.ResultsTypeObject,
					Properties: map[string]pipeline.PropertySpec{"world": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results types with other valid types",
		tr: &v1.TaskRun{
			Spec: v1.TaskRunSpec{
				TaskSpec: &v1.TaskSpec{
					Results: []v1.TaskResult{
						{
							Name: "string-result",
							Type: v1.ResultsTypeString,
						},
						{
							Name: "array-result-1",
							Type: v1.ResultsTypeArray,
						}, {
							Name: "array-result-2",
							Type: v1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1.ResultsTypeObject,
							Properties: map[string]v1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					Results: []v1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result-1",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						}, {
							Name:  "array-result-2",
							Type:  v1.ResultsTypeString,
							Value: *v1.NewStructuredValues(""),
						},
						{
							Name:  "object-result",
							Type:  v1.ResultsTypeObject,
							Value: *v1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &pipeline.TaskSpec{
			Results: []pipeline.TaskResult{},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTaskRunResults(tc.tr, tc.rtr)
			if err == nil && tc.wantErr {
				t.Errorf("expected err: %t, but got different err: %s", tc.wantErr, err)
			} else if err != nil && !tc.wantErr {
				t.Errorf("did not expect any err, but got err: %s", err)
			}
		})
	}
}
