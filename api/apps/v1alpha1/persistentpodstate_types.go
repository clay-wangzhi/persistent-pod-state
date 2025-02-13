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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AnnotationAutoGeneratePersistentPodState indicates kruise will auto generate PersistentPodState object
	// Need to work with AnnotationRequiredPersistentTopology and AnnotationPreferredPersistentTopology
	AnnotationAutoGeneratePersistentPodState = "clay.io/auto-generate-persistent-pod-state"
	// AnnotationRequiredPersistentTopology Pod rebuilt topology required for node labels
	// for example kruise.io/required-persistent-topology: topology.kubernetes.io/zone[,xxx]
	// optional
	AnnotationRequiredPersistentTopology = "clay.io/required-persistent-topology"
)

// PersistentPodStateSpec defines the desired state of PersistentPodState
type PersistentPodStateSpec struct {
	// TargetReference contains enough information to let you identify an workload for PersistentPodState
	// Selector and TargetReference are mutually exclusive, TargetReference is priority to take effect
	// current support VirtualMachineInstance、StatefulSet
	TargetReference TargetReference `json:"targetRef"`

	// Persist the annotations information of the pods that need to be saved
	PersistentPodAnnotations []PersistentPodAnnotation `json:"persistentPodAnnotations,omitempty"`

	// Pod rebuilt topology required for node labels
	// for example kubernetes.io/hostname
	RequiredPersistentTopology *NodeTopologyTerm `json:"requiredPersistentTopology,omitempty"`
}

// TargetReference contains enough information to let you identify an workload
type TargetReference struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion"`
	// Kind of the referent.
	Kind string `json:"kind"`
	// Name of the referent.
	Name string `json:"name"`
}

type PersistentPodAnnotation struct {
	Key string `json:"key"`
}

type NodeTopologyTerm struct {
	// A list of node selector requirements by node's labels.
	NodeTopologyKeys []string `json:"nodeTopologyKeys"`
}

// PersistentPodStateStatus defines the observed state of PersistentPodState
type PersistentPodStateStatus struct {
	// observedGeneration is the most recent generation observed for this PersistentPodState. It corresponds to the
	// PersistentPodState's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64 `json:"observedGeneration"`
	// When the pod is ready, record some status information of the pod, such as: labels, annotations, topologies, etc.
	// map[string]PodState -> map[Pod.Name]PodState
	PodStates map[string]PodState `json:"podStates,omitempty"`
}

type PodState struct {
	// pod.spec.nodeName
	NodeName string `json:"nodeName,omitempty"`
	// node topology labels key=value
	// for example kubernetes.io/hostname=node-1
	NodeTopologyLabels map[string]string `json:"nodeTopologyLabels,omitempty"`
	// pod persistent annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	// pod.status.podIP
	PodIP string `json:"podIP,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PersistentPodState is the Schema for the persistentpodstates API
type PersistentPodState struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PersistentPodStateSpec   `json:"spec,omitempty"`
	Status PersistentPodStateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PersistentPodStateList contains a list of PersistentPodState
type PersistentPodStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PersistentPodState `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PersistentPodState{}, &PersistentPodStateList{})
}
