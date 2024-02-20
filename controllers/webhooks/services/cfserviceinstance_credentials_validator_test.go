package services_test

import (
	"context"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"code.cloudfoundry.org/korifi/controllers/webhooks/services"
	"code.cloudfoundry.org/korifi/tests/matchers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = FDescribe("CFServiceInstanceCredentialsValidatingWebhook", func() {
	var (
		ctx               context.Context
		originalSecret    *corev1.Secret
		validatingWebhook *services.CFServiceInstanceCredentialsValidator
	)

	BeforeEach(func() {
		ctx = context.Background()

		scheme := runtime.NewScheme()
		err := korifiv1alpha1.AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		originalSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: generateGUID("service-instance-credentials"),
			},
			Data: map[string][]byte{
				korifiv1alpha1.CredentialsSecretKey: []byte(`{"foo": "bar"}`),
			},
		}

		serviceInstance := &korifiv1alpha1.CFServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: generateGUID("service-instance"),
			},
			Spec: korifiv1alpha1.CFServiceInstanceSpec{
				DisplayName: generateGUID("service-instance"),
				Type:        korifiv1alpha1.UserProvidedType,
			},
		}

		Expect(controllerutil.SetOwnerReference(serviceInstance, originalSecret, scheme)).To(Succeed())

		validatingWebhook = services.NewCFServiceInstanceCredentialsValidator()
	})

	Describe("ValidateUpdate", func() {
		var (
			updatedSecret *corev1.Secret
			err           error
		)

		BeforeEach(func() {
			updatedSecret = originalSecret.DeepCopy()
			updatedSecret.Data[korifiv1alpha1.CredentialsSecretKey] = []byte(`{"a":"b"}`)
		})

		JustBeforeEach(func() {
			_, err = validatingWebhook.ValidateUpdate(ctx, originalSecret, updatedSecret)
		})

		It("allows the request", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		When("secret type is updated", func() {
			BeforeEach(func() {
				updatedSecret.Data = map[string][]byte{
					korifiv1alpha1.CredentialsSecretKey: []byte(`{"type":"my-type","foo": "bar"}`),
				}
			})

			It("denies the request", func() {
				Expect(err).To(matchers.BeValidationError(
					services.ImmutableCredentialsSecretTypeErrorType,
					Equal("Credentials secret type is immutable"),
				))
			})

			When("the secret is not owned by a service instance", func() {
				BeforeEach(func() {
					updatedSecret.OwnerReferences = []metav1.OwnerReference{}
				})

				It("allows the request", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Describe("invalid original secret", func() {
				When("the original secret is missing the credentials key", func() {
					BeforeEach(func() {
						originalSecret.Data = map[string][]byte{}
					})

					It("allows the request", func() {
						Expect(err).NotTo(HaveOccurred())
					})
				})

				When("the original secret credentials key is an invalid json", func() {
					BeforeEach(func() {
						originalSecret.Data = map[string][]byte{
							korifiv1alpha1.CredentialsSecretKey: []byte("invalid"),
						}
					})

					It("allows the request", func() {
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Describe("invalid updated secret", func() {
				When("the updated secret is missing the credentials key", func() {
					BeforeEach(func() {
						updatedSecret.Data = map[string][]byte{}
					})

					It("denies the request", func() {
						Expect(err).To(matchers.BeValidationError(
							services.InvalidCredentialsFormatErrorType,
							ContainSubstring("invalid format"),
						))
					})
				})

				When("the updated secret credentials key is an invalid json", func() {
					BeforeEach(func() {
						updatedSecret.Data = map[string][]byte{
							korifiv1alpha1.CredentialsSecretKey: []byte("invalid"),
						}
					})

					It("denies the request", func() {
						Expect(err).To(matchers.BeValidationError(
							services.InvalidCredentialsFormatErrorType,
							ContainSubstring("invalid format"),
						))
					})
				})
			})
		})
	})
})
