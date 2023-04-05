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

package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionutil "github.com/tektoncd/pipeline/pkg/internal/resolution"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResolvedTask contains the data that is needed to execute
// the TaskRun.
type ResolvedTask struct {
	TaskName string
	Kind     v1beta1.TaskKind
	TaskSpec *v1beta1.TaskSpec
	// TaskObjectMeta is used to store the Pipeline resolved task ObjectMeta to construct the task for trusted resources verification
	TaskObjectMeta *metav1.ObjectMeta
	// RefSource is used to store the RefSource of Pipeline task, the RefSource.URI is used to filter VerificationPolicies for trusted resources verification
	RefSource *v1beta1.RefSource
}

// GetTask is a function used to retrieve Tasks.
type GetTask func(context.Context, string) (*v1beta1.Task, *v1beta1.RefSource, error)

// GetTaskRun is a function used to retrieve TaskRuns
type GetTaskRun func(string) (*v1beta1.TaskRun, error)

// GetTaskData will retrieve the Task metadata and Spec associated with the
// provided TaskRun. This can come from a reference Task or from the TaskRun's
// metadata and embedded TaskSpec.
func GetTaskData(ctx context.Context, taskRun *v1beta1.TaskRun, getTask GetTask) (*resolutionutil.ResolvedObjectMeta, *v1beta1.TaskSpec, error) {
	taskMeta := metav1.ObjectMeta{}
	var refSource *v1beta1.RefSource
	taskSpec := v1beta1.TaskSpec{}
	switch {
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "":
		// Get related task for taskrun
		t, source, err := getTask(ctx, taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing tasks for taskRun %s: %w", taskRun.Name, err)
		}
		taskMeta = t.TaskMetadata()
		taskSpec = t.TaskSpec()
		refSource = source
	case taskRun.Spec.TaskSpec != nil:
		taskMeta = taskRun.ObjectMeta
		taskSpec = *taskRun.Spec.TaskSpec
		// TODO: if we want to set RefSource for embedded taskspec, set it here.
		// https://github.com/tektoncd/pipeline/issues/5522
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Resolver != "":
		task, source, err := getTask(ctx, taskRun.Name)
		switch {
		case err != nil:
			return nil, nil, err
		case task == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no task was returned")
		default:
			taskMeta = task.TaskMetadata()
			taskSpec = task.TaskSpec()
		}
		refSource = source
	default:
		return nil, nil, fmt.Errorf("taskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}

	return &resolutionutil.ResolvedObjectMeta{
		ObjectMeta: &taskMeta,
		RefSource:  refSource,
	}, &taskSpec, nil
}
