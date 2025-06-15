/*
Copyright 2025 flemzord.

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

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	devv1alpha1 "github.com/flemzord/mutating-registry-webhook/api/v1alpha1"
)

// RulesWatcher watches for changes to RegistryRewriteRule resources
type RulesWatcher struct {
	client.Client
	Mutator *PodMutator
}

// Reconcile handles changes to RegistryRewriteRule resources
func (r *RulesWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("RegistryRewriteRule changed, invalidating cache", "name", req.Name)

	// Check if the resource still exists
	rule := &devv1alpha1.RegistryRewriteRule{}
	err := r.Get(ctx, req.NamespacedName, rule)
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource was deleted
			log.Info("RegistryRewriteRule deleted", "name", req.Name)
		} else {
			log.Error(err, "Failed to get RegistryRewriteRule", "name", req.Name)
			return reconcile.Result{}, err
		}
	}

	// Invalidate the cache
	r.Mutator.InvalidateCache()

	// Update status if the resource exists
	if err == nil {
		rule.Status.ObservedGeneration = rule.Generation
		rule.Status.Ready = true
		rule.Status.RuleCount = len(rule.Spec.Rules)
		now := r.now()
		rule.Status.LastUpdateTime = &now

		if err := r.Status().Update(ctx, rule); err != nil {
			log.Error(err, "Failed to update RegistryRewriteRule status", "name", req.Name)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// now returns the current time
func (r *RulesWatcher) now() metav1.Time {
	return metav1.Now()
}
