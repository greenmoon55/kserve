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

// +kubebuilder:rbac:groups=serving.kserve.io,resources=inferenceservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.kserve.io,resources=modelcachenodegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.kserve.io,resources=clustercachedmodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.kserve.io,resources=clustercachedmodels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
package cachedmodel

import (
	"context"
	"fmt"
	"log"

	"github.com/go-logr/logr"
	"github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	v1alpha1api "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	"github.com/kserve/kserve/pkg/constants"
	"github.com/kserve/kserve/pkg/utils"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CachedModelReconciler struct {
	client.Client
	Clientset *kubernetes.Clientset
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

var (
	ownerKey = ".metadata.controller"
)

func launchK8sJob(clientset *kubernetes.Clientset, jobName *string, image *string, delete bool, namespace *string, cachedModel *v1alpha1api.ClusterCachedModel, scheme *runtime.Scheme, storageUri string, claimName string, node string) *batchv1.Job {
	jobs := clientset.BatchV1().Jobs(*namespace)
	var backOffLimit int32 = 0
	log.Printf("Job %s %s %s", *namespace, *jobName, node)

	job, err := jobs.Get(context.TODO(), *jobName, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			log.Fatalln("Get job err", err)
		}
		// log.Fatalln("IsNotfound", err)
	} else {
		log.Println("Job exists, returning")
		log.Printf("Success %d", job.Status.Succeeded)
		return job
	}
	// if existingJob != nil {
	// 	log.Printf("Job exists %s %s", existingJob.Namespace, existingJob.Name)
	// 	return
	// }

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *jobName,
			Namespace: *namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeName: node,
					Containers: []v1.Container{
						{
							Name:  *jobName,
							Image: *image,
							Args:  []string{storageUri, "/mnt/models"},
							// Command: []string{"perl", "-Mbignum=bpi", "-wle", "print bpi(2000)"},
							VolumeMounts: []v1.VolumeMount{{
								MountPath: "/mnt/models",
								Name:      "kserve-pvc-source",
								ReadOnly:  false,
								SubPath:   "models/" + cachedModel.Name,
							},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
					Volumes: []v1.Volume{
						{
							Name: "kserve-pvc-source",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: claimName,
								},
							},
						},
					},
				},
			},
			BackoffLimit: &backOffLimit,
		},
	}

	if delete {
		jobSpec.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "rm -rf /mnt/models/*"}
		jobSpec.Spec.Template.Spec.Containers[0].Args = nil
		// jobSpec.Spec.Template.Spec.Containers[0].Command = []string{"sleep", "300000"}
		// jobSpec.Spec.Template.Spec.Containers[0].Command = strings.Split("find . ! -type d -exec rm '{}' \\;", " ")
		// jobSpec.Spec.Template.Spec.Containers[0].SecurityContext = &v1.SecurityContext{}
		// jobSpec.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = new(int64)
	}

	if err := controllerutil.SetControllerReference(cachedModel, jobSpec, scheme); err != nil {
		log.Fatalln("set controller reference", err)
	}
	job, err = jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
	if err != nil {
		log.Fatalln("Failed to create K8s job.", err)
	}

	log.Printf("Created K8s job successfully %s %s", *jobName, *namespace)
	return job
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

// func (c *CachedModelReconciler) ReconcileForNamespace(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
// }

