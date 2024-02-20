package services

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"code.cloudfoundry.org/korifi/controllers/webhooks"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	ImmutableCredentialsSecretTypeErrorType = "ImmutableCredentialsSecretTypeErrorType"
	InvalidCredentialsFormatErrorType       = "InvalidCredentialsFormatErrorType"

	cfServiceInstanceKind = "CFServiceInstance"
)

var serviceCredentialsLog = logf.Log.WithName("cfserviceinstance-validate")

//+kubebuilder:webhook:path=/validate-v1-secret,mutating=false,failurePolicy=fail,sideEffects=None,resources=secrets,verbs=update,groups="",versions=v1,name=vcfserviceinstancecredentials.korifi.cloudfoundry.org,admissionReviewVersions={v1,v1beta1}

type CFServiceInstanceCredentialsValidator struct{}

var _ webhook.CustomValidator = &CFServiceInstanceCredentialsValidator{}

func NewCFServiceInstanceCredentialsValidator() *CFServiceInstanceCredentialsValidator {
	return &CFServiceInstanceCredentialsValidator{}
}

func (v *CFServiceInstanceCredentialsValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Secret{}).
		WithValidator(v).
		Complete()
}

func (v *CFServiceInstanceCredentialsValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *CFServiceInstanceCredentialsValidator) ValidateUpdate(ctx context.Context, oldObj, obj runtime.Object) (admission.Warnings, error) {
	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected oldObj to be a Secret but got a %T", obj))
	}

	newSecret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected obj to be a Secret but got a %T", obj))
	}

	if !isOwnedByCFServiceInstance(newSecret) {
		return nil, nil
	}

	oldCredentials, err := getCredentials(oldSecret)
	if err != nil {
		return nil, nil
	}

	newCredentials, err := getCredentials(newSecret)
	if err != nil {
		return nil, webhooks.ValidationError{
			Type:    InvalidCredentialsFormatErrorType,
			Message: fmt.Sprintf("Secret %q is has invalid format: %v", newSecret.Name, err.Error()),
		}.ExportJSONError()
	}

	if !reflect.DeepEqual(oldCredentials["type"], newCredentials["type"]) {
		return nil, webhooks.ValidationError{
			Type:    ImmutableCredentialsSecretTypeErrorType,
			Message: "Credentials secret type is immutable",
		}.ExportJSONError()
	}

	return nil, nil
}

func isOwnedByCFServiceInstance(secret *corev1.Secret) bool {
	for _, owner := range secret.OwnerReferences {
		if owner.Kind == cfServiceInstanceKind {
			return true
		}
	}

	return false
}

func getCredentials(credentialsSecret *corev1.Secret) (map[string]any, error) {
	credentials, ok := credentialsSecret.Data[korifiv1alpha1.CredentialsSecretKey]
	if !ok {
		return nil, fmt.Errorf(
			"data of secret %q does not contain the %q key",
			credentialsSecret.Name,
			korifiv1alpha1.CredentialsSecretKey,
		)
	}
	credentialsObject := map[string]any{}
	err := json.Unmarshal(credentials, &credentialsObject)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret data: %w", err)
	}

	return credentialsObject, nil
}

func (v *CFServiceInstanceCredentialsValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
