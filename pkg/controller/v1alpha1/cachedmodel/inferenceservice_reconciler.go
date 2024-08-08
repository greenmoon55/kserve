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
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
package cachedmodel

import (
	"context"
	"log"

	"github.com/go-logr/logr"
	"github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CachedModelInferenceServiceReconciler struct {
	client.Client
	Clientset *kubernetes.Clientset
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

// func getContainerSpecForStorageUri(storageUri string, client client.Client) (*v1.Container, error) {
// 	storageContainers := &v1alpha1.ClusterStorageContainerList{}
// 	if err := client.List(context.TODO(), storageContainers); err != nil {
// 		return nil, err
// 	}

// 	for _, sc := range storageContainers.Items {
// 		if sc.IsDisabled() {
// 			continue
// 		}
// 		supported, err := sc.Spec.IsStorageUriSupported(storageUri)
// 		if err != nil {
// 			return nil, fmt.Errorf("error checking storage container %s: %w", sc.Name, err)
// 		}
// 		if supported {
// 			return &sc.Spec.Container, nil
// 		}
// 	}

// 	return nil, nil
// }

func getModelCacheSpecForStorageUri(storageUri string, client client.Client) (*v1alpha1.ClusterCachedModel, error) {
	models := &v1alpha1.ClusterCachedModelList{}
	if err := client.List(context.TODO(), models); err != nil {
		return nil, err
	}

	for _, model := range models.Items {
		if model.Spec.StorageUri == storageUri {
			return &model, nil
		}
	}

	return nil, nil
}

func (c *CachedModelInferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Println("Hello from CachedModelInferenceServiceReconciler")
	// // jobName := "hello"
	// // containerImage := "perl:5.34.0"
	// entryCommand := "perl -Mbignum=bpi -wle print bpi(2000)"
	// namespace := "kserve"
	log.Println("reconcile isvc: ", req.NamespacedName)

	isvc := &v1beta1.InferenceService{}

	if err := c.Get(ctx, req.NamespacedName, isvc); err != nil {
		return reconcile.Result{}, err
	}

	var deleted bool

	if !isvc.ObjectMeta.DeletionTimestamp.IsZero() {
		deleted = true
		log.Println("the isvc is being deleted")
	} else {
		deleted = false
		log.Println("the isvc is not being deleted")
	}

	storageUri := isvc.Spec.Predictor.GetImplementation().GetStorageUri()

	cachedModel, err := getModelCacheSpecForStorageUri(*storageUri, c.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cachedModel == nil {
		return reconcile.Result{}, nil
	}
	if !deleted {
		// cachedModel.Status.InferenceServices[v1alpha1.NamespacedName{Namespace: isvc.Namespace, Name: isvc.Name}] = struct{}{}

		// pvSpec := cachedModel.Spec.PersistentVolume
		// pvSpec.Name = cachedModel.Name + "-" + isvc.Namespace
		// persistentVolumes := c.Clientset.CoreV1().PersistentVolumes()
		// if _, err := persistentVolumes.Get(context.TODO(), pvSpec.Name, metav1.GetOptions{}); err != nil {
		// 	if !apierr.IsNotFound(err) {
		// 		log.Fatalln("Get pv err", err)
		// 	}
		// 	log.Println("Creating PV")
		// 	if err := controllerutil.SetControllerReference(cachedModel, &pvSpec, c.Scheme); err != nil {
		// 		log.Fatalln("set controller reference", err)
		// 	}
		// 	if _, err := persistentVolumes.Create(context.TODO(), &pvSpec, metav1.CreateOptions{}); err != nil {
		// 		log.Fatalln("Create pv err", err)
		// 	}
		// 	log.Println("PV Created")
		// }
		// log.Println("PV Exists")

		// pvcSpec := cachedModel.Spec.PersistentVolumeClaim
		// pvcSpec.Name = cachedModel.Name
		// pvcSpec.Spec.VolumeName = pvSpec.Name
		// persistentVolumeClaims := c.Clientset.CoreV1().PersistentVolumeClaims(isvc.Namespace)
		// log.Println("Checking PVC")
		// if _, err := persistentVolumeClaims.Get(context.TODO(), pvcSpec.Name, metav1.GetOptions{}); err != nil {
		// 	if !apierr.IsNotFound(err) {
		// 		log.Fatalln("Get pvc err", err)
		// 	}
		// 	log.Println("Creating PVC")
		// 	if err := controllerutil.SetControllerReference(cachedModel, &pvcSpec, c.Scheme); err != nil {
		// 		log.Fatalln("set controller reference", err)
		// 	}
		// 	if _, err := persistentVolumeClaims.Create(context.TODO(), &pvcSpec, metav1.CreateOptions{}); err != nil {
		// 		log.Fatalln("Create PVC err", err)
		// 	}
		// 	log.Println("PVC Created")
		// }
	} else {
		// delete(cachedModel.Status.InferenceServices, v1alpha1.NamespacedName{Namespace: isvc.Namespace, Name: isvc.Name})
	}
	// client.Update(context.TODO(), cachedModel.Status)

	return reconcile.Result{}, nil
}

func (c *CachedModelInferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.InferenceService{}).
		Complete(c)
}
