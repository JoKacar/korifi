package env

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type EnvValueBuilder interface {
	BuildEnvValue(context.Context, corev1.ObjectReference) (map[string]string, error)
}

func BuildSecret(
	ctx context.Context,
	k8sClient client.Client,
	ownerRef corev1.ObjectReference,
	builder EnvValueBuilder,
	secretName string,
) error {
	log := logr.FromContextOrDiscard(ctx).WithName("reconcileVCAPSecret").WithValues("secretName", secretName)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ownerRef.Namespace,
		},
	}

	envValue, err := builder.BuildEnvValue(ctx, ownerRef)
	if err != nil {
		log.Info("failed to build env value", "reason", err)
		return err
	}

	_, err = controllerutil.CreateOrPatch(ctx, k8sClient, secret, func() error {
		secret.StringData = envValue

		return err
	})
	if err != nil {
		log.Info("unable to create or patch Secret", "reason", err)
		return err
	}

	return nil
}
