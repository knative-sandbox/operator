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

package knativeserving

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"github.com/go-logr/zapr"

	mf "github.com/manifestival/manifestival"
	clientset "knative.dev/operator/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	servingv1alpha1 "knative.dev/operator/pkg/apis/operator/v1alpha1"
	knsreconciler "knative.dev/operator/pkg/client/injection/reconciler/operator/v1alpha1/knativeserving"
	"knative.dev/operator/pkg/reconciler/common"
	ksc "knative.dev/operator/pkg/reconciler/knativeserving/common"
	"knative.dev/operator/version"
	"knative.dev/pkg/logging"

	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	oldFinalizerName = "delete-knative-serving-manifest"
)

// Reconciler implements controller.Reconciler for Knativeserving resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifests is the map of Knative Serving manifests
	manifests map[string]mf.Manifest
	// mfClient is the client needed for manifestival.
	mfClient mf.Client
	// Platform-specific behavior to affect the transform
	platform common.Platforms
}

// Check that our Reconciler implements controller.Reconciler
var _ knsreconciler.Interface = (*Reconciler)(nil)
var _ knsreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a KnativeServing.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *servingv1alpha1.KnativeServing) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all KnativeServings to determine if cluster-scoped resources should be deleted.
	kss, err := r.operatorClientSet.OperatorV1alpha1().KnativeServings("").List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all KnativeServings: %w", err)
	}

	for _, ks := range kss.Items {
		if ks.GetDeletionTimestamp().IsZero() {
			// Not deleting all KnativeServings. Nothing to do here.
			return nil
		}
	}

	manifest, err := r.getCurrentManifest(ctx, original)
	if err != nil {
		return fmt.Errorf("failed to transform manifest: %w", err)
	}

	logger.Info("Deleting cluster-scoped resources")
	return common.Uninstall(&manifest)
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, ks *servingv1alpha1.KnativeServing) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	ks.Status.InitializeConditions()
	ks.Status.ObservedGeneration = ks.Generation

	logger.Infow("Reconciling KnativeServing", "status", ks.Status)

	// Get the Serving Manifest to be installed
	servingManifest, err := r.getTargetManifest(ctx, ks)
	if err != nil {
		ks.Status.MarkInstallFailed(err.Error())
		return err
	}

	// Get the previous Manifest, which has been installed.
	oldManifest, err := r.getCurrentManifest(ctx, ks)
	if err != nil {
		return err
	}

	stages := []func(context.Context, *mf.Manifest, *servingv1alpha1.KnativeServing) error{
		r.ensureFinalizerRemoval,
		r.install,
		r.checkDeployments,
	}

	for _, stage := range stages {
		if err := stage(ctx, &servingManifest, ks); err != nil {
			return err
		}
	}

	// Remove the resources, that does not exist in the new Serving manifest
	if err := oldManifest.Filter(mf.None(mf.In(servingManifest))).Delete(); err != nil {
		return err
	}

	logger.Infow("Reconcile stages complete", "status", ks.Status)
	return nil
}

// Transform the resources
func (r *Reconciler) transform(ctx context.Context, instance *servingv1alpha1.KnativeServing) ([]mf.Transformer, error) {
	logger := logging.FromContext(ctx)

	logger.Debug("Transforming manifest")

	platform, err := r.platform.Transformers(r.kubeClientSet, logger)
	if err != nil {
		return []mf.Transformer{}, err
	}

	transformers := common.Transformers(ctx, instance)
	transformers = append(transformers,
		ksc.GatewayTransform(instance, logger),
		ksc.CustomCertsTransform(instance, logger),
		ksc.HighAvailabilityTransform(instance, logger),
		ksc.AggregationRuleTransform(r.mfClient))
	return append(transformers, platform...), nil
}

// Apply the manifest resources
func (r *Reconciler) install(ctx context.Context, manifest *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Installing manifest")
	return common.Install(manifest, instance.Spec.GetVersion(), &instance.Status)
}

// Check for all deployments available
func (r *Reconciler) checkDeployments(ctx context.Context, manifest *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Checking deployments")
	return common.CheckDeployments(r.kubeClientSet, manifest, &instance.Status)
}

// ensureFinalizerRemoval ensures that the obsolete "delete-knative-serving-manifest" is removed from the resource.
func (r *Reconciler) ensureFinalizerRemoval(_ context.Context, _ *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	patch, err := common.FinalizerRemovalPatch(instance, oldFinalizerName)
	if err != nil {
		return fmt.Errorf("failed to construct the patch: %w", err)
	}
	if patch == nil {
		// Nothing to do here.
		return nil
	}

	patcher := r.operatorClientSet.OperatorV1alpha1().KnativeServings(instance.Namespace)
	if _, err := patcher.Patch(instance.Name, types.MergePatchType, patch); err != nil {
		return fmt.Errorf("failed to patch finalizer away: %w", err)
	}
	return nil
}

func (r *Reconciler) retrieveManifest(ctx context.Context, version string, instance *servingv1alpha1.KnativeServing) (mf.Manifest, error) {
	if val, found := r.manifests[version]; found {
		return val, nil
	}

	logger := logging.FromContext(ctx)
	koDataDir := os.Getenv("KO_DATA_PATH")
	manifesrDir := fmt.Sprintf("knative-serving/%s", version)
	manifest, err := mf.NewManifest(filepath.Join(koDataDir, manifesrDir),
		mf.UseClient(r.mfClient),
		mf.UseLogger(zapr.NewLogger(logger.Desugar()).WithName("manifestival")))

	if err != nil {
		return manifest, err
	}

	if len(manifest.Resources()) == 0 {
		return manifest, fmt.Errorf("unable to find the manifest for the Knative Serving version %s", version)
	}

	// Transform the manifest
	transformers, err := r.transform(ctx, instance)
	if err != nil {
		return mf.Manifest{}, err
	}

	manifestTransformed, err := manifest.Transform(transformers...)
	if err != nil {
		return mf.Manifest{}, err
	}

	// Save the transformed manifest in the map
	r.manifests[version] = manifestTransformed

	return manifest, nil
}

func (r *Reconciler) getTargetManifest(ctx context.Context, instance *servingv1alpha1.KnativeServing) (mf.Manifest, error) {
	// The version is set to the default version
	version := version.ServingVersion
	if instance.Spec.GetVersion() != "" {
		// If the version is set in spec of the CR, pick the version from the spec of the CR
		version = instance.Spec.GetVersion()
	} else if instance.Status.GetVersion() != "" {
		// If the version is not set in the spec of the CR, use the one in status of the CR
		version = instance.Status.GetVersion()
	}

	return r.retrieveManifest(ctx, version, instance)
}

func (r *Reconciler) getCurrentManifest(ctx context.Context, instance *servingv1alpha1.KnativeServing) (mf.Manifest, error) {
	// The version is set to the default version
	version := version.ServingVersion
	if instance.Status.GetVersion() != "" {
		// If the version is set in the status of the CR, pick the version from the status of the CR
		version = instance.Status.GetVersion()
	}
	return r.retrieveManifest(ctx, version, instance)
}
