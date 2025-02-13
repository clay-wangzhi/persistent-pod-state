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
	"sigs.k8s.io/controller-runtime/pkg/handler"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// 尝试获取 VirtualMachineInstance 资源
	var virtualMachineInstance kubevirtv1.VirtualMachineInstance
	if err := r.Get(ctx, req.NamespacedName, &virtualMachineInstance); err == nil {
		klog.InfoS("Found a VirtualMachineInstance resource", "name", req.Name)
		if virtualMachineInstance.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" && virtualMachineInstance.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != "" {
			return ctrl.Result{}, r.autoGeneratePersistentPodState(virtualMachineInstance)
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// auto generate PersistentPodState crd
func (r *PersistentPodStateReconciler) autoGeneratePersistentPodState(virtualMachineInstance kubevirtv1.VirtualMachineInstance) error {
	// req.Name Format = generate#{apiVersion}#{workload.Kind}#{workload.Name}
	// example for generate#apps/v1#StatefulSet#echoserver
	// arr := strings.Split(req.Name, "#")
	// if len(arr) != 4 {
	// 	klog.InfoS("Reconcile PersistentPodState workload is invalid", "workload", req)
	// 	return nil
	// }
	// // fetch workload
	// apiVersion, kind, ns, name := arr[1], arr[2], req.Namespace, arr[3]
	// workload, err := r.finder.GetScaleAndSelectorForRef(apiVersion, kind, ns, name, "")
	// if err != nil {
	// 	return err
	// } else if workload == nil {
	// 	klog.InfoS("Reconcile PersistentPodState workload is Not Found", "workload", req)
	// 	return nil
	// }

	ns := virtualMachineInstance.Namespace
	name := virtualMachineInstance.Name
	// fetch persistentPodState crd
	oldObj := &appsv1alpha1.PersistentPodState{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, oldObj)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// not found
		oldObj = nil
	}

	// auto generate persistentPodState crd object
	if virtualMachineInstance.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" {
		if virtualMachineInstance.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] == "" {
			klog.InfoS("VMI persistentPodState annotation was incomplete", "VMI", klog.KRef(ns, name))
			return nil
		}
		// vm := &kubevirtv1.VirtualMachine{}
		// err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: virtualMachineInstance.Namespace, Name: virtualMachineInstance.Name}, vm)
		// if err != nil {
		// 	return err
		// }
		newObj := newVMIPersistentPodState(virtualMachineInstance)
		// create new obj
		if oldObj == nil {
			if err = r.Create(context.TODO(), newObj); err != nil {
				if errors.IsAlreadyExists(err) {
					return nil
				}
				return err
			}
			klog.V(3).InfoS("Created VMI persistentPodState success", "VMI", klog.KRef(ns, name), "persistentPodState", klog.KObj(newObj))
			return nil
		}
		// compare with old object
		if reflect.DeepEqual(oldObj.Spec, newObj.Spec) {
			return nil
		}
		objClone := &appsv1alpha1.PersistentPodState{}
		if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err = r.Client.Get(context.TODO(), client.ObjectKey{Namespace: newObj.Namespace, Name: newObj.Name}, objClone); err != nil {
				return err
			}
			objClone.Spec = *newObj.Spec.DeepCopy()
			return r.Client.Update(context.TODO(), objClone)
		}); err != nil {
			klog.ErrorS(err, "Failed to update persistentPodState", "persistentPodState", klog.KObj(newObj))
			return err
		}
		klog.V(3).InfoS("Updated persistentPodState success", "persistentPodState", klog.KObj(newObj), "oldSpec",
			dumpJSON(oldObj.Spec), "newSpec", dumpJSON(newObj.Spec))
		return nil
	}

	// delete auto generated persistentPodState crd object
	if oldObj == nil {
		return nil
	}
	if err = r.Delete(context.TODO(), oldObj); err != nil {
		return err
	}
	klog.V(3).InfoS("Deleted StatefulSet persistentPodState", "statefulSet", klog.KRef(ns, name))
	return nil
}

func newVMIPersistentPodState(virtualMachineInstance kubevirtv1.VirtualMachineInstance) *appsv1alpha1.PersistentPodState {
	obj := &appsv1alpha1.PersistentPodState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualMachineInstance.Name,
			Namespace: virtualMachineInstance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "kubevirt.io/v1",
					Kind:       "VirtualMachineInstance",
					Name:       virtualMachineInstance.Name,
					Controller: utilpointer.BoolPtr(true),
					UID:        virtualMachineInstance.UID,
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
	}
	// required topology term
	if virtualMachineInstance.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] != "" {
		requiredTopologyKeys := strings.Split(virtualMachineInstance.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology], ",")
		obj.Spec.RequiredPersistentTopology = &appsv1alpha1.NodeTopologyTerm{
			NodeTopologyKeys: requiredTopologyKeys,
		}
	}

	return obj
}

// DumpJSON returns the JSON encoding
func dumpJSON(o interface{}) string {
	j, _ := json.Marshal(o)
	return string(j)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentPodStateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.PersistentPodState{}).
		Watches(
			&kubevirtv1.VirtualMachineInstance{},
			&handler.EnqueueRequestForObject{},
		).
		Complete(r)
}
