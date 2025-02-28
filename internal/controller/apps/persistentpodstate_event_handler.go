package apps

import (
	"context"
	"fmt"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	KindVmi                              = "VirtualMachineInstance"
	AutoGeneratePersistentPodStatePrefix = "generate"
)

var _ handler.TypedEventHandler[client.Object] = &enqueueRequestForVirtualMachineInstance{}

type enqueueRequestForVirtualMachineInstance struct {
	reader client.Reader
}

func (p *enqueueRequestForVirtualMachineInstance) Create(ctx context.Context, evt event.TypedCreateEvent[client.Object], q workqueue.RateLimitingInterface) {
	vmi := evt.Object.(*kubevirtv1.VirtualMachineInstance)
	if vmi.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" &&
		(vmi.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != "") && vmi.IsRunning() {
		enqueuePersistentPodStateRequest(q, vmi)
	}
}

func (p *enqueueRequestForVirtualMachineInstance) Delete(ctx context.Context, evt event.TypedDeleteEvent[client.Object], q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForVirtualMachineInstance) Generic(ctx context.Context, evt event.TypedGenericEvent[client.Object], q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForVirtualMachineInstance) Update(ctx context.Context, evt event.TypedUpdateEvent[client.Object], q workqueue.RateLimitingInterface) {
	vmi := evt.ObjectNew.(*kubevirtv1.VirtualMachineInstance)
	if vmi.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" &&
		(vmi.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != "") && vmi.IsRunning() {
		enqueuePersistentPodStateRequest(q, vmi)
	}
}

func enqueuePersistentPodStateRequest(q workqueue.RateLimitingInterface, vmi *kubevirtv1.VirtualMachineInstance) {
	// name Format = generate#{workload.Kind}#{workload.Name}
	// example for generate#VirtualMachineInstance#echoserver
	qName := fmt.Sprintf("%s#%s#%s", AutoGeneratePersistentPodStatePrefix, KindVmi, vmi.Name)
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: vmi.Namespace,
		Name:      qName,
	}})
	klog.V(3).InfoS("Enqueue PersistentPodState request", "qName", qName)
}
