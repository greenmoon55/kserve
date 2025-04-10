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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	servingv1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// LocalModelCacheLister helps list LocalModelCaches.
// All objects returned here must be treated as read-only.
type LocalModelCacheLister interface {
	// List lists all LocalModelCaches in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servingv1alpha1.LocalModelCache, err error)
	// LocalModelCaches returns an object that can list and get LocalModelCaches.
	LocalModelCaches(namespace string) LocalModelCacheNamespaceLister
	LocalModelCacheListerExpansion
}

// localModelCacheLister implements the LocalModelCacheLister interface.
type localModelCacheLister struct {
	listers.ResourceIndexer[*servingv1alpha1.LocalModelCache]
}

// NewLocalModelCacheLister returns a new LocalModelCacheLister.
func NewLocalModelCacheLister(indexer cache.Indexer) LocalModelCacheLister {
	return &localModelCacheLister{listers.New[*servingv1alpha1.LocalModelCache](indexer, servingv1alpha1.Resource("localmodelcache"))}
}

// LocalModelCaches returns an object that can list and get LocalModelCaches.
func (s *localModelCacheLister) LocalModelCaches(namespace string) LocalModelCacheNamespaceLister {
	return localModelCacheNamespaceLister{listers.NewNamespaced[*servingv1alpha1.LocalModelCache](s.ResourceIndexer, namespace)}
}

// LocalModelCacheNamespaceLister helps list and get LocalModelCaches.
// All objects returned here must be treated as read-only.
type LocalModelCacheNamespaceLister interface {
	// List lists all LocalModelCaches in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servingv1alpha1.LocalModelCache, err error)
	// Get retrieves the LocalModelCache from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*servingv1alpha1.LocalModelCache, error)
	LocalModelCacheNamespaceListerExpansion
}

// localModelCacheNamespaceLister implements the LocalModelCacheNamespaceLister
// interface.
type localModelCacheNamespaceLister struct {
	listers.ResourceIndexer[*servingv1alpha1.LocalModelCache]
}
