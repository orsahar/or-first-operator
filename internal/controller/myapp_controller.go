/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1alpha1 "github.com/orsahar/or-first-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MyAppReconciler reconciles a MyApp object
type MyAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.my.domain,resources=myapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.my.domain,resources=myapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.my.domain,resources=myapps/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *MyAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the MyApp instance
	var myApp appsv1alpha1.MyApp
	if err := r.Get(ctx, req.NamespacedName, &myApp); err != nil {
		if apierrors.IsNotFound(err) {
			// MyApp resource not found, could have been deleted after reconcile request
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request
		return ctrl.Result{}, err
	}

	// List all pods owned by this MyApp instance
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), client.MatchingLabels{"app": myApp.Name}); err != nil {
		return ctrl.Result{}, err
	}

	// Determine the desired number of replicas
	desiredReplicas := myApp.Spec.Size
	currentReplicas := int32(len(pods.Items))

	// Compare the desired and current number of replicas
	if currentReplicas < desiredReplicas {
		// Need to scale up
		for i := currentReplicas; i < desiredReplicas; i++ {
			newPod := newPodForCR(&myApp)
			if err := r.Create(ctx, newPod); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else if currentReplicas > desiredReplicas {
		// Need to scale down
		for i := currentReplicas; i > desiredReplicas; i-- {
			pod := pods.Items[i-1]
			if err := r.Delete(ctx, &pod); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Update the status if necessary
	if myApp.Status.Replicas != currentReplicas {
		myApp.Status.Replicas = currentReplicas
		if err := r.Status().Update(ctx, &myApp); err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.Info("Reconciliation completed", "desiredReplicas", desiredReplicas, "currentReplicas", currentReplicas)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.MyApp{}).
		Complete(r)
}

func newPodForCR(cr *appsv1alpha1.MyApp) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pod-%d", cr.Name, time.Now().UnixNano()),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "myapp-container",
				Image:   "busybox",
				Command: []string{"sh", "-c", "while true; do echo hello; sleep 10;done"},
			}},
		},
	}
}
