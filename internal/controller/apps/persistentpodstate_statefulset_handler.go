package apps

import (
	"context"
	"fmt"

	appsv1alpha1 "github.com/clay-wangzhi/persistent-pod-state/api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	KindStatefulSet = "StatefulSet"
)

var _ handler.TypedEventHandler[client.Object] = &enqueueRequestForStatefulSet{}

type enqueueRequestForStatefulSet struct {
	reader client.Reader
}

func (p *enqueueRequestForStatefulSet) Create(ctx context.Context, evt event.TypedCreateEvent[client.Object], q workqueue.RateLimitingInterface) {
	sts := evt.Object.(*appsv1.StatefulSet)
	if sts.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" &&
		(sts.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != "") {
		enqueueStatefulSetPersistentPodStateRequest(q, sts)
	}
}

func (p *enqueueRequestForStatefulSet) Delete(ctx context.Context, evt event.TypedDeleteEvent[client.Object], q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForStatefulSet) Generic(ctx context.Context, evt event.TypedGenericEvent[client.Object], q workqueue.RateLimitingInterface) {
}

func (p *enqueueRequestForStatefulSet) Update(ctx context.Context, evt event.TypedUpdateEvent[client.Object], q workqueue.RateLimitingInterface) {
	sts := evt.ObjectNew.(*appsv1.StatefulSet)
	if sts.Annotations[appsv1alpha1.AnnotationAutoGeneratePersistentPodState] == "true" &&
		(sts.Annotations[appsv1alpha1.AnnotationRequiredPersistentTopology] != "") {
		enqueueStatefulSetPersistentPodStateRequest(q, sts)
	}
}

func enqueueStatefulSetPersistentPodStateRequest(q workqueue.RateLimitingInterface, sts *appsv1.StatefulSet) {
	// name Format = generate#{workload.Kind}#{workload.Name}
	// example for generate#StatefulSet#web
	qName := fmt.Sprintf("%s#%s#%s", AutoGeneratePersistentPodStatePrefix, KindStatefulSet, sts.Name)
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: sts.Namespace,
		Name:      qName,
	}})
	klog.InfoS("Enqueue StatefulSet PersistentPodState request", "qName", qName)
}
