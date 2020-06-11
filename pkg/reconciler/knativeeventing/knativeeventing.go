/*
Copyright 2019 The Knative Authors

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

package knativeeventing

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientset "knative.dev/operator/pkg/client/clientset/versioned"

	"knative.dev/operator/pkg/apis/operator/v1alpha1"
	knereconciler "knative.dev/operator/pkg/client/injection/reconciler/operator/v1alpha1/knativeeventing"
	"knative.dev/operator/pkg/reconciler/common"
	kec "knative.dev/operator/pkg/reconciler/knativeeventing/common"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	oldFinalizerName = "delete-knative-eventing-manifest"
)

// Reconciler implements controller.Reconciler for KnativeEventing resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// kubeClientSet allows us to talk to the k8s for operator APIs
	operatorClientSet clientset.Interface
	// manifest is empty but with a valid client and logger
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	platform common.Extension
}

// Check that our Reconciler implements controller.Reconciler
var _ knereconciler.Interface = (*Reconciler)(nil)
var _ knereconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a KnativeEventing.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.KnativeEventing) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all KnativeEventings to determine if cluster-scoped resources should be deleted.
	kes, err := r.operatorClientSet.OperatorV1alpha1().KnativeEventings("").List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all KnativeEventings: %w", err)
	}

	for _, ke := range kes.Items {
		if ke.GetDeletionTimestamp().IsZero() {
			// Not deleting all KnativeEventings. Nothing to do here.
			return nil
		}
	}

	// TODO: which manifest should we delete? both?
	version := ""
	if original.GetStatus().IsReady() {
		version = original.Status.Version
	} else {
		version = common.TargetRelease(original)
	}

	manifest, err := common.FetchManifest(common.ManifestPath(version, common.PathElement(original)))
	if err != nil {
		return err
	}
	manifest = r.manifest.Append(manifest)
	if err := r.transform(ctx, &manifest, original); err != nil {
		return err
	}
	logger.Info("Deleting cluster-scoped resources")
	return common.Uninstall(&manifest)
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, ke *v1alpha1.KnativeEventing) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	ke.Status.InitializeConditions()
	ke.Status.ObservedGeneration = ke.Generation

	logger.Infow("Reconciling KnativeEventing", "status", ke.Status)
	stages := []func(context.Context, *mf.Manifest, v1alpha1.KComponent) error{
		r.create,
		r.transform,
		r.ensureFinalizerRemoval,
		r.install,
		r.checkDeployments,
		r.deleteObsoleteResources,
		r.updateVersion,
	}

	// Appending nothing effectively results in a deep-copy clone
	manifest := r.manifest.Append()
	for _, stage := range stages {
		if err := stage(ctx, &manifest, ke); err != nil {
			return err
		}
	}
	logger.Infow("Reconcile stages complete", "status", ke.Status)
	return nil
}

// create mutates the passed manifest, appending to it one appropriate
// for the specified version in the instance
func (r *Reconciler) create(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.KComponent) error {
	path := common.ManifestPath(common.TargetRelease(instance), common.PathElement(instance))
	m, err := common.FetchManifest(path)
	if err != nil {
		return err
	}
	*manifest = manifest.Append(m)
	return nil
}

// transform mutates the passed manifest to one with common and
// platform transforms, plus any extras passed in
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, component v1alpha1.KComponent) error {
	logger := logging.FromContext(ctx)
	instance := component.(*v1alpha1.KnativeEventing)
	return common.Transform(ctx, manifest, instance, r.platform,
		kec.DefaultBrokerConfigMapTransform(instance, logger))
}

// ensureFinalizerRemoval ensures that the obsolete "delete-knative-eventing-manifest" is removed from the resource.
func (r *Reconciler) ensureFinalizerRemoval(_ context.Context, _ *mf.Manifest, instance v1alpha1.KComponent) error {
	patch, err := common.FinalizerRemovalPatch(instance, oldFinalizerName)
	if err != nil {
		return fmt.Errorf("failed to construct the patch: %w", err)
	}
	if patch == nil {
		// Nothing to do here.
		return nil
	}

	patcher := r.operatorClientSet.OperatorV1alpha1().KnativeEventings(instance.GetNamespace())
	if _, err := patcher.Patch(instance.GetName(), types.MergePatchType, patch); err != nil {
		return fmt.Errorf("failed to patch finalizer away: %w", err)
	}
	return nil
}

func (r *Reconciler) install(ctx context.Context, manifest *mf.Manifest, ke v1alpha1.KComponent) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Installing manifest")
	return common.Install(manifest, ke.GetStatus())
}

func (r *Reconciler) checkDeployments(ctx context.Context, manifest *mf.Manifest, ke v1alpha1.KComponent) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Checking deployments")
	return common.CheckDeployments(r.kubeClientSet, manifest, ke.GetStatus())
}

// Delete obsolete resources from previous versions
func (r *Reconciler) deleteObsoleteResources(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.KComponent) error {
	if instance.GetStatus().GetVersion() == "" {
		return nil
	}
	if common.TargetRelease(instance) == instance.GetStatus().GetVersion() {
		return nil
	}
	logger := logging.FromContext(ctx)
	m, err := common.FetchManifest(common.ManifestPath(instance.GetStatus().GetVersion(), common.PathElement(instance)))
	if err != nil {
		logger.Error(err, "Unable to fetch previous manifest; some obsolete resources may remain")
		return nil
	}
	m = r.manifest.Append(m)
	if err := r.transform(ctx, &m, instance); err != nil {
		return err
	}
	return m.Filter(mf.None(mf.In(*manifest))).Delete()
}

// updateVersion sets the status version if all conditions are satisfied
func (r *Reconciler) updateVersion(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.KComponent) error {
	if instance.GetStatus().IsReady() {
		instance.GetStatus().SetVersion(common.TargetRelease(instance))
	}
	return nil
}
