package apps

import (
	"context"
	"fmt"
	"reflect"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	calicoapi "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	utilpointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StatefulSetAutoGeneratePersistentPodState 为 StatefulSet 的 Pod 生成 PersistentPodState
func (r *PersistentPodStateReconciler) StatefulSetAutoGeneratePersistentPodState(ctx context.Context, statefulSet *appsv1.StatefulSet) error {
	// 获取 StatefulSet 的所有 Pod
	podList := &corev1.PodList{}
	if err := r.Client.List(ctx, podList, client.InNamespace(statefulSet.Namespace), client.MatchingLabels(statefulSet.Spec.Selector.MatchLabels)); err != nil {
		klog.ErrorS(err, "获取 StatefulSet Pod 列表失败", "StatefulSet", klog.KRef(statefulSet.Namespace, statefulSet.Name))
		return err
	}

	// 如果没有 Pod，跳过处理
	if len(podList.Items) == 0 {
		klog.InfoS("StatefulSet 没有 Pod，跳过处理", "StatefulSet", klog.KRef(statefulSet.Namespace, statefulSet.Name))
		return nil
	}

	// 创建 PersistentPodState 对象
	persistentPodState := &appsv1alpha1.PersistentPodState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSet.Name,
			Namespace: statefulSet.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       statefulSet.Name,
					Controller: utilpointer.BoolPtr(true),
					UID:        statefulSet.UID,
				},
			},
		},
		Spec: appsv1alpha1.PersistentPodStateSpec{
			TargetReference: appsv1alpha1.TargetReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       statefulSet.Name,
			},
		},
		Status: appsv1alpha1.PersistentPodStateStatus{
			PodStates: make(map[string]appsv1alpha1.PodState),
		},
	}

	// 为每个 Pod 创建 PodState
	for _, pod := range podList.Items {
		// 跳过没有 IP 的 Pod
		if pod.Status.PodIP == "" || pod.Spec.NodeName == "" {
			continue
		}

		// 为 Pod 创建 IPReservation
		if err := r.createOrUpdateIPReservation(ctx, &pod); err != nil {
			klog.ErrorS(err, "处理 Pod IPReservation 失败", "Pod", klog.KRef(pod.Namespace, pod.Name))
			continue
		}

		// 添加 Pod 状态
		persistentPodState.Status.PodStates[pod.Name] = appsv1alpha1.PodState{
			NodeName: pod.Spec.NodeName,
			PodIP:    pod.Status.PodIP,
			NodeTopologyLabels: map[string]string{
				"kubernetes.io/hostname": pod.Spec.NodeName,
			},
		}
	}

	// 如果没有有效的 Pod 状态，跳过处理
	if len(persistentPodState.Status.PodStates) == 0 {
		klog.InfoS("没有有效的 Pod 状态，跳过处理", "StatefulSet", klog.KRef(statefulSet.Namespace, statefulSet.Name))
		return nil
	}

	// 检查是否已存在 PersistentPodState
	existingPersistentPodState := &appsv1alpha1.PersistentPodState{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: statefulSet.Namespace, Name: statefulSet.Name}, existingPersistentPodState)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// 不存在则创建
		if err := r.Create(ctx, persistentPodState); err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}
		klog.InfoS("创建 StatefulSet PersistentPodState 成功",
			"StatefulSet", klog.KRef(statefulSet.Namespace, statefulSet.Name),
			"persistentPodState", klog.KObj(persistentPodState))
		return nil
	}

	// 比较状态是否有变化
	if reflect.DeepEqual(existingPersistentPodState.Status, persistentPodState.Status) {
		return nil
	}

	if len(persistentPodState.Status.PodStates) < len(existingPersistentPodState.Status.PodStates) {
		return nil
	}

	// 更新状态
	objClone := &appsv1alpha1.PersistentPodState{}
	if err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err = r.Client.Get(ctx, client.ObjectKey{Namespace: persistentPodState.Namespace, Name: persistentPodState.Name}, objClone); err != nil {
			return err
		}
		objClone.Spec = *persistentPodState.Spec.DeepCopy()
		objClone.Status = *persistentPodState.Status.DeepCopy()
		return r.Client.Status().Update(ctx, objClone)
	}); err != nil {
		klog.ErrorS(err, "更新 PersistentPodState 失败", "persistentPodState", klog.KObj(persistentPodState))
		return err
	}

	klog.InfoS("更新 StatefulSet PersistentPodState 成功",
		"StatefulSet", klog.KRef(statefulSet.Namespace, statefulSet.Name),
		"oldStatus", dumpJSON(existingPersistentPodState.Status),
		"newStatus", dumpJSON(persistentPodState.Status))
	return nil
}

// createOrUpdateIPReservation 创建或更新 Pod 的 IPReservation
func (r *PersistentPodStateReconciler) createOrUpdateIPReservation(ctx context.Context, pod *corev1.Pod) error {
	if pod.Status.PodIP == "" {
		return nil
	}

	// 构建 IPReservation 对象
	ipReservation := &calicoapi.IPReservation{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("pod-%s-%s", pod.Namespace, pod.Name),
		},
		Spec: calicoapi.IPReservationSpec{
			ReservedCIDRs: []string{
				fmt.Sprintf("%s/32", pod.Status.PodIP),
			},
		},
	}

	// 检查 IPReservation 是否已存在
	existingIPReservation := &calicoapi.IPReservation{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: ipReservation.Name}, existingIPReservation)
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.ErrorS(err, "获取 IPReservation 失败", "Pod", klog.KRef(pod.Namespace, pod.Name))
			return err
		}
		// 不存在则创建
		if err := r.Client.Create(ctx, ipReservation); err != nil {
			klog.ErrorS(err, "创建 IPReservation 失败", "Pod", klog.KRef(pod.Namespace, pod.Name))
			return err
		}
		klog.InfoS("成功创建 IPReservation", "Pod", klog.KRef(pod.Namespace, pod.Name), "IP", pod.Status.PodIP)
		return nil
	}

	// 检查 ReservedCIDRs 是否一致，不一致才更新
	if !reflect.DeepEqual(existingIPReservation.Spec.ReservedCIDRs, ipReservation.Spec.ReservedCIDRs) {
		existingIPReservation.Spec.ReservedCIDRs = ipReservation.Spec.ReservedCIDRs
		if err := r.Client.Update(ctx, existingIPReservation); err != nil {
			klog.ErrorS(err, "更新 IPReservation 失败", "Pod", klog.KRef(pod.Namespace, pod.Name))
			return err
		}
		klog.InfoS("成功更新 IPReservation", "Pod", klog.KRef(pod.Namespace, pod.Name), "IP", pod.Status.PodIP)
	}

	return nil
}
