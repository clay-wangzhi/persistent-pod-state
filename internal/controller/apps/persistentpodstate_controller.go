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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
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

	// 如果没有找到 PersistentPodState，尝试获取 VirtualMachineInstance 资源
	var virtualMachineInstance kubevirtv1.VirtualMachineInstance
	if err := r.Get(ctx, req.NamespacedName, &virtualMachineInstance); err == nil {
		klog.V(3).InfoS("Found a VirtualMachineInstance resource", "name", req.Name)
		// 处理 VirtualMachineInstance 逻辑
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentPodStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.PersistentPodState{}).
		Complete(r)
}
