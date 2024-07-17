/*
Copyright 2023 The KServe Authors.

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

// +kubebuilder:rbac:groups=serving.kserve.io,resources=clustercachedmodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.kserve.io,resources=clustercachedmodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
package cachedmodel

import (
	"context"
	"fmt"
	"log"

	"github.com/go-logr/logr"
	"github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	v1alpha1api "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CachedModelReconciler struct {
	client.Client
	Clientset *kubernetes.Clientset
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

func launchK8sJob(clientset *kubernetes.Clientset, jobName *string, image *string, cmd *string, namespace *string, cachedModel *v1alpha1api.ClusterCachedModel, scheme *runtime.Scheme) {
	jobs := clientset.BatchV1().Jobs(*namespace)
	var backOffLimit int32 = 0

	existingJob, err := jobs.Get(context.TODO(), *jobName, metav1.GetOptions{})
	if err != nil {
		log.Fatalln("Get job err")
	}
	if existingJob != nil {
		log.Printf("Job exists %s %s", existingJob.Namespace, existingJob.Name)
		return
	}

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *jobName,
			Namespace: *namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    *jobName,
							Image:   *image,
							Command: []string{"perl", "-Mbignum=bpi", "-wle", "print bpi(2000)"},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
			BackoffLimit: &backOffLimit,
		},
	}

	if err := controllerutil.SetControllerReference(cachedModel, jobSpec, scheme); err != nil {
		log.Fatalln("set controller reference", err)
	}

	_, err = jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
	if err != nil {
		log.Fatalln("Failed to create K8s job.", err)
	}

	//print job details
	log.Printf("Created K8s job successfully %s %s", *jobName, *namespace)
}

func getContainerSpecForStorageUri(storageUri string, client client.Client) (*v1.Container, error) {
	storageContainers := &v1alpha1.ClusterStorageContainerList{}
	if err := client.List(context.TODO(), storageContainers); err != nil {
		return nil, err
	}

	for _, sc := range storageContainers.Items {
		if sc.IsDisabled() {
			continue
		}
		supported, err := sc.Spec.IsStorageUriSupported(storageUri)
		if err != nil {
			return nil, fmt.Errorf("error checking storage container %s: %w", sc.Name, err)
		}
		if supported {
			return &sc.Spec.Container, nil
		}
	}

	return nil, nil
}

func (c *CachedModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.Log.Info("Hello from model cache controller")
	// jobName := "hello"
	containerImage := "perl:5.34.0"
	entryCommand := "perl -Mbignum=bpi -wle print bpi(2000)"
	namespace := "kserve"

	// Fetch the InferenceService instance
	cachedModel := &v1alpha1api.ClusterCachedModel{}
	if err := c.Get(ctx, req.NamespacedName, cachedModel); err != nil {
		return reconcile.Result{}, err
	}
	container, err := getContainerSpecForStorageUri(cachedModel.Spec.StorageUri, c.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	log.Println(container.Image)
	log.Println(cachedModel.Spec.StorageUri)
	log.Println(req.NamespacedName.Name)
	log.Println(req.NamespacedName.Namespace)

	launchK8sJob(c.Clientset, &req.NamespacedName.Name, &containerImage, &entryCommand, &namespace, cachedModel, c.Scheme)

	return reconcile.Result{}, nil
}

func (c *CachedModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1api.ClusterCachedModel{}).
		Complete(c)
}
