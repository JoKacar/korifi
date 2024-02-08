/*
Copyright 2021.

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

package services

import (
	"context"
	"encoding/json"
	"strings"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"code.cloudfoundry.org/korifi/controllers/controllers/shared"
	"code.cloudfoundry.org/korifi/tools/k8s"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CFServiceInstanceReconciler reconciles a CFServiceInstance object
type CFServiceInstanceReconciler struct {
	k8sClient client.Client
	scheme    *runtime.Scheme
	log       logr.Logger
}

func NewCFServiceInstanceReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
) *k8s.PatchingReconciler[korifiv1alpha1.CFServiceInstance, *korifiv1alpha1.CFServiceInstance] {
	serviceInstanceReconciler := CFServiceInstanceReconciler{k8sClient: client, scheme: scheme, log: log}
	return k8s.NewPatchingReconciler[korifiv1alpha1.CFServiceInstance, *korifiv1alpha1.CFServiceInstance](log, client, &serviceInstanceReconciler)
}

func (r *CFServiceInstanceReconciler) SetupWithManager(mgr ctrl.Manager) *builder.Builder {
	return ctrl.NewControllerManagedBy(mgr).
		For(&korifiv1alpha1.CFServiceInstance{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueSecretRequests),
		)
}

func (r *CFServiceInstanceReconciler) enqueueSecretRequests(ctx context.Context, o client.Object) []reconcile.Request {
	var requests []reconcile.Request

	secret, ok := o.(*corev1.Secret)
	if !ok {
		return []reconcile.Request{}
	}

	var serviceInstances korifiv1alpha1.CFServiceInstanceList
	err := r.k8sClient.List(
		ctx,
		&serviceInstances,
		client.InNamespace(secret.Namespace),
		client.MatchingFields{shared.IndexServiceInstanceBySecretName: secret.Name},
	)
	if err != nil {
		return []reconcile.Request{}
	}

	for _, inst := range serviceInstances.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      inst.Name,
				Namespace: inst.Namespace,
			},
		})
	}

	return requests
}

//+kubebuilder:rbac:groups=korifi.cloudfoundry.org,resources=cfserviceinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=korifi.cloudfoundry.org,resources=cfserviceinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=korifi.cloudfoundry.org,resources=cfserviceinstances/finalizers,verbs=update

func (r *CFServiceInstanceReconciler) ReconcileResource(ctx context.Context, cfServiceInstance *korifiv1alpha1.CFServiceInstance) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)

	cfServiceInstance.Status.ObservedGeneration = cfServiceInstance.Generation
	log.V(1).Info("set observed generation", "generation", cfServiceInstance.Status.ObservedGeneration)

	err := r.reconcileCredentials(ctx, cfServiceInstance)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CFServiceInstanceReconciler) reconcileCredentials(ctx context.Context, serviceInstance *korifiv1alpha1.CFServiceInstance) error {
	log := logr.FromContextOrDiscard(ctx).WithValues("service-instance-name", serviceInstance.Name)

	if serviceInstance.Status.Credentials.Name != "" {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceInstance.Spec.SecretName,
			Namespace: serviceInstance.Namespace,
		},
	}

	err := r.k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	if err != nil {
		log.Error(err, "service instance secret is not available")
		return err
	}

	if !strings.HasPrefix(string(secret.Type), "servicebinding.io/") {
		log.Info("applying credentials secret", "secret", secret.Name)
		serviceInstance.Status.Credentials.Name = secret.Name
		return nil
	}

	migratedSecret, err := r.migrateLegacySecret(ctx, serviceInstance, secret)
	if err != nil {
		log.Error(err, "failed to migrate legacy service instance secret")
		return err
	}

	serviceInstance.Status.Credentials.Name = migratedSecret.Name

	return nil
}

func (r *CFServiceInstanceReconciler) migrateLegacySecret(ctx context.Context, owner metav1.Object, legacySecret *corev1.Secret) (*corev1.Secret, error) {
	log := logr.FromContextOrDiscard(ctx)

	log.Info("migrating legacy secret", "legacy-secret-name", legacySecret.Name)
	migratedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.GetName() + "-migrated",
			Namespace: legacySecret.Namespace,
		},
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.k8sClient, migratedSecret, func() error {
		if migratedSecret.Data == nil {
			migratedSecret.Data = map[string][]byte{}
		}

		data := map[string]any{}
		for k, v := range legacySecret.Data {
			data[k] = string(v)
		}

		dataBytes, err := json.Marshal(data)
		if err != nil {
			log.Error(err, "failed to marshal backup secret data")
			return err
		}

		migratedSecret.Data["data"] = dataBytes
		return controllerutil.SetOwnerReference(owner, migratedSecret, r.scheme)
	})
	if err != nil {
		log.Error(err, "failed to create migrated secret")
		return nil, err
	}

	return migratedSecret, nil
}
