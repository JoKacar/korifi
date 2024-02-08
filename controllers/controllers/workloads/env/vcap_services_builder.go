package env

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"code.cloudfoundry.org/korifi/controllers/controllers/shared"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const UserProvided = "user-provided"

type VCAPServicesEnvValueBuilder struct {
	k8sClient client.Client
}

func NewVCAPServicesEnvValueBuilder(k8sClient client.Client) *VCAPServicesEnvValueBuilder {
	return &VCAPServicesEnvValueBuilder{k8sClient: k8sClient}
}

func (b *VCAPServicesEnvValueBuilder) BuildEnvValue(ctx context.Context, cfApp *korifiv1alpha1.CFApp) (map[string]string, error) {
	serviceBindings := &korifiv1alpha1.CFServiceBindingList{}
	err := b.k8sClient.List(ctx, serviceBindings,
		client.InNamespace(cfApp.Namespace),
		client.MatchingFields{shared.IndexServiceBindingAppGUID: cfApp.Name},
	)
	if err != nil {
		return nil, fmt.Errorf("error listing CFServiceBindings: %w", err)
	}

	if len(serviceBindings.Items) == 0 {
		return map[string]string{"VCAP_SERVICES": "{}"}, nil
	}

	serviceEnvs := VCAPServices{}
	for _, currentServiceBinding := range serviceBindings.Items {
		// If finalizing do not append
		if !currentServiceBinding.DeletionTimestamp.IsZero() {
			continue
		}

		var serviceEnv ServiceDetails
		var serviceLabel string
		serviceEnv, serviceLabel, err = buildSingleServiceEnv(ctx, b.k8sClient, currentServiceBinding)
		if err != nil {
			return nil, err
		}

		serviceEnvs[serviceLabel] = append(serviceEnvs[serviceLabel], serviceEnv)
	}

	jsonVal, err := json.Marshal(serviceEnvs)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"VCAP_SERVICES": string(jsonVal),
	}, nil
}

func buildSingleServiceEnv(ctx context.Context, k8sClient client.Client, serviceBinding korifiv1alpha1.CFServiceBinding) (ServiceDetails, string, error) {
	if serviceBinding.Status.Binding.Name == "" {
		return ServiceDetails{}, "", fmt.Errorf("secret name not set for service binding %q", serviceBinding.Name)
	}

	serviceLabel := UserProvided

	serviceInstance := korifiv1alpha1.CFServiceInstance{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: serviceBinding.Namespace, Name: serviceBinding.Spec.Service.Name}, &serviceInstance)
	if err != nil {
		return ServiceDetails{}, "", fmt.Errorf("error fetching CFServiceInstance: %w", err)
	}

	if serviceInstance.Status.Credentials.Name == "" {
		return ServiceDetails{}, "", errors.New("service instance credentials secret not available")
	}

	secret := corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Namespace: serviceInstance.Namespace, Name: serviceInstance.Status.Credentials.Name}, &secret)
	if err != nil {
		return ServiceDetails{}, "", fmt.Errorf("error fetching CFServiceBinding credentials secret: %w", err)
	}

	if serviceInstance.Spec.ServiceLabel != nil && *serviceInstance.Spec.ServiceLabel != "" {
		serviceLabel = *serviceInstance.Spec.ServiceLabel
	}

	serviceDetails, err := fromServiceBinding(serviceBinding, serviceInstance, secret, serviceLabel)
	return serviceDetails, serviceLabel, err
}

func fromServiceBinding(
	serviceBinding korifiv1alpha1.CFServiceBinding,
	serviceInstance korifiv1alpha1.CFServiceInstance,
	credentialsSecret corev1.Secret,
	serviceLabel string,
) (ServiceDetails, error) {
	var serviceName string
	var bindingName *string

	if serviceBinding.Spec.DisplayName != nil {
		serviceName = *serviceBinding.Spec.DisplayName
		bindingName = serviceBinding.Spec.DisplayName
	} else {
		serviceName = serviceInstance.Spec.DisplayName
		bindingName = nil
	}

	tags := serviceInstance.Spec.Tags
	if tags == nil {
		tags = []string{}
	}

	credentials := map[string]any{}
	err := json.Unmarshal(credentialsSecret.Data["data"], &credentials)
	if err != nil {
		return ServiceDetails{}, fmt.Errorf("failed to unmarshal secret data for secret %q", credentialsSecret.Name)
	}

	return ServiceDetails{
		Label:          serviceLabel,
		Name:           serviceName,
		Tags:           tags,
		InstanceGUID:   serviceInstance.Name,
		InstanceName:   serviceInstance.Spec.DisplayName,
		BindingGUID:    serviceBinding.Name,
		BindingName:    bindingName,
		Credentials:    credentials,
		SyslogDrainURL: nil,
		VolumeMounts:   []string{},
	}, nil
}
