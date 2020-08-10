/*


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

package controllers

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sandboxv1beta1 "morhidi.io/api/v1beta1"
)

// WebPageReconciler reconciles a WebPage object
type WebPageReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sandbox.morhidi.io,resources=webpages,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sandbox.morhidi.io,resources=webpages/status,verbs=get;update;patch

func (r *WebPageReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("webpage", req.NamespacedName)

	log.Info("starting reconcile")

	// Get custom resource
	var webpage sandboxv1beta1.WebPage
	if err := r.Get(ctx, req.NamespacedName, &webpage); err != nil {
		log.Error(err, "unable to fetch WebPage")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Object found", "content", webpage.Spec.Static)

	hostPathDirectory := corev1.HostPathDirectory

	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      webpage.Name,
			Namespace: webpage.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  webpage.Name,
					Image: "nginx:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "static",
							MountPath: "/usr/share/nginx/html",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "static",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: webpage.Spec.Static,
							Type: &hostPathDirectory,
						},
					},
				},
			},
		},
	}

	// For garbage collector to clean up resource
	if err := ctrl.SetControllerReference(&webpage, &pod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("webpage-controller")}

	err := r.Patch(ctx, &pod, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	webpage.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}

	if err = r.Status().Update(ctx, &webpage); err != nil {
		log.Error(err, "unable to update status")
	}

	log.Info("finished reconcile")
	return ctrl.Result{}, nil
}

func (r *WebPageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sandboxv1beta1.WebPage{}).
		Complete(r)
}
