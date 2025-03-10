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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	utilpointer "k8s.io/utils/pointer"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	"github.com/clay-wangzhi/persistent-pod-state/pkg/features"
	calicoapi "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IPReservationSuffix = "-ip-reservation"
	CIDRSuffix          = "/32"
)

// PersistentPodStateReconciler reconciles a PersistentPodState object
type PersistentPodStateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachineinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.clay.io,resources=persistentpodstates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.clay.io,resources=persistentpodstates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.clay.io,resources=persistentpodstates/finalizers,verbs=update
// +kubebuilder:rbac:groups=projectcalico.org,resources=ipreservations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PersistentPodState object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *PersistentPodStateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// 处理 VMI 请求
	if strings.Contains(req.Name, AutoGeneratePersistentPodStatePrefix+"#"+KindVmi) {
		if !features.GlobalConfig.EnableVMIPersistence {
			klog.V(4).InfoS("VMI persistence disabled, skipping VMI request", "request", req)
			return ctrl.Result{}, nil
		}

		arr := strings.Split(req.Name, "#")
		if len(arr) != 3 {
			klog.InfoS("Reconcile PersistentPodState kubevirt is invalid", "workload", req)
			return ctrl.Result{}, nil
		}
		ns, name := req.Namespace, arr[2]
		virtualMachineInstance := &kubevirtv1.VirtualMachineInstance{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, virtualMachineInstance)
		if err != nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, r.VmiAutoGeneratePersistentPodState(ctx, virtualMachineInstance)
	}

	// 处理 StatefulSet 请求
	if strings.Contains(req.Name, AutoGeneratePersistentPodStatePrefix+"#"+KindStatefulSet) {
		if !features.GlobalConfig.EnableStatefulSetPersistence {
			klog.V(4).InfoS("StatefulSet persistence disabled, skipping StatefulSet request", "request", req)
			return ctrl.Result{}, nil
		}

		arr := strings.Split(req.Name, "#")
		if len(arr) != 3 {
			klog.InfoS("Reconcile PersistentPodState StatefulSet is invalid", "workload", req)
			return ctrl.Result{}, nil
		}
		ns, name := req.Namespace, arr[2]
		statefulSet := &appsv1.StatefulSet{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, statefulSet)
		if err != nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, r.StatefulSetAutoGeneratePersistentPodState(ctx, statefulSet)
	}

	// 检查 VMI 是否存在
	if features.GlobalConfig.EnableVMIPersistence {
		vmi := &kubevirtv1.VirtualMachineInstance{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, vmi); err == nil {
			return ctrl.Result{}, r.VmiAutoGeneratePersistentPodState(ctx, vmi)
		}
	}
	// 检查 StatefulSet 是否存在
	if features.GlobalConfig.EnableStatefulSetPersistence {
		statefulSet := &appsv1.StatefulSet{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, statefulSet); err == nil {
			return ctrl.Result{}, r.StatefulSetAutoGeneratePersistentPodState(ctx, statefulSet)
		}
	}

	return ctrl.Result{}, nil
}

