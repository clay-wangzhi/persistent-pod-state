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
	"strings"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	"github.com/clay-wangzhi/persistent-pod-state/pkg/features"
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

func shouldIgnoreIfNotVMIOrPod(req admission.Request) bool {
	// Ignore all calls to sub resources or resources other than pods or VMIs.
	if len(req.AdmissionRequest.SubResource) != 0 {
		return true
	}

	resource := req.AdmissionRequest.Resource.Resource
	return resource != "virtualmachineinstances" && resource != "pods"
}

// Handle handles admission requests.
func (h *PodMutatingHandler) Handle(ctx context.Context, req admission.Request) (resp admission.Response) {
	if shouldIgnoreIfNotVMIOrPod(req) {
		return admission.Allowed("")
	}

	// 处理 VMI 请求
	if req.AdmissionRequest.Resource.Resource == "virtualmachineinstances" {
		if !features.GlobalConfig.EnableVMIPersistence {
			klog.V(4).Infof("VMI persistence disabled, skipping VMI webhook for %s", req.Name)
			return admission.Allowed("")
		}
		return h.handleVMI(ctx, req)
	}

	// 处理 Pod 请求
	if req.AdmissionRequest.Resource.Resource == "pods" {
		if !features.GlobalConfig.EnableStatefulSetPersistence {
			klog.V(4).Infof("StatefulSet persistence disabled, skipping Pod webhook for %s", req.Name)
			return admission.Allowed("")
		}
		return h.handlePod(ctx, req)
	}

	return admission.Allowed("")
}

