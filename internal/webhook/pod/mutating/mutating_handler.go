/*
Copyright 2022 The Koordinator Authors.

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

package mutating

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodMutatingHandler handles Pod
type PodMutatingHandler struct {
	Client client.Client

	// Decoder decodes objects
	Decoder admission.Decoder
}

var _ admission.Handler = &PodMutatingHandler{}

func shouldIgnoreIfNotVMI(req admission.Request) bool {
	// Ignore all calls to sub resources or resources other than pods.
	if len(req.AdmissionRequest.SubResource) != 0 ||
		req.AdmissionRequest.Resource.Resource != "virtualmachineinstances" {
		return true
	}
	return false
}

// Handle handles admission requests.
func (h *PodMutatingHandler) Handle(ctx context.Context, req admission.Request) (resp admission.Response) {
	if shouldIgnoreIfNotVMI(req) {
		return admission.Allowed("")
	}

	obj := &kubevirtv1.VirtualMachineInstance{}
	err := h.Decoder.Decode(req, obj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	clone := obj.DeepCopy()
	// when pod.namespace is empty, using req.namespace
	var isNamespaceEmpty bool
	if obj.Namespace == "" {
		obj.Namespace = req.Namespace
		isNamespaceEmpty = true
	}

	switch req.Operation {
	case admissionv1.Create:
		klog.Infof("Enter Create pod, namespace is %s, pod name is %s", obj.Namespace, obj.Name)
		err = h.handleCreate(ctx, req, obj)
	case admissionv1.Update:
		klog.Infof("Enter Update pod, namespace is %s, pod name is %s", obj.Namespace, obj.Name)
		err = h.handleUpdate(ctx, req, obj)
	default:
		return admission.Allowed("")
	}

	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Do not modify namespace in webhook
	if isNamespaceEmpty {
		obj.Namespace = ""
	}

	if reflect.DeepEqual(obj, clone) {
		return admission.Allowed("")
	}
	marshaled, err := json.Marshal(obj)
	if err != nil {
		klog.Errorf("Failed to marshal mutated Pod %s/%s, err: %v", obj.Namespace, obj.Name, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	original, err := json.Marshal(clone)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(original, marshaled)
}

func (h *PodMutatingHandler) handleCreate(ctx context.Context, req admission.Request, obj *kubevirtv1.VirtualMachineInstance) error {
	// 如果没有annotations，初始化一个空的map
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}

	// 从 persistentpodstate 中获取 IP
	pps := &appsv1alpha1.PersistentPodState{}
	ppsName := obj.Name
	err := h.Client.Get(ctx, types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      ppsName,
	}, pps)
	if err != nil {
		klog.Errorf("获取 PersistentPodState %s/%s 失败: %v", obj.Namespace, ppsName, err)
		return err
	}

	// 添加 Calico 注解以固定 IP
	if pps.Status.PodStates[obj.Name].PodIP != "" {
		obj.Annotations["cni.projectcalico.org/ipAddrs"] = fmt.Sprintf("[\"%s\"]", pps.Status.PodStates[obj.Name].PodIP)
		klog.Infof("已为 VMI %s/%s 添加 Calico IP 固定注解: %s", obj.Namespace, obj.Name, pps.Status.PodStates[obj.Name].PodIP)
	}
	// 根据 PersistentPodState 中的 NodeName 添加节点亲和性
	if pps.Status.PodStates[obj.Name].NodeName != "" {
		if obj.Spec.Affinity == nil {
			obj.Spec.Affinity = &corev1.Affinity{}
		}
		if obj.Spec.Affinity.NodeAffinity == nil {
			obj.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
		}
		if obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}

		nodeSelectorTerm := corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      "kubernetes.io/hostname",
					Operator: "In",
					Values:   []string{pps.Status.PodStates[obj.Name].NodeName},
				},
			},
		}

		obj.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{nodeSelectorTerm}
		klog.Infof("已为 VMI %s/%s 添加节点亲和性,指定节点: %s", obj.Namespace, obj.Name, pps.Status.PodStates[obj.Name].NodeName)
	}

	klog.Infof("Testing VMI print namespace is %s, VMIName is %s, UID is %s", obj.Namespace, obj.ObjectMeta.Name, obj.UID)

	return nil
}

func (h *PodMutatingHandler) handleUpdate(ctx context.Context, req admission.Request, obj *kubevirtv1.VirtualMachineInstance) error {
	// TODO: add mutating logic for pod update here
	return nil
}
