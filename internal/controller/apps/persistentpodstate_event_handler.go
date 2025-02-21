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
	KindVmi                              = kubevirtv1.SchemeGroupVersion.WithKind("VirtualMachineInstance")
	AutoGeneratePersistentPodStatePrefix = "generate#"
)

var _ handler.TypedEventHandler[client.Object] = &enqueueRequestForVirtualMachineInstance{}

type enqueueRequestForVirtualMachineInstance struct {
	reader client.Reader
}

func (p *enqueueRequestForVirtualMachineInstance) Create(ctx context.Context, evt event.TypedCreateEvent[client.Object], q workqueue.RateLimitingInterface) {
	vmi := evt.Object.(*kubevirtv1.VirtualMachineInstance)
	if vmi.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" &&
		(vmi.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != "") {
		enqueuePersistentPodStateRequest(q, "create", KindVmi.GroupVersion().String(), KindVmi.Kind, vmi.Namespace, vmi.Name)
	}
}

func (p *enqueueRequestForVirtualMachineInstance) Delete(ctx context.Context, evt event.TypedDeleteEvent[client.Object], q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForVirtualMachineInstance) Generic(ctx context.Context, evt event.TypedGenericEvent[client.Object], q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForVirtualMachineInstance) Update(ctx context.Context, evt event.TypedUpdateEvent[client.Object], q workqueue.RateLimitingInterface) {
	oVmi := evt.ObjectOld.(*kubevirtv1.VirtualMachineInstance)
	nVmi := evt.ObjectNew.(*kubevirtv1.VirtualMachineInstance)
	if nVmi.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" {
		if oVmi.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] != nVmi.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] ||
			oVmi.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != nVmi.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] {
			enqueuePersistentPodStateRequest(q, "create", KindVmi.GroupVersion().String(), KindVmi.Kind, nVmi.Namespace, nVmi.Name)
			return
		}
		enqueuePersistentPodStateRequest(q, "update", KindVmi.GroupVersion().String(), KindVmi.Kind, nVmi.Namespace, nVmi.Name)
	}

}

func enqueuePersistentPodStateRequest(q workqueue.RateLimitingInterface, event, apiVersion, kind, ns, name string) {
	// name Format = generate#{apiVersion}#{workload.Kind}#{workload.Name}
	// example for generate#apps/v1#StatefulSet#echoserver
	qName := fmt.Sprintf("%s#%s%s#%s#%s", event, AutoGeneratePersistentPodStatePrefix, apiVersion, kind, name)
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: ns,
		Name:      qName,
	}})
	klog.V(3).InfoS("Enqueue PersistentPodState request", "qName", qName)
}
