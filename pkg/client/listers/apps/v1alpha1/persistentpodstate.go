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
// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PersistentPodStateLister helps list PersistentPodStates.
// All objects returned here must be treated as read-only.
type PersistentPodStateLister interface {
	// List lists all PersistentPodStates in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.PersistentPodState, err error)
	// PersistentPodStates returns an object that can list and get PersistentPodStates.
	PersistentPodStates(namespace string) PersistentPodStateNamespaceLister
	PersistentPodStateListerExpansion
}

// persistentPodStateLister implements the PersistentPodStateLister interface.
type persistentPodStateLister struct {
	indexer cache.Indexer
}

// NewPersistentPodStateLister returns a new PersistentPodStateLister.
func NewPersistentPodStateLister(indexer cache.Indexer) PersistentPodStateLister {
	return &persistentPodStateLister{indexer: indexer}
}

// List lists all PersistentPodStates in the indexer.
func (s *persistentPodStateLister) List(selector labels.Selector) (ret []*v1alpha1.PersistentPodState, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PersistentPodState))
	})
	return ret, err
}

// PersistentPodStates returns an object that can list and get PersistentPodStates.
func (s *persistentPodStateLister) PersistentPodStates(namespace string) PersistentPodStateNamespaceLister {
	return persistentPodStateNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PersistentPodStateNamespaceLister helps list and get PersistentPodStates.
// All objects returned here must be treated as read-only.
type PersistentPodStateNamespaceLister interface {
	// List lists all PersistentPodStates in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.PersistentPodState, err error)
	// Get retrieves the PersistentPodState from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.PersistentPodState, error)
	PersistentPodStateNamespaceListerExpansion
}

// persistentPodStateNamespaceLister implements the PersistentPodStateNamespaceLister
// interface.
type persistentPodStateNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PersistentPodStates in the indexer for a given namespace.
func (s persistentPodStateNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PersistentPodState, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PersistentPodState))
	})
	return ret, err
}

// Get retrieves the PersistentPodState from the indexer for a given namespace and name.
func (s persistentPodStateNamespaceLister) Get(name string) (*v1alpha1.PersistentPodState, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("persistentpodstate"), name)
	}
	return obj.(*v1alpha1.PersistentPodState), nil
}