func (c *CachedModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.Log.Info("Hello from model cache controller")
	// jobName := "hello"
	// containerImage := "perl:5.34.0"
	namespace := "kserve"
	log.Println("reconcile: ", req.NamespacedName)

	cachedModel := &v1alpha1api.ClusterCachedModel{}
	if err := c.Get(ctx, req.NamespacedName, cachedModel); err != nil {
		return reconcile.Result{}, err
	}

	nodeGroup := &v1alpha1api.ModelCacheNodeGroup{}
	nodeGroupNamespacedName := types.NamespacedName{Name: cachedModel.Spec.NodeGroup}
	if err := c.Get(ctx, nodeGroupNamespacedName, nodeGroup); err != nil {
		return reconcile.Result{}, err
	}

	finalizerName := "cachedModel.finalizers"

	if cachedModel.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !utils.Includes(cachedModel.ObjectMeta.Finalizers, finalizerName) {
			cachedModel.ObjectMeta.Finalizers = append(cachedModel.ObjectMeta.Finalizers, finalizerName)
			if err := c.Update(context.Background(), cachedModel); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if utils.Includes(cachedModel.ObjectMeta.Finalizers, finalizerName) {
			// our finalizer is present, so lets handle any external dependency

			// Check job status and decide whether to delete finalizer
			log.Println("being deleted, finalizer exists")
			// if err := c.deleteExternalResources(isvc); err != nil {
			// 	// if fail to delete the external dependency here, return with error
			// 	// so that it can be retried
			// 	return ctrl.Result{}, err
			// }

			// remove our finalizer from the list and update it.
			// isvc.ObjectMeta.Finalizers = utils.RemoveString(isvc.ObjectMeta.Finalizers, finalizerName)
			// if err := r.Update(context.Background(), isvc); err != nil {
			// 	return ctrl.Result{}, err
			// }
			image := "kserve/storage-initializer:latest"
			nodes := &v1.NodeList{}
			selector, _ := labels.ValidatedSelectorFromSet(nodeGroup.Spec.NodeSelector)
			if err := c.Client.List(context.TODO(), nodes, &client.ListOptions{LabelSelector: selector}); err != nil {
				log.Fatalln(err)
			}

			log.Println("print nodes")
			pendingNodes := 0
			for _, node := range nodes.Items {
				log.Println(node.Name)
				jobName := req.NamespacedName.Name + "-" + node.Name + "-delete"

				job := launchK8sJob(c.Clientset, &jobName, &image, true, &namespace, cachedModel, c.Scheme, cachedModel.Spec.StorageUri, cachedModel.Name, node.Name)
				if job.Status.Succeeded != 1 {
					pendingNodes += 1
				}
			}
			if pendingNodes == 0 {
				// remove our finalizer from the list and update it.
				cachedModel.ObjectMeta.Finalizers = utils.RemoveString(cachedModel.ObjectMeta.Finalizers, finalizerName)
				if err := c.Update(context.Background(), cachedModel); err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	pvSpec := nodeGroup.Spec.PersistentVolume
	pvSpec.Name = cachedModel.Name + "-download"
	persistentVolumes := c.Clientset.CoreV1().PersistentVolumes()
	if _, err := persistentVolumes.Get(context.TODO(), pvSpec.Name, metav1.GetOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Fatalln("Get pv err", err)
		}
		log.Println("Creating PV")
		if _, err := persistentVolumes.Create(context.TODO(), &pvSpec, metav1.CreateOptions{}); err != nil {
			log.Fatalln("Create pv err", err)
		}
		log.Println("PV Created")
		if err := controllerutil.SetControllerReference(cachedModel, &pvSpec, c.Scheme); err != nil {
			log.Fatalln("set controller reference", err)
		}
	}
	log.Println("PV Exists")

	pvcSpec := nodeGroup.Spec.PersistentVolumeClaim
	pvcSpec.Name = cachedModel.Name
	pvcSpec.Spec.VolumeName = pvSpec.Name
	persistentVolumeClaims := c.Clientset.CoreV1().PersistentVolumeClaims(namespace)
	log.Println("Checking PVC")
	if _, err := persistentVolumeClaims.Get(context.TODO(), pvcSpec.Name, metav1.GetOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Fatalln("Get pvc err", err)
		}
		log.Println("Creating PVC")
		if _, err := persistentVolumeClaims.Create(context.TODO(), &pvcSpec, metav1.CreateOptions{}); err != nil {
			log.Fatalln("Create PVC err", err)
		}
		log.Println("PVC Created")
		if err := controllerutil.SetControllerReference(cachedModel, &pvcSpec, c.Scheme); err != nil {
			log.Fatalln("set controller reference", err)
		}
	}

	container, err := getContainerSpecForStorageUri(cachedModel.Spec.StorageUri, c.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	log.Println(container.Image)
	log.Println(cachedModel.Spec.StorageUri)
	log.Println(req.NamespacedName.Name)
	log.Println(req.NamespacedName.Namespace)

	nodes := &v1.NodeList{}
	selector, _ := labels.ValidatedSelectorFromSet(nodeGroup.Spec.NodeSelector)
	if err = c.Client.List(context.TODO(), nodes, &client.ListOptions{LabelSelector: selector}); err != nil {
		log.Fatalln(err)
	}

	log.Println("print nodes")
	for _, node := range nodes.Items {
		log.Println(node.Name)
		jobName := req.NamespacedName.Name + "-" + node.Name
		job := launchK8sJob(c.Clientset, &jobName, &container.Image, false, &namespace, cachedModel, c.Scheme, cachedModel.Spec.StorageUri, pvcSpec.Name, node.Name)
		if job.Status.Succeeded == 1 {
			log.Println("Update status to ready")
			cachedModel.Status.OverallStatus = v1alpha1.ModelReady
		} else {
			log.Println("Update status to not ready")
			cachedModel.Status.OverallStatus = v1alpha1.ModelDownloading
		}
	}

	// job := launchK8sJob(c.Clientset, &req.NamespacedName.Name, &container.Image, &entryCommand, &namespace, cachedModel, c.Scheme, cachedModel.Spec.StorageUri, pvcSpec.Name, "nodeName")
	// if job.Status.Succeeded == 1 {
	// 	log.Println("Update status to ready")
	// 	cachedModel.Status.OverallStatus = v1alpha1.ModelReady
	// } else {
	// 	log.Println("Update status to not ready")
	// 	cachedModel.Status.OverallStatus = v1alpha1.ModelDownloading
	// }

	isvcs := &v1beta1.InferenceServiceList{}
	// if err = c.Client.List(context.TODO(), isvcs, client.MatchingLabels{constants.ModelCacheEnabled: cachedModel.Name}); err != nil {
	// 	log.Fatalln(err)
	// }
	if err = c.Client.List(context.TODO(), isvcs, client.MatchingFields{ownerKey: cachedModel.Name}); err != nil {
		log.Fatalln(err)
	}

	log.Println("Got isvcs", len(isvcs.Items))
	isvcNames := []v1alpha1.NamespacedName{}
	namespaces := make(map[string]struct{})
	for _, isvc := range isvcs.Items {
		log.Println(isvc.Name, isvc.Namespace)
		isvcNames = append(isvcNames, v1alpha1.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace})
		namespaces[isvc.Namespace] = struct{}{}
	}
	cachedModel.Status.InferenceServices = isvcNames
	if err := c.Status().Update(context.TODO(), cachedModel); err != nil {
		log.Fatalln("cannot update status", err)
	}

	pvcs := v1.PersistentVolumeClaimList{}
	log.Println("pvcs")
	if err := c.List(ctx, &pvcs, client.MatchingFields{ownerKey: req.Name}); err != nil {
		log.Fatalln(err, "unable to list pvcs")
		return ctrl.Result{}, err
	}
	for _, pvc := range pvcs.Items {
		log.Println("listing pvc")
		log.Println(pvc.Name, pvc.Namespace)
		log.Println("listing pvc done")
		if _, ok := namespaces[pvc.Namespace]; !ok {
			if pvc.Namespace == "kserve" {
				continue
			}
			log.Println("deleting pvc", pvc.Name, " in ", pvc.Namespace)
			persistentVolumeClaims := c.Clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace)
			if err := persistentVolumeClaims.Delete(context.TODO(), pvc.Name, metav1.DeleteOptions{}); err != nil {
				log.Println("deleting pvc in ", pvc.Namespace, "err", err)
			}
			log.Println("deleting pv", pvc.Namespace)
			persistentVolumes := c.Clientset.CoreV1().PersistentVolumes()
			if err := persistentVolumes.Delete(context.TODO(), pvc.Name+"-"+pvc.Namespace, metav1.DeleteOptions{}); err != nil {
				log.Println("deleting pv err", err)
			}
		}
	}

	for namespace := range namespaces {
		pvSpec := nodeGroup.Spec.PersistentVolume
		pvSpec.Name = cachedModel.Name + "-" + namespace
		persistentVolumes := c.Clientset.CoreV1().PersistentVolumes()
		if _, err := persistentVolumes.Get(context.TODO(), pvSpec.Name, metav1.GetOptions{}); err != nil {
			if !apierr.IsNotFound(err) {
				log.Println("Get pv err", err)
			}
			if err := controllerutil.SetControllerReference(cachedModel, &pvSpec, c.Scheme); err != nil {
				log.Println("set controller reference", err)
			}
			log.Println("Creating PV")
			if _, err := persistentVolumes.Create(context.TODO(), &pvSpec, metav1.CreateOptions{}); err != nil {
				log.Println("Create pv err", err)
			}
			log.Println("PV Created")
		}
		log.Println("PV Exists")

		pvcSpec := nodeGroup.Spec.PersistentVolumeClaim
		pvcSpec.Name = cachedModel.Name
		pvcSpec.Spec.VolumeName = pvSpec.Name
		persistentVolumeClaims := c.Clientset.CoreV1().PersistentVolumeClaims(namespace)
		log.Println("Checking PVC")
		if _, err := persistentVolumeClaims.Get(context.TODO(), pvcSpec.Name, metav1.GetOptions{}); err != nil {
			if !apierr.IsNotFound(err) {
				log.Println("Get pvc err", err)
			}
			if err := controllerutil.SetControllerReference(cachedModel, &pvcSpec, c.Scheme); err != nil {
				log.Println("set controller reference", err)
			}
			log.Println("Creating PVC")
			if _, err := persistentVolumeClaims.Create(context.TODO(), &pvcSpec, metav1.CreateOptions{}); err != nil {
				log.Println("Create PVC err", err)
			}
			log.Println("PVC Created")
		}
	}

	return reconcile.Result{}, nil
}

