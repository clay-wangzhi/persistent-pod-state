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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	kubevirtv1 "kubevirt.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	if strings.Contains(req.Name, AutoGeneratePersistentPodStatePrefix+"kubevirt") {
		switch {
		case strings.Contains(req.Name, "create#"+AutoGeneratePersistentPodStatePrefix):
			klog.Info("PersistentPod.spec of kubevirt create/update info ", req.Name)
			return ctrl.Result{}, r.autoGeneratePersistentPodState(req)
		case strings.Contains(req.Name, "update#"+AutoGeneratePersistentPodStatePrefix):
			klog.Info("PersistentPod.status of kubevirt update info ", req.Name)
			arr := strings.Split(req.Name, "#")
			if len(arr) != 5 {
				klog.InfoS("Reconcile PersistentPodState kubevirt is invalid", "workload", req)
			}
			name := arr[4]

			persistentPodState := &appsv1alpha1.PersistentPodState{}
			err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: req.Namespace, Name: name}, persistentPodState)
			if err != nil {
				if errors.IsNotFound(err) {
					// Object not found, return.  Created objects are automatically garbage collected.
					// For additional cleanup logic use finalizers.
					return ctrl.Result{}, nil
				}
				// Error reading the object - requeue the request.
				return ctrl.Result{}, err
			}

			virtualMachineInstance := &kubevirtv1.VirtualMachineInstance{}
			err = r.Client.Get(context.TODO(), client.ObjectKey{Namespace: req.Namespace, Name: name}, virtualMachineInstance)
			if err != nil {
				return ctrl.Result{}, err
			}
			var vmiIP string
			var vmiNodeNmae string
			if len(virtualMachineInstance.Status.Interfaces) == 1 && virtualMachineInstance.Status.Interfaces[0].IP != "" {
				klog.Info(name, " vmi ip is ", virtualMachineInstance.Status.Interfaces[0].IP, " and nodename is ", virtualMachineInstance.Status.NodeName)
				vmiIP = virtualMachineInstance.Status.Interfaces[0].IP
				vmiNodeNmae = virtualMachineInstance.Status.NodeName
			} else {
				return ctrl.Result{}, err
			}
			newStatus := persistentPodState.Status.DeepCopy()
			if newStatus.PodStates == nil {
				newStatus.PodStates = make(map[string]appsv1alpha1.PodState)
			}
			nodeTopologyKeys := sets.NewString()
			if persistentPodState.Spec.RequiredPersistentTopology != nil {
				nodeTopologyKeys.Insert(persistentPodState.Spec.RequiredPersistentTopology.NodeTopologyKeys...)
			}
			podState := appsv1alpha1.PodState{
				NodeTopologyLabels: map[string]string{},
			}
			podState.NodeName = vmiNodeNmae
			podState.PodIP = vmiIP
			for _, key := range nodeTopologyKeys.List() {
				podState.NodeTopologyLabels[key] = vmiNodeNmae
			}
			newStatus.PodStates[name] = podState
			if reflect.DeepEqual(persistentPodState.Status, newStatus) {
				return ctrl.Result{}, nil
			}
			// update PersistentPodState status
			persistentPodStateClone := &appsv1alpha1.PersistentPodState{}
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				if err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: req.Namespace, Name: name}, persistentPodStateClone); err != nil {
					return err
				}
				persistentPodStateClone.Status = *newStatus
				return r.Client.Status().Update(context.TODO(), persistentPodStateClone)
			}); err != nil {
				klog.ErrorS(err, "Failed to update PersistentPodState status", "persistentPodState", klog.KObj(persistentPodState))
				return ctrl.Result{}, err

			}
			klog.InfoS("Updated PersistentPodState status success", "persistentPodState", klog.KObj(persistentPodState), "oldPodStatesCount", len(persistentPodState.Status.PodStates), "newPodStatesCount", len(newStatus.PodStates))
		}

	}

	return ctrl.Result{}, nil
}

// auto generate PersistentPodState crd
func (r *PersistentPodStateReconciler) autoGeneratePersistentPodState(req ctrl.Request) error {

	arr := strings.Split(req.Name, "#")
	if len(arr) != 5 {
		klog.InfoS("Reconcile PersistentPodState kubevirt is invalid", "workload", req)
		return nil
	}
	ns, name := req.Namespace, arr[4]

	virtualMachineInstance := &kubevirtv1.VirtualMachineInstance{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, virtualMachineInstance)
	if err != nil {
		return err
	}
	// fetch persistentPodState crd
	oldObj := &appsv1alpha1.PersistentPodState{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, oldObj)
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
		newObj := newVMIPersistentPodState(*virtualMachineInstance)
		// create new obj
		if oldObj == nil {
			if err = r.Create(context.TODO(), newObj); err != nil {
				if errors.IsAlreadyExists(err) {
					return nil
				}
				return err
			}
			klog.InfoS("Created VMI persistentPodState success", "VMI", klog.KRef(ns, name), "persistentPodState", klog.KObj(newObj))
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
		klog.InfoS("Updated persistentPodState success", "persistentPodState", klog.KObj(newObj), "oldSpec",
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
	klog.InfoS("Deleted StatefulSet persistentPodState", "statefulSet", klog.KRef(ns, name))
	return nil
}

// func (r *PersistentPodStateReconciler) updatePersistentPodStateStatus(req ctrl.Request) error {

// }

func newVMIPersistentPodState(virtualMachineInstance kubevirtv1.VirtualMachineInstance) *appsv1alpha1.PersistentPodState {
	obj := &appsv1alpha1.PersistentPodState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualMachineInstance.Name,
			Namespace: virtualMachineInstance.Namespace,
			// OwnerReferences: []metav1.OwnerReference{
			// 	{
			// 		APIVersion: "kubevirt.io/v1",
			// 		Kind:       "VirtualMachineInstance",
			// 		Name:       virtualMachineInstance.Name,
			// 		Controller: utilpointer.BoolPtr(true),
			// 		UID:        virtualMachineInstance.UID,
			// 	},
			// },
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
			&enqueueRequestForVirtualMachineInstance{reader: mgr.GetClient()},
		).
		Complete(r)
}
