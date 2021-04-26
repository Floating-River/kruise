/*
Copyright 2020 The Kruise Authors.

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

package daemonset

import (
	"context"
	"reflect"
	"testing"

	"github.com/onsi/gomega"
	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	utilpointer "k8s.io/utils/pointer"
	crmanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

func Test_maxRevision(t *testing.T) {
	type args struct {
		histories []*apps.ControllerRevision
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "GetMaxRevision",
			args: args{
				histories: []*apps.ControllerRevision{
					{
						Revision: 123456789,
					},
					{
						Revision: 213456789,
					},
					{
						Revision: 312456789,
					},
				},
			},
			want: 312456789,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maxRevision(tt.args.histories); got != tt.want {
				t.Errorf("maxRevision() = %v, want %v", got, tt.want)
			}
			t.Logf("maxRevision() = %v", tt.want)
		})
	}
}

func TestGetTemplateGeneration(t *testing.T) {
	type args struct {
		ds *appsv1alpha1.DaemonSet
	}
	constNum := int64(1000)
	tests := []struct {
		name    string
		args    args
		want    *int64
		wantErr bool
	}{
		{
			name: "GetTemplateGeneration",
			args: args{
				ds: &appsv1alpha1.DaemonSet{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							apps.DeprecatedTemplateGeneration: "1000",
						},
					},
					Spec:   appsv1alpha1.DaemonSetSpec{},
					Status: appsv1alpha1.DaemonSetStatus{},
				},
			},
			want:    &constNum,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTemplateGeneration(tt.args.ds)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTemplateGeneration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if *got != *tt.want {
				t.Errorf("GetTemplateGeneration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterDaemonPodsNodeToUpdate(t *testing.T) {
	now := metav1.Now()
	type testcase struct {
		name             string
		rolling          *appsv1alpha1.RollingUpdateDaemonSet
		hash             string
		nodeToDaemonPods map[string][]*corev1.Pod
		nodes            []*corev1.Node
		expectNodes      []string
	}

	tests := []testcase{
		{
			name: "Standard,partition=0",
			rolling: &appsv1alpha1.RollingUpdateDaemonSet{
				Type:      appsv1alpha1.StandardRollingUpdateType,
				Partition: utilpointer.Int32Ptr(0),
			},
			hash: "v2",
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"n1": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n2": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v2"}}},
				},
				"n3": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
			},
			expectNodes: []string{"n2", "n3", "n1"},
		},
		{
			name: "Standard,partition=1",
			rolling: &appsv1alpha1.RollingUpdateDaemonSet{
				Type:      appsv1alpha1.StandardRollingUpdateType,
				Partition: utilpointer.Int32Ptr(1),
			},
			hash: "v2",
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"n1": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v2"}}},
				},
				"n2": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n3": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
			},
			expectNodes: []string{"n1", "n3"},
		},
		{
			name: "Standard,partition=1,selector=1",
			rolling: &appsv1alpha1.RollingUpdateDaemonSet{
				Type:      appsv1alpha1.StandardRollingUpdateType,
				Partition: utilpointer.Int32Ptr(1),
				Selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"node-type": "canary"}},
			},
			hash: "v2",
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"n1": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n2": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v2"}}},
				},
				"n3": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n4": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
			},
			nodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "n1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n3", Labels: map[string]string{"node-type": "canary"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n4"}},
			},
			expectNodes: []string{"n2", "n3"},
		},
		{
			name: "Standard,partition=2,selector=3",
			rolling: &appsv1alpha1.RollingUpdateDaemonSet{
				Type:      appsv1alpha1.StandardRollingUpdateType,
				Partition: utilpointer.Int32Ptr(2),
				Selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"node-type": "canary"}},
			},
			hash: "v2",
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"n1": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n2": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v2"}}},
				},
				"n3": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n4": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
			},
			nodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "n1", Labels: map[string]string{"node-type": "canary"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n3", Labels: map[string]string{"node-type": "canary"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n4", Labels: map[string]string{"node-type": "canary"}}},
			},
			expectNodes: []string{"n2", "n4"},
		},
		{
			name: "Standard,partition=0,selector=3,terminating",
			rolling: &appsv1alpha1.RollingUpdateDaemonSet{
				Type:      appsv1alpha1.StandardRollingUpdateType,
				Partition: utilpointer.Int32Ptr(0),
				Selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"node-type": "canary"}},
			},
			hash: "v2",
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"n1": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
				"n2": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v2"}}},
				},
				"n3": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}, DeletionTimestamp: &now}},
				},
				"n4": {
					{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{apps.DefaultDaemonSetUniqueLabelKey: "v1"}}},
				},
			},
			nodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "n1", Labels: map[string]string{"node-type": "canary"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n3", Labels: map[string]string{"node-type": "canary"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "n4"}},
			},
			expectNodes: []string{"n3", "n2", "n1"},
		},
	}

	testFn := func(test *testcase, t *testing.T) {
		indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		for _, node := range test.nodes {
			if err := indexer.Add(node); err != nil {
				t.Fatalf("failed to add node into indexer: %v", err)
			}
		}
		nodeLister := corelisters.NewNodeLister(indexer)
		dsc := &ReconcileDaemonSet{nodeLister: nodeLister}

		ds := &appsv1alpha1.DaemonSet{Spec: appsv1alpha1.DaemonSetSpec{UpdateStrategy: appsv1alpha1.DaemonSetUpdateStrategy{
			Type:          appsv1alpha1.RollingUpdateDaemonSetStrategyType,
			RollingUpdate: test.rolling,
		}}}
		got, err := dsc.filterDaemonPodsNodeToUpdate(ds, test.hash, test.nodeToDaemonPods)
		if err != nil {
			t.Fatalf("failed to call filterDaemonPodsNodeToUpdate: %v", err)
		}
		if !reflect.DeepEqual(got, test.expectNodes) {
			t.Fatalf("expected %v, got %v", test.expectNodes, got)
		}
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			testFn(&tests[i], t)
		})
	}
}

func TestControlledHistories(t *testing.T) {
	// start test manager
	g := gomega.NewGomegaWithT(t)
	mgr, err := crmanager.New(cfg, crmanager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()
	stopMgr, mgrStopped := StartTestManager(mgr, g)
	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// create DaemonSet
	ds1 := newDaemonSet("ds1")
	err = c.Create(context.TODO(), ds1)
	if err != nil {
		t.Fatalf("create ds1 error, %v", err)
	}

	// define CR
	crOfDs1 := newControllerRevision(ds1.GetName()+"-x1", ds1.GetNamespace(), ds1.Spec.Template.Labels,
		[]metav1.OwnerReference{*metav1.NewControllerRef(ds1, controllerKind)})
	orphanCrInSameNsWithDs1 := newControllerRevision(ds1.GetName()+"-x2", ds1.GetNamespace(), ds1.Spec.Template.Labels, nil)
	orphanCrNotInSameNsWithDs1 := newControllerRevision(ds1.GetName()+"-x3", ds1.GetNamespace()+"-other", ds1.Spec.Template.Labels, nil)

	cases := []struct {
		name                      string
		manager                   *DaemonSetsController
		historyCRAll              []*apps.ControllerRevision
		expectControllerRevisions []*apps.ControllerRevision
	}{
		{
			name: "controller revision in the same namespace",
			manager: func() *DaemonSetsController {
				manager, _, _, err := newTestController(c, ds1, crOfDs1, orphanCrInSameNsWithDs1)
				if err != nil {
					t.Fatalf("error creating DaemonSets controller: %v", err)
				}
				return manager
			}(),
			historyCRAll:              []*apps.ControllerRevision{crOfDs1, orphanCrInSameNsWithDs1},
			expectControllerRevisions: []*apps.ControllerRevision{crOfDs1, orphanCrInSameNsWithDs1},
		},
		{
			name: "Skip adopt the controller revision in the other namespaces",
			manager: func() *DaemonSetsController {
				manager, _, _, err := newTestController(c, ds1, orphanCrNotInSameNsWithDs1)
				if err != nil {
					t.Fatalf("error creating DaemonSets controller: %v", err)
				}
				return manager
			}(),
			historyCRAll:              []*apps.ControllerRevision{orphanCrNotInSameNsWithDs1},
			expectControllerRevisions: []*apps.ControllerRevision{},
		},
	}

	for _, c := range cases {
		for _, h := range c.historyCRAll {
			err := c.manager.historyStore.Add(h)
			if err != nil {
				t.Fatalf("add CR to history store error, %v", err)
			}
		}

		crList, err := c.manager.controlledHistories(ds1)
		if err != nil {
			t.Fatalf("Test case: %s. Unexpected error: %v", c.name, err)
		}

		if len(crList) != len(c.expectControllerRevisions) {
			t.Errorf("Test case: %s, expect controllerrevision count %d but got:%d",
				c.name, len(c.expectControllerRevisions), len(crList))
		} else {
			// check controller revisions match
			for _, cr := range crList {
				found := false
				for _, expectCr := range c.expectControllerRevisions {
					if reflect.DeepEqual(cr, expectCr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Test case: %s, controllerrevision %v not expected",
						c.name, cr)
				}
			}
			t.Logf("Test case: %s done", c.name)
		}
	}
}
