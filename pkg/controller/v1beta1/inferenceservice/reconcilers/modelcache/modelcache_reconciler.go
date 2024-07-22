/*
Copyright 2021 The KServe Authors.

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

package modelcache

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	v1alpha1api "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	v1beta1api "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logf.Log.WithName("ModelCacheReconciler")

type ModelCacheReconciler struct {
	client     client.Client
	clientset  kubernetes.Interface
	scheme     *runtime.Scheme
	storageUri string
}

func NewModelCacheReconciler(client client.Client, clientset kubernetes.Interface, scheme *runtime.Scheme, storageUri string) *ModelCacheReconciler {
	return &ModelCacheReconciler{
		client:     client,
		clientset:  clientset,
		scheme:     scheme,
		storageUri: storageUri,
	}
}

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

func (c *ModelCacheReconciler) Reconcile(isvc *v1beta1api.InferenceService) error {
	log.Info("ModelCacheReconciler", isvc.Namespace, isvc.Name)

	cachedModel := &v1alpha1api.ClusterCachedModel{}

	modelcache, err := getModelCacheSpecForStorageUri(c.storageUri, c.client)
	if err != nil {
		return err
	}
	namespacedName := types.NamespacedName{Name: modelcache.Name}
	if err := c.client.Get(context.TODO(), namespacedName, cachedModel); err != nil {
		return err
	}

	pvSpec := cachedModel.Spec.PersistentVolume
	pvSpec.Name = cachedModel.Name + "-" + isvc.Namespace
	persistentVolumes := c.clientset.CoreV1().PersistentVolumes()
	if _, err := persistentVolumes.Get(context.TODO(), pvSpec.Name, metav1.GetOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Info("Get pv err", err)
		}
		log.Info("Creating PV")
		if _, err := persistentVolumes.Create(context.TODO(), &pvSpec, metav1.CreateOptions{}); err != nil {
			log.Info("Create pv err", err)
		}
		log.Info("PV Created")
		if err := controllerutil.SetControllerReference(cachedModel, &pvSpec, c.scheme); err != nil {
			log.Info("set controller reference", err)
		}
	}
	log.Info("PV Exists")

	pvcSpec := cachedModel.Spec.PersistentVolumeClaim
	pvcSpec.Name = cachedModel.Name
	pvcSpec.Spec.VolumeName = pvSpec.Name
	persistentVolumeClaims := c.clientset.CoreV1().PersistentVolumeClaims(isvc.Namespace)
	log.Info("Checking PVC")
	if _, err := persistentVolumeClaims.Get(context.TODO(), pvcSpec.Name, metav1.GetOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Info("Get pvc err", err)
		}
		log.Info("Creating PVC")
		if _, err := persistentVolumeClaims.Create(context.TODO(), &pvcSpec, metav1.CreateOptions{}); err != nil {
			log.Info("Create PVC err", err)
		}
		log.Info("PVC Created")
		if err := controllerutil.SetControllerReference(cachedModel, &pvcSpec, c.scheme); err != nil {
			log.Info("set controller reference", err)
		}
	}

	// if v1beta1utils.IsMMSPredictor(&isvc.Spec.Predictor) {
	// 	// Create an empty modelConfig for every InferenceService shard
	// 	// An InferenceService without storageUri is an empty model server with for multi-model serving so a modelConfig configmap should be created
	// 	// An InferenceService with storageUri is considered as multi-model InferenceService with only one model, a modelConfig configmap should be created as well
	// 	shardStrategy := memory.MemoryStrategy{}
	// 	for _, id := range shardStrategy.GetShard(isvc) {
	// 		modelConfigName := constants.ModelConfigName(isvc.Name, id)
	// 		_, err := c.clientset.CoreV1().ConfigMaps(isvc.Namespace).Get(context.TODO(), modelConfigName, metav1.GetOptions{})
	// 		if err != nil {
	// 			if errors.IsNotFound(err) {
	// 				// If the modelConfig does not exist for an InferenceService without storageUri, create an empty modelConfig
	// 				log.Info("Creating modelConfig", "configmap", modelConfigName, "inferenceservice", isvc.Name, "namespace", isvc.Namespace)
	// 				newModelConfig, err := modelconfig.CreateEmptyModelConfig(isvc, id)
	// 				if err != nil {
	// 					return err
	// 				}
	// 				if err := controllerutil.SetControllerReference(isvc, newModelConfig, c.scheme); err != nil {
	// 					return err
	// 				}
	// 				err = c.client.Create(context.TODO(), newModelConfig)
	// 				if err != nil {
	// 					return err
	// 				}
	// 			} else {
	// 				return err
	// 			}
	// 		}
	// 	}
	// }
	return nil
}
