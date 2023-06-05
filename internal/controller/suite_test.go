/*
Copyright 2022.

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
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/movetokube/postgres-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var managerClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var k8sManager manager.Manager
var realClient bool

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

func clearPgs(namespace string) (err error) {
	l := v1alpha1.PostgresList{}
	err = k8sClient.List(ctx, &l, client.InNamespace(namespace))
	Expect(err).ToNot(HaveOccurred())
	for _, el := range l.Items {
		org := el.DeepCopy()
		el.SetFinalizers(nil)
		err = k8sClient.Patch(ctx, &el, client.MergeFrom(org))
		if err != nil {
			return
		}
	}
	return k8sClient.DeleteAllOf(ctx, &v1alpha1.Postgres{}, client.InNamespace(namespace))
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	_, realClient = os.LookupEnv("ENVTEST_K8S_VERSION")
	var err error

	// tests still have some issue with the real etcd
	if realClient {
		testEnv = &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
			ErrorIfCRDPathMissing: true,
		}
		// cfg is defined in this file globally.
		cfg, err = testEnv.Start()
		testEnv.ControlPlane.Etcd.Out = os.Stdout
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
	}
	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	if realClient {
		k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			err = k8sManager.Start(ctx)
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}()
		managerClient = k8sManager.GetClient()
		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(k8sClient).NotTo(BeNil())
	} else {
		k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithStatusSubresource(&v1alpha1.Postgres{}, &v1alpha1.PostgresUser{}).Build()
		managerClient = k8sClient
	}
	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "operator",
	}})).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})