func (r *PersistentPodStateReconciler) VmiAutoGeneratePersistentPodState(ctx context.Context, virtualMachineInstance *kubevirtv1.VirtualMachineInstance) error {
	if len(virtualMachineInstance.Status.Interfaces) == 0 || virtualMachineInstance.Status.Interfaces[0].IP == "" {
		klog.InfoS("VMI has no valid IP, skip auto generate PersistentPodState", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name))
		return nil
	}
	// 检查是否需要创建 Calico IPReservation
	if virtualMachineInstance.Status.Interfaces[0].IP != "" {
		// 构建 IPReservation 对象
		ipReservation := &calicoapi.IPReservation{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("vmi-%s-%s", virtualMachineInstance.Namespace, virtualMachineInstance.Name),
			},
			Spec: calicoapi.IPReservationSpec{
				ReservedCIDRs: []string{
					fmt.Sprintf("%s/32", virtualMachineInstance.Status.Interfaces[0].IP),
				},
			},
		}

		// 检查 IPReservation 是否已存在
		existingIPReservation := &calicoapi.IPReservation{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: ipReservation.Name}, existingIPReservation)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.ErrorS(err, "获取 IPReservation 失败", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name))
				return err
			}
			// 不存在则创建
			if err := r.Client.Create(ctx, ipReservation); err != nil {
				klog.ErrorS(err, "创建 IPReservation 失败", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name))
				return err
			}
			klog.InfoS("成功创建 IPReservation", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name), "IP", virtualMachineInstance.Status.Interfaces[0].IP)
		} else {
			// 检查 ReservedCIDRs 是否一致，不一致才更新
			if !reflect.DeepEqual(existingIPReservation.Spec.ReservedCIDRs, ipReservation.Spec.ReservedCIDRs) {
				existingIPReservation.Spec.ReservedCIDRs = ipReservation.Spec.ReservedCIDRs
				if err := r.Client.Update(ctx, existingIPReservation); err != nil {
					klog.ErrorS(err, "更新 IPReservation 失败", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name))
					return err
				}
				klog.InfoS("成功更新 IPReservation", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name), "IP", virtualMachineInstance.Status.Interfaces[0].IP)
			}
		}
	}
	persistentPodState := &appsv1alpha1.PersistentPodState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualMachineInstance.Name,
			Namespace: virtualMachineInstance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: virtualMachineInstance.OwnerReferences[0].APIVersion,
					Kind:       virtualMachineInstance.OwnerReferences[0].Kind,
					Name:       virtualMachineInstance.OwnerReferences[0].Name,
					Controller: utilpointer.BoolPtr(true),
					UID:        virtualMachineInstance.OwnerReferences[0].UID,
				},
			},
		},
		Spec: appsv1alpha1.PersistentPodStateSpec{
			TargetReference: appsv1alpha1.TargetReference{
				APIVersion: "kubevirt.io/v1",
				Kind:       "VirtualMachineInstance",
				Name:       virtualMachineInstance.Name,
			},
		},
		Status: appsv1alpha1.PersistentPodStateStatus{
			PodStates: map[string]appsv1alpha1.PodState{
				virtualMachineInstance.Name: {
					NodeName: virtualMachineInstance.Status.NodeName,
					PodIP:    virtualMachineInstance.Status.Interfaces[0].IP,
					NodeTopologyLabels: map[string]string{
						"kubernetes.io/hostname": virtualMachineInstance.Status.NodeName,
					},
				},
			},
		},
	}
	existingPersistentPodState := &appsv1alpha1.PersistentPodState{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: virtualMachineInstance.Namespace, Name: virtualMachineInstance.Name}, existingPersistentPodState)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// not found
		existingPersistentPodState = nil
	}
	if existingPersistentPodState == nil {
		// 创建新的 persistentPodState 对象
		if err := r.Create(ctx, persistentPodState); err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}
		klog.InfoS("创建 VMI persistentPodState 成功", "VMI", klog.KRef(virtualMachineInstance.Namespace, virtualMachineInstance.Name), "persistentPodState", klog.KObj(persistentPodState))
		return nil
	}
	// compare with old object
	if reflect.DeepEqual(existingPersistentPodState.Status, persistentPodState.Status) {
		return nil
	}
	objClone := &appsv1alpha1.PersistentPodState{}
	if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err = r.Client.Get(ctx, client.ObjectKey{Namespace: persistentPodState.Namespace, Name: persistentPodState.Name}, objClone); err != nil {
			return err
		}
		objClone.Spec = *persistentPodState.Spec.DeepCopy()
		objClone.Status = *persistentPodState.Status.DeepCopy()
		return r.Client.Status().Update(ctx, objClone)
	}); err != nil {
		klog.ErrorS(err, "Failed to update PersistentPodState", "persistentPodState", klog.KObj(persistentPodState))
		return err
	}
	klog.InfoS("Updated PersistentPodState success", "persistentPodState", klog.KObj(persistentPodState), "oldStatus",
		dumpJSON(existingPersistentPodState.Status), "newStatus", dumpJSON(persistentPodState.Status))
	return nil
}

// DumpJSON returns the JSON encoding
func dumpJSON(o interface{}) string {
	j, _ := json.Marshal(o)
	return string(j)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentPodStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 添加 Calico API scheme
	if err := calicoapi.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("unable to add Calico APIs to scheme: %v", err)
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.PersistentPodState{})

	// 根据配置决定是否监听 VMI
	if features.GlobalConfig.EnableVMIPersistence {
		builder = builder.Watches(
			&kubevirtv1.VirtualMachineInstance{},
			&enqueueRequestForVirtualMachineInstance{reader: mgr.GetClient()},
		)
		klog.InfoS("VMI persistence enabled, watching VirtualMachineInstance resources")
	} else {
		klog.InfoS("VMI persistence disabled, not watching VirtualMachineInstance resources")
	}

	// 根据配置决定是否监听 StatefulSet
	if features.GlobalConfig.EnableStatefulSetPersistence {
		builder = builder.Watches(
			&appsv1.StatefulSet{},
			&enqueueRequestForStatefulSet{reader: mgr.GetClient()},
		)
		klog.InfoS("StatefulSet persistence enabled, watching StatefulSet resources")
	} else {
		klog.InfoS("StatefulSet persistence disabled, not watching StatefulSet resources")
	}

	return builder.Complete(r)
}
