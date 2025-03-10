/*
Copyright 2025 Clay.

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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePersistentPodStates implements PersistentPodStateInterface
type FakePersistentPodStates struct {
	Fake *FakeAppsV1alpha1
	ns   string
}

var persistentpodstatesResource = v1alpha1.SchemeGroupVersion.WithResource("persistentpodstates")

var persistentpodstatesKind = v1alpha1.SchemeGroupVersion.WithKind("PersistentPodState")

// Get takes name of the persistentPodState, and returns the corresponding persistentPodState object, and an error if there is any.
func (c *FakePersistentPodStates) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.PersistentPodState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(persistentpodstatesResource, c.ns, name), &v1alpha1.PersistentPodState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PersistentPodState), err
}

// List takes label and field selectors, and returns the list of PersistentPodStates that match those selectors.
func (c *FakePersistentPodStates) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PersistentPodStateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(persistentpodstatesResource, persistentpodstatesKind, c.ns, opts), &v1alpha1.PersistentPodStateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PersistentPodStateList{ListMeta: obj.(*v1alpha1.PersistentPodStateList).ListMeta}
	for _, item := range obj.(*v1alpha1.PersistentPodStateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested persistentPodStates.
func (c *FakePersistentPodStates) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(persistentpodstatesResource, c.ns, opts))

}

// Create takes the representation of a persistentPodState and creates it.  Returns the server's representation of the persistentPodState, and an error, if there is any.
func (c *FakePersistentPodStates) Create(ctx context.Context, persistentPodState *v1alpha1.PersistentPodState, opts v1.CreateOptions) (result *v1alpha1.PersistentPodState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(persistentpodstatesResource, c.ns, persistentPodState), &v1alpha1.PersistentPodState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PersistentPodState), err
}

// Update takes the representation of a persistentPodState and updates it. Returns the server's representation of the persistentPodState, and an error, if there is any.
func (c *FakePersistentPodStates) Update(ctx context.Context, persistentPodState *v1alpha1.PersistentPodState, opts v1.UpdateOptions) (result *v1alpha1.PersistentPodState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(persistentpodstatesResource, c.ns, persistentPodState), &v1alpha1.PersistentPodState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PersistentPodState), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePersistentPodStates) UpdateStatus(ctx context.Context, persistentPodState *v1alpha1.PersistentPodState, opts v1.UpdateOptions) (*v1alpha1.PersistentPodState, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(persistentpodstatesResource, "status", c.ns, persistentPodState), &v1alpha1.PersistentPodState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PersistentPodState), err
}

// Delete takes name of the persistentPodState and deletes it. Returns an error if one occurs.
func (c *FakePersistentPodStates) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(persistentpodstatesResource, c.ns, name, opts), &v1alpha1.PersistentPodState{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePersistentPodStates) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(persistentpodstatesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.PersistentPodStateList{})
	return err
}

// Patch applies the patch and returns the patched persistentPodState.
func (c *FakePersistentPodStates) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PersistentPodState, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(persistentpodstatesResource, c.ns, name, pt, data, subresources...), &v1alpha1.PersistentPodState{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.PersistentPodState), err
}