// 处理 VMI 请求
func (h *PodMutatingHandler) handleVMI(ctx context.Context, req admission.Request) admission.Response {
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
		klog.Infof("Enter Create VMI, namespace is %s, VMI name is %s", obj.Namespace, obj.Name)
		err = h.handleVMICreate(ctx, req, obj)
	case admissionv1.Update:
		klog.Infof("Enter Update VMI, namespace is %s, VMI name is %s", obj.Namespace, obj.Name)
		err = h.handleVMIUpdate(ctx, req, obj)
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
		klog.Errorf("Failed to marshal mutated VMI %s/%s, err: %v", obj.Namespace, obj.Name, err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	original, err := json.Marshal(clone)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(original, marshaled)
}

// 处理 Pod 请求
func (h *PodMutatingHandler) handlePod(ctx context.Context, req admission.Request) admission.Response {
	klog.Infof("开始处理 Pod 请求: %s, 操作: %s", req.Name, req.Operation)

	obj := &corev1.Pod{}
	err := h.Decoder.Decode(req, obj)
	if err != nil {
		klog.Errorf("解码 Pod 对象失败: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// 记录原始 Pod 信息
	klog.Infof("原始 Pod: %s/%s, 注解: %v, 亲和性: %v",
		obj.Namespace, obj.Name, obj.Annotations,
		obj.Spec.Affinity)

	clone := obj.DeepCopy()
	// when pod.namespace is empty, using req.namespace
	var isNamespaceEmpty bool
	if obj.Namespace == "" {
		obj.Namespace = req.Namespace
		isNamespaceEmpty = true
	}

	switch req.Operation {
	case admissionv1.Create:
		klog.Infof("Enter Create Pod, namespace is %s, Pod name is %s", obj.Namespace, obj.Name)
		err = h.handlePodCreate(ctx, req, obj)
	case admissionv1.Update:
		klog.Infof("Enter Update Pod, namespace is %s, Pod name is %s", obj.Namespace, obj.Name)
		err = h.handlePodUpdate(ctx, req, obj)
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

	// 记录修改后的 Pod 信息
	klog.Infof("修改后 Pod: %s/%s, 注解: %v, 亲和性: %v",
		obj.Namespace, obj.Name, obj.Annotations,
		obj.Spec.Affinity)

	// 检查是否有变化
	if reflect.DeepEqual(obj, clone) {
		klog.Infof("Pod %s/%s 没有变化，返回 Allowed", obj.Namespace, obj.Name)
		return admission.Allowed("")
	}

	// 生成 patch
	marshaled, err := json.Marshal(obj)
	if err != nil {
		klog.Errorf("序列化修改后的 Pod 失败: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	original, err := json.Marshal(clone)
	if err != nil {
		klog.Errorf("序列化原始 Pod 失败: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	klog.Infof("为 Pod %s/%s 生成 patch 成功", obj.Namespace, obj.Name)
	return admission.PatchResponseFromRaw(original, marshaled)
}

func (h *PodMutatingHandler) handleVMICreate(ctx context.Context, req admission.Request, obj *kubevirtv1.VirtualMachineInstance) error {
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
		return nil
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

func (h *PodMutatingHandler) handleVMIUpdate(ctx context.Context, req admission.Request, obj *kubevirtv1.VirtualMachineInstance) error {
	// TODO: add mutating logic for VMI update here
	return nil
}

func (h *PodMutatingHandler) handlePodCreate(ctx context.Context, req admission.Request, obj *corev1.Pod) error {
	klog.Infof("处理 Pod 创建请求: %s/%s, UID: %s", obj.Namespace, obj.Name, obj.UID)

	// 检查 Pod 是否已经有 OwnerReferences
	if len(obj.OwnerReferences) > 0 {
		klog.Infof("Pod %s/%s 的 OwnerReferences: %v", obj.Namespace, obj.Name, obj.OwnerReferences)
	} else {
		klog.Warningf("Pod %s/%s 没有 OwnerReferences", obj.Namespace, obj.Name)
	}

	// 如果没有annotations，初始化一个空的map
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}

	// 检查是否是 StatefulSet 的 Pod
	stsName := getStatefulSetNameFromPod(obj)
	if stsName == "" {
		return nil
	}

	// 从 persistentpodstate 中获取 IP
	pps := &appsv1alpha1.PersistentPodState{}
	err := h.Client.Get(ctx, types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      stsName,
	}, pps)
	if err != nil {
		klog.Errorf("获取 StatefulSet PersistentPodState %s/%s 失败: %v", obj.Namespace, stsName, err)
		return nil
	}

	// 添加 Calico 注解以固定 IP
	if pps.Status.PodStates[obj.Name].PodIP != "" {
		obj.Annotations["cni.projectcalico.org/ipAddrs"] = fmt.Sprintf("[\"%s\"]", pps.Status.PodStates[obj.Name].PodIP)
		klog.Infof("已为 Pod %s/%s 添加 Calico IP 固定注解: %s", obj.Namespace, obj.Name, pps.Status.PodStates[obj.Name].PodIP)
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
		klog.Infof("已为 Pod %s/%s 添加节点亲和性,指定节点: %s", obj.Namespace, obj.Name, pps.Status.PodStates[obj.Name].NodeName)
	}

	return nil
}

func (h *PodMutatingHandler) handlePodUpdate(ctx context.Context, req admission.Request, obj *corev1.Pod) error {
	// TODO: add mutating logic for pod update here
	return nil
}

// getStatefulSetNameFromPod 从 Pod 获取 StatefulSet 名称
func getStatefulSetNameFromPod(pod *corev1.Pod) string {
	// 检查 OwnerReferences
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "StatefulSet" {
			return ownerRef.Name
		}
	}

	// 检查 Pod 名称格式 (StatefulSet Pod 格式为 <statefulset-name>-<ordinal>)
	if pod.Name != "" {
		lastDashIndex := strings.LastIndex(pod.Name, "-")
		if lastDashIndex > 0 {
			suffix := pod.Name[lastDashIndex+1:]
			// 检查后缀是否为数字
			isNumeric := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					isNumeric = false
					break
				}
			}
			if isNumeric {
				return pod.Name[:lastDashIndex]
			}
		}
	}

	return ""
}