func (c *CachedModelReconciler) myFunc(ctx context.Context, obj client.Object) []reconcile.Request {
	log.Println("In myFunc")
	log.Println(obj.GetName())
	log.Println(obj.GetNamespace())
	isvc := &v1beta1.InferenceService{}
	if err := c.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, isvc); err != nil {
		log.Println("err", err) // can be deleted
		models := &v1alpha1.ClusterCachedModelList{}
		if err := c.Client.List(context.TODO(), models); err != nil {
			return []reconcile.Request{}
		}

		for _, model := range models.Items {
			for _, isvc := range model.Status.InferenceServices {
				if isvc.Namespace == obj.GetNamespace() && isvc.Name == obj.GetName() {
					log.Println("Reconcile model cache (deleted isvc): ", isvc.Name)
					return []reconcile.Request{{
						NamespacedName: types.NamespacedName{
							Name: model.Name,
						}}}
				}
			}
		}
		// return []reconcile.Request{}
	}
	name := ""
	var ok bool
	if isvc.Labels != nil {
		if name, ok = isvc.Labels[constants.ModelCacheEnabled]; ok {
			log.Println("Model cache name on isvc: ", name)
		}
	}
	if name == "" {
		return []reconcile.Request{}
	}
	cachedModel := &v1alpha1api.ClusterCachedModel{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, cachedModel); err != nil {
		log.Println("err", err)
		return []reconcile.Request{}
	}

	log.Println("Reconcile model cache: ", name)

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name: name,
		}}}
}

func (c *CachedModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.PersistentVolumeClaim{}, ownerKey, func(rawObj client.Object) []string {
		pvc := rawObj.(*v1.PersistentVolumeClaim)
		owner := metav1.GetControllerOf(pvc)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != "serving.kserve.io/v1alpha1" || owner.Kind != "ClusterCachedModel" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1beta1.InferenceService{}, ownerKey, func(rawObj client.Object) []string {
		isvc := rawObj.(*v1beta1.InferenceService)
		if model, ok := isvc.GetLabels()[constants.ModelCacheEnabled]; ok {
			return []string{model}
		}
		return nil
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1api.ClusterCachedModel{}).
		Owns(&batchv1.Job{}).
		Owns(&v1.PersistentVolume{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Watches(&v1beta1.InferenceService{}, handler.EnqueueRequestsFromMapFunc(c.myFunc)).
		Complete(c)
}
