package credentials_test

import (
	"fmt"

	korifiv1alpha1 "code.cloudfoundry.org/korifi/controllers/api/v1alpha1"
	"code.cloudfoundry.org/korifi/controllers/controllers/services/credentials"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Credentials", func() {
	var (
		err               error
		credentialsSecret *corev1.Secret
	)

	BeforeEach(func() {
		credentialsSecret = &corev1.Secret{
			Data: map[string][]byte{
				korifiv1alpha1.CredentialsSecretKey: []byte(`{"foo":{"bar": "baz"}}`),
			},
		}
	})

	Describe("GetBindingSecretType", func() {
		var secretType corev1.SecretType

		JustBeforeEach(func() {
			secretType, err = credentials.GetBindingSecretType(credentialsSecret)
		})

		It("returns user-provided type by default", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(secretType).To(BeEquivalentTo(credentials.ServiceBindingSecretTypePrefix + "user-provided"))
		})

		When("the type is specified in the credentials", func() {
			BeforeEach(func() {
				credentialsSecret.Data[korifiv1alpha1.CredentialsSecretKey] = []byte(`{"type":"my-type"}`)
			})

			It("returns the speicified type", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(secretType).To(BeEquivalentTo(credentials.ServiceBindingSecretTypePrefix + "my-type"))
			})
		})

		When("the type is not a plain string", func() {
			BeforeEach(func() {
				credentialsSecret.Data[korifiv1alpha1.CredentialsSecretKey] = []byte(`{"type":{"my-type":"is-not-a-string"}}`)
			})

			It("returns user-provided type", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(secretType).To(BeEquivalentTo(credentials.ServiceBindingSecretTypePrefix + "user-provided"))
			})
		})

		When("the credentials cannot be unmarshalled", func() {
			BeforeEach(func() {
				credentialsSecret.Data[korifiv1alpha1.CredentialsSecretKey] = []byte("invalid")
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal secret data")))
			})
		})

		When("the credentials key is missing from the secret data", func() {
			BeforeEach(func() {
				credentialsSecret.Data = map[string][]byte{
					"foo": {},
				}
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring(
					fmt.Sprintf("does not contain the %q key", korifiv1alpha1.CredentialsSecretKey),
				)))
			})
		})
	})

	Describe("GetBindingSecretData", func() {
		var bindingSecretData map[string]string

		JustBeforeEach(func() {
			bindingSecretData, err = credentials.GetBindingSecretData(credentialsSecret)
		})

		It("converts the credentials into a flat strings map", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(bindingSecretData).To(MatchAllKeys(Keys{
				"foo": Equal(`{"bar":"baz"}`),
			}))
		})

		When("the credentials key is missing from the secret data", func() {
			BeforeEach(func() {
				credentialsSecret.Data = map[string][]byte{
					"foo": {},
				}
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring(
					fmt.Sprintf("does not contain the %q key", korifiv1alpha1.CredentialsSecretKey),
				)))
			})
		})

		When("the credentials cannot be unmarshalled", func() {
			BeforeEach(func() {
				credentialsSecret.Data[korifiv1alpha1.CredentialsSecretKey] = []byte("invalid")
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to unmarshal secret data")))
			})
		})
	})
})
