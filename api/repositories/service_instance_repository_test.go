package repositories_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/cf-k8s-controllers/api/repositories"
	servicesv1alpha1 "code.cloudfoundry.org/cf-k8s-controllers/controllers/apis/services/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	hnsv1alpha2 "sigs.k8s.io/hierarchical-namespaces/api/v1alpha2"
)

var _ = Describe("ServiceInstanceRepository", func() {
	var (
		testCtx             context.Context
		serviceInstanceRepo *repositories.ServiceInstanceRepo

		org                 *hnsv1alpha2.SubnamespaceAnchor
		space               *hnsv1alpha2.SubnamespaceAnchor
		serviceInstanceName string
	)

	BeforeEach(func() {
		testCtx = context.Background()
		serviceInstanceRepo = repositories.NewServiceInstanceRepo(namespaceRetriever, userClientFactory, nsPerms)

		org = createOrgAnchorAndNamespace(testCtx, rootNamespace, prefixedGUID("org"))
		space = createSpaceAnchorAndNamespace(testCtx, org.Name, prefixedGUID("space1"))
		serviceInstanceName = prefixedGUID("service-instance")
	})

	Describe("CreateServiceInstance", func() {
		var (
			serviceInstanceCreateMessage repositories.CreateServiceInstanceMessage
			serviceInstanceTags          []string
		)

		BeforeEach(func() {
			serviceInstanceTags = []string{"foo", "bar"}

			serviceInstanceCreateMessage = initializeServiceInstanceCreateMessage(serviceInstanceName, space.Name, serviceInstanceTags)
		})

		When("user has permissions to create ServiceInstances", func() {
			BeforeEach(func() {
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space.Name)
			})

			It("creates a new ServiceInstance CR", func() {
				createdServiceInstanceRecord, err := serviceInstanceRepo.CreateServiceInstance(testCtx, authInfo, serviceInstanceCreateMessage)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdServiceInstanceRecord.GUID).To(MatchRegexp("^[-0-9a-f]{36}$"), "record GUID was not a 36 character guid")
				Expect(createdServiceInstanceRecord.SpaceGUID).To(Equal(space.Name), "SpaceGUID in record did not match input")
				Expect(createdServiceInstanceRecord.Name).To(Equal(serviceInstanceName), "Name in record did not match input")
				Expect(createdServiceInstanceRecord.Type).To(Equal("user-provided"), "Type in record did not match input")
				Expect(createdServiceInstanceRecord.Tags).To(ConsistOf([]string{"foo", "bar"}), "Tags in record did not match input")

				recordCreatedTime, err := time.Parse(repositories.TimestampFormat, createdServiceInstanceRecord.CreatedAt)
				Expect(err).NotTo(HaveOccurred())
				Expect(recordCreatedTime).To(BeTemporally("~", time.Now(), 2*time.Second))

				recordUpdatedTime, err := time.Parse(repositories.TimestampFormat, createdServiceInstanceRecord.UpdatedAt)
				Expect(err).NotTo(HaveOccurred())
				Expect(recordUpdatedTime).To(BeTemporally("~", time.Now(), 2*time.Second))
			})

			When("no ServiceInstance credentials are given", func() {
				It("creates a secret and sets the secret ref on the ServiceInstance", func() {
					createdServiceInstanceRecord, err := serviceInstanceRepo.CreateServiceInstance(testCtx, authInfo, serviceInstanceCreateMessage)
					Expect(err).NotTo(HaveOccurred())
					Expect(createdServiceInstanceRecord).NotTo(BeNil())
					Expect(createdServiceInstanceRecord.SecretName).To(Equal(createdServiceInstanceRecord.GUID))

					secretLookupKey := types.NamespacedName{Name: createdServiceInstanceRecord.SecretName, Namespace: createdServiceInstanceRecord.SpaceGUID}
					createdSecret := new(corev1.Secret)
					Eventually(func() error {
						return k8sClient.Get(context.Background(), secretLookupKey, createdSecret)
					}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

					Expect(createdSecret.Data).To(MatchAllKeys(Keys{
						"type": BeEquivalentTo("user-provided"),
					}))
					Expect(createdSecret.Type).To(Equal(corev1.SecretType("servicebinding.io/user-provided")))
				})
			})

			When("ServiceInstance credentials are given", func() {
				BeforeEach(func() {
					serviceInstanceCreateMessage.Credentials = map[string]string{
						"type": "i get clobbered",
						"foo":  "bar",
						"baz":  "baz",
					}
				})

				It("creates a secret and sets the secret ref on the ServiceInstance", func() {
					createdServiceInstanceRecord, err := serviceInstanceRepo.CreateServiceInstance(testCtx, authInfo, serviceInstanceCreateMessage)
					Expect(err).NotTo(HaveOccurred())
					Expect(createdServiceInstanceRecord).NotTo(BeNil())
					Expect(createdServiceInstanceRecord.SecretName).To(Equal(createdServiceInstanceRecord.GUID))

					secretLookupKey := types.NamespacedName{Name: createdServiceInstanceRecord.SecretName, Namespace: createdServiceInstanceRecord.SpaceGUID}
					createdSecret := new(corev1.Secret)
					Eventually(func() error {
						return k8sClient.Get(context.Background(), secretLookupKey, createdSecret)
					}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

					Expect(createdSecret.Data).To(MatchAllKeys(Keys{
						"type": BeEquivalentTo("user-provided"),
						"foo":  BeEquivalentTo("bar"),
						"baz":  BeEquivalentTo("baz"),
					}))
				})
			})
		})

		When("user does not have permissions to create ServiceInstances", func() {
			It("returns a Forbidden error", func() {
				_, err := serviceInstanceRepo.CreateServiceInstance(testCtx, authInfo, serviceInstanceCreateMessage)
				Expect(err).To(BeAssignableToTypeOf(repositories.ForbiddenError{}))
			})
		})
	})

	Describe("ListServiceInstances", Serial, func() {
		var (
			space2, space3                                             *hnsv1alpha2.SubnamespaceAnchor
			cfServiceInstance1, cfServiceInstance2, cfServiceInstance3 *servicesv1alpha1.CFServiceInstance
			nonCFNamespace                                             string
			filters                                                    repositories.ListServiceInstanceMessage
		)

		BeforeEach(func() {
			space2 = createSpaceAnchorAndNamespace(testCtx, org.Name, prefixedGUID("space2"))
			space3 = createSpaceAnchorAndNamespace(testCtx, org.Name, prefixedGUID("space3"))

			nonCFNamespace = prefixedGUID("non-cf")
			Expect(k8sClient.Create(
				testCtx,
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nonCFNamespace}},
			)).To(Succeed())

			DeferCleanup(func() {
				_ = k8sClient.Delete(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: rootNamespace}})
				_ = k8sClient.Delete(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nonCFNamespace}})
				_ = k8sClient.Delete(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: org.Name}})
				_ = k8sClient.Delete(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: space.Name}})
				_ = k8sClient.Delete(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: space2.Name}})
				_ = k8sClient.Delete(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: space3.Name}})
			})

			cfServiceInstance1 = createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space.Name, "service-instance-1", prefixedGUID("secret"))
			cfServiceInstance2 = createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space2.Name, "service-instance-2", prefixedGUID("secret"))
			cfServiceInstance3 = createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space3.Name, "service-instance-3", prefixedGUID("secret"))
			createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), nonCFNamespace, "service-instance-4", prefixedGUID("secret"))
		})

		When("query parameters are not provided and", func() {
			BeforeEach(func() {
				filters = repositories.ListServiceInstanceMessage{}
			})

			When("no service instances exist in spaces where the user has permission", func() {
				It("returns an empty list of ServiceInstanceRecord", func() {
					serviceInstanceList, err := serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
					Expect(err).NotTo(HaveOccurred())
					Expect(serviceInstanceList).To(BeEmpty())
				})
			})

			When("multiple service instances exist in spaces where the user has permissions", func() {
				BeforeEach(func() {
					createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space.Name)
					createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space2.Name)
				})

				It("eventually returns ServiceInstance records from only the spaces where the user has permission", func() {
					Eventually(func() []repositories.ServiceInstanceRecord {
						serviceInstanceList, err := serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
						Expect(err).NotTo(HaveOccurred())
						return serviceInstanceList
					}, timeCheckThreshold*time.Second).Should(ConsistOf(
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance1.Name)}),
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance2.Name)}),
					))
				})
			})
		})

		When("query parameters are provided", func() {
			BeforeEach(func() {
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space.Name)
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space2.Name)
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space3.Name)
			})

			When("the name filter is set", func() {
				BeforeEach(func() {
					filters = repositories.ListServiceInstanceMessage{
						Names: []string{
							cfServiceInstance1.Spec.Name,
							cfServiceInstance3.Spec.Name,
						},
					}
				})
				It("eventually returns only records for the ServiceInstances with matching spec.name fields", func() {
					Eventually(func() []repositories.ServiceInstanceRecord {
						serviceInstanceList, err := serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
						Expect(err).NotTo(HaveOccurred())
						return serviceInstanceList
					}, timeCheckThreshold*time.Second).Should(ConsistOf(
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance1.Name)}),
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance3.Name)}),
					))
				})
			})

			When("the spaceGUID filter is set", func() {
				BeforeEach(func() {
					filters = repositories.ListServiceInstanceMessage{
						SpaceGuids: []string{
							cfServiceInstance2.Namespace,
							cfServiceInstance3.Namespace,
						},
					}
				})
				It("eventually returns only records for the ServiceInstances within the matching spaces", func() {
					Eventually(func() []repositories.ServiceInstanceRecord {
						serviceInstanceList, err := serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
						Expect(err).NotTo(HaveOccurred())
						return serviceInstanceList
					}, timeCheckThreshold*time.Second).Should(ConsistOf(
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance2.Name)}),
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance3.Name)}),
					))
				})
			})

			When("the OrderBy field is set to 'name'", func() {
				BeforeEach(func() {
					filters = repositories.ListServiceInstanceMessage{
						SpaceGuids: []string{
							cfServiceInstance1.Namespace,
							cfServiceInstance2.Namespace,
							cfServiceInstance3.Namespace,
						},
						OrderBy: "name",
					}
				})

				It("eventually returns the ServiceBindings in ascending name order", func() {
					var serviceInstanceList []repositories.ServiceInstanceRecord
					Eventually(func() []repositories.ServiceInstanceRecord {
						var err error
						serviceInstanceList, err = serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
						Expect(err).NotTo(HaveOccurred())
						return serviceInstanceList
					}, timeCheckThreshold*time.Second).Should(HaveLen(3))
					Expect(serviceInstanceList[0]).To(MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance1.Name)}))
					Expect(serviceInstanceList[1]).To(MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance2.Name)}))
					Expect(serviceInstanceList[2]).To(MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance3.Name)}))
				})

				When("the DescendingOrder field is true", func() {
					BeforeEach(func() {
						filters.DescendingOrder = true
					})

					It("eventually returns the ServiceBindings in descending name order", func() {
						var serviceInstanceList []repositories.ServiceInstanceRecord
						Eventually(func() []repositories.ServiceInstanceRecord {
							var err error
							serviceInstanceList, err = serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
							Expect(err).NotTo(HaveOccurred())
							return serviceInstanceList
						}, timeCheckThreshold*time.Second).Should(HaveLen(3))
						Expect(serviceInstanceList[0]).To(MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance3.Name)}))
						Expect(serviceInstanceList[1]).To(MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance2.Name)}))
						Expect(serviceInstanceList[2]).To(MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfServiceInstance1.Name)}))
					})
				})
			})

			When("the OrderBy field is set to 'created_at'", func() {
				BeforeEach(func() {
					filters = repositories.ListServiceInstanceMessage{
						SpaceGuids: []string{
							cfServiceInstance1.Namespace,
							cfServiceInstance2.Namespace,
							cfServiceInstance3.Namespace,
						},
						OrderBy: "created_at",
					}
					time.Sleep(time.Second)
					createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space3.Name, "another-service-instance", prefixedGUID("secret"))
				})

				It("eventually returns the ServiceBindings in ascending creation order", func() {
					var serviceInstanceList []repositories.ServiceInstanceRecord
					Eventually(func() []repositories.ServiceInstanceRecord {
						var err error
						serviceInstanceList, err = serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
						Expect(err).NotTo(HaveOccurred())
						return serviceInstanceList
					}, timeCheckThreshold*time.Second).Should(HaveLen(4))
					createTime1, err := time.Parse(time.RFC3339, serviceInstanceList[0].CreatedAt)
					Expect(err).NotTo(HaveOccurred())
					createTime2, err := time.Parse(time.RFC3339, serviceInstanceList[1].CreatedAt)
					Expect(err).NotTo(HaveOccurred())
					createTime3, err := time.Parse(time.RFC3339, serviceInstanceList[2].CreatedAt)
					Expect(err).NotTo(HaveOccurred())
					createTime4, err := time.Parse(time.RFC3339, serviceInstanceList[3].CreatedAt)
					Expect(err).NotTo(HaveOccurred())
					Expect(createTime1).To(BeTemporally("<=", createTime2))
					Expect(createTime2).To(BeTemporally("<=", createTime3))
					Expect(createTime3).To(BeTemporally("<=", createTime4))
				})

				When("the DescendingOrder field is true", func() {
					BeforeEach(func() {
						filters.DescendingOrder = true
					})

					It("eventually returns the ServiceBindings in descending creation order", func() {
						var serviceInstanceList []repositories.ServiceInstanceRecord
						Eventually(func() []repositories.ServiceInstanceRecord {
							var err error
							serviceInstanceList, err = serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
							Expect(err).NotTo(HaveOccurred())
							return serviceInstanceList
						}, timeCheckThreshold*time.Second).Should(HaveLen(4))
						createTime1, err := time.Parse(time.RFC3339, serviceInstanceList[0].CreatedAt)
						Expect(err).NotTo(HaveOccurred())
						createTime2, err := time.Parse(time.RFC3339, serviceInstanceList[1].CreatedAt)
						Expect(err).NotTo(HaveOccurred())
						createTime3, err := time.Parse(time.RFC3339, serviceInstanceList[2].CreatedAt)
						Expect(err).NotTo(HaveOccurred())
						createTime4, err := time.Parse(time.RFC3339, serviceInstanceList[3].CreatedAt)
						Expect(err).NotTo(HaveOccurred())
						Expect(createTime1).To(BeTemporally(">=", createTime2))
						Expect(createTime2).To(BeTemporally(">=", createTime3))
						Expect(createTime3).To(BeTemporally(">=", createTime4))
					})
				})
			})

			When("the OrderBy field is set to 'updated_at'", func() {
				BeforeEach(func() {
					filters = repositories.ListServiceInstanceMessage{
						SpaceGuids: []string{
							cfServiceInstance1.Namespace,
							cfServiceInstance2.Namespace,
							cfServiceInstance3.Namespace,
						},
						OrderBy: "updated_at",
					}
					time.Sleep(time.Second)
					createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space3.Name, "another-service-instance", prefixedGUID("secret"))
				})

				It("eventually returns the ServiceBindings in ascending update order", func() {
					var serviceInstanceList []repositories.ServiceInstanceRecord
					Eventually(func() []repositories.ServiceInstanceRecord {
						var err error
						serviceInstanceList, err = serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
						Expect(err).NotTo(HaveOccurred())
						return serviceInstanceList
					}, timeCheckThreshold*time.Second).Should(HaveLen(4))
					updateTime1, err := time.Parse(time.RFC3339, serviceInstanceList[0].UpdatedAt)
					Expect(err).NotTo(HaveOccurred())
					updateTime2, err := time.Parse(time.RFC3339, serviceInstanceList[1].UpdatedAt)
					Expect(err).NotTo(HaveOccurred())
					updateTime3, err := time.Parse(time.RFC3339, serviceInstanceList[2].UpdatedAt)
					Expect(err).NotTo(HaveOccurred())
					updateTime4, err := time.Parse(time.RFC3339, serviceInstanceList[3].UpdatedAt)
					Expect(err).NotTo(HaveOccurred())
					Expect(updateTime1).To(BeTemporally("<=", updateTime2))
					Expect(updateTime2).To(BeTemporally("<=", updateTime3))
					Expect(updateTime3).To(BeTemporally("<=", updateTime4))
				})

				When("the DescendingOrder field is true", func() {
					BeforeEach(func() {
						filters.DescendingOrder = true
					})

					It("eventually returns the ServiceBindings in descending update order", func() {
						var serviceInstanceList []repositories.ServiceInstanceRecord
						Eventually(func() []repositories.ServiceInstanceRecord {
							var err error
							serviceInstanceList, err = serviceInstanceRepo.ListServiceInstances(testCtx, authInfo, filters)
							Expect(err).NotTo(HaveOccurred())
							return serviceInstanceList
						}, timeCheckThreshold*time.Second).Should(HaveLen(4))
						updateTime1, err := time.Parse(time.RFC3339, serviceInstanceList[0].UpdatedAt)
						Expect(err).NotTo(HaveOccurred())
						updateTime2, err := time.Parse(time.RFC3339, serviceInstanceList[1].UpdatedAt)
						Expect(err).NotTo(HaveOccurred())
						updateTime3, err := time.Parse(time.RFC3339, serviceInstanceList[2].UpdatedAt)
						Expect(err).NotTo(HaveOccurred())
						updateTime4, err := time.Parse(time.RFC3339, serviceInstanceList[3].UpdatedAt)
						Expect(err).NotTo(HaveOccurred())
						Expect(updateTime1).To(BeTemporally(">=", updateTime2))
						Expect(updateTime2).To(BeTemporally(">=", updateTime3))
						Expect(updateTime3).To(BeTemporally(">=", updateTime4))
					})
				})
			})
		})
	})

	Describe("GetServiceInstance", func() {
		var (
			space2          *hnsv1alpha2.SubnamespaceAnchor
			serviceInstance *servicesv1alpha1.CFServiceInstance
			record          repositories.ServiceInstanceRecord
			getErr          error
			getGUID         string
		)

		BeforeEach(func() {
			space2 = createSpaceAnchorAndNamespace(testCtx, org.Name, prefixedGUID("space2"))

			serviceInstance = createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space.Name, "the-service-instance", prefixedGUID("secret"))
			createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space2.Name, "some-other-service-instance", prefixedGUID("secret"))
			getGUID = serviceInstance.Name
		})

		JustBeforeEach(func() {
			record, getErr = serviceInstanceRepo.GetServiceInstance(testCtx, authInfo, getGUID)
		})

		When("there are no permissions on service instances", func() {
			It("returns a forbidden error", func() {
				Expect(errors.As(getErr, &repositories.ForbiddenError{})).To(BeTrue())
			})
		})

		When("the user has permissions to get the service instance", func() {
			BeforeEach(func() {
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space.Name)
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space2.Name)
			})

			It("returns the correct service instance", func() {
				Expect(getErr).NotTo(HaveOccurred())

				Expect(record.Name).To(Equal(serviceInstance.Spec.Name))
				Expect(record.GUID).To(Equal(serviceInstance.Name))
				Expect(record.SpaceGUID).To(Equal(serviceInstance.Namespace))
				Expect(record.SecretName).To(Equal(serviceInstance.Spec.SecretName))
				Expect(record.Tags).To(Equal(serviceInstance.Spec.Tags))
				Expect(record.Type).To(Equal(string(serviceInstance.Spec.Type)))
			})
		})

		When("the service instance does not exist", func() {
			BeforeEach(func() {
				getGUID = "does-not-exist"
			})

			It("returns a not found error", func() {
				notFoundErr := repositories.NotFoundError{}
				Expect(errors.As(getErr, &notFoundErr)).To(BeTrue())
				Expect(notFoundErr.ResourceType()).To(Equal(repositories.ServiceInstanceResourceType))
			})
		})

		When("more than one service instance with the same guid exists", func() {
			BeforeEach(func() {
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space.Name)
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space2.Name)
				createServiceInstanceCR(testCtx, k8sClient, getGUID, space2.Name, "the-service-instance", prefixedGUID("secret"))
			})

			It("returns a error", func() {
				Expect(getErr).To(MatchError(ContainSubstring("get-service instance duplicate records exist")))
			})
		})

		When("the context has expired", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				testCtx, cancel = context.WithCancel(testCtx)
				cancel()
			})

			It("returns a error", func() {
				Expect(getErr).To(HaveOccurred())
			})
		})
	})

	Describe("DeleteServiceInstance", func() {
		var (
			serviceInstance *servicesv1alpha1.CFServiceInstance
			deleteMessage   repositories.DeleteServiceInstanceMessage
			deleteErr       error
		)

		BeforeEach(func() {
			serviceInstance = createServiceInstanceCR(testCtx, k8sClient, prefixedGUID("service-instance"), space.Name, "the-service-instance", prefixedGUID("secret"))

			deleteMessage = repositories.DeleteServiceInstanceMessage{
				GUID:      serviceInstance.Name,
				SpaceGUID: space.Name,
			}
		})

		JustBeforeEach(func() {
			deleteErr = serviceInstanceRepo.DeleteServiceInstance(testCtx, authInfo, deleteMessage)
		})

		When("the user has permissions to delete service instances", func() {
			BeforeEach(func() {
				createRoleBinding(testCtx, userName, spaceDeveloperRole.Name, space.Name)
			})

			It("deletes the service instance", func() {
				Expect(deleteErr).NotTo(HaveOccurred())

				namespacedName := types.NamespacedName{
					Name:      serviceInstance.Name,
					Namespace: space.Name,
				}

				err := k8sClient.Get(context.Background(), namespacedName, &servicesv1alpha1.CFServiceInstance{})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue(), fmt.Sprintf("error: %+v", err))
			})

			When("the service instances does not exist", func() {
				BeforeEach(func() {
					deleteMessage.GUID = "does-not-exist"
				})

				It("returns a not found error", func() {
					Expect(errors.As(deleteErr, &repositories.NotFoundError{})).To(BeTrue())
				})
			})
		})

		When("there are no permissions on service instances", func() {
			It("returns a forbidden error", func() {
				Expect(errors.As(deleteErr, &repositories.ForbiddenError{})).To(BeTrue())
			})
		})
	})
})

func initializeServiceInstanceCreateMessage(serviceInstanceName string, spaceGUID string, tags []string) repositories.CreateServiceInstanceMessage {
	return repositories.CreateServiceInstanceMessage{
		Name:      serviceInstanceName,
		SpaceGUID: spaceGUID,
		Type:      "user-provided",
		Tags:      tags,
	}
}