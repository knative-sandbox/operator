/*
Copyright 2019 The Knative Authors.

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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

var eventingCondSet = apis.NewLivingConditionSet(
	DependenciesInstalled,
	DeploymentsAvailable,
	InstallSucceeded,
)

// GroupVersionKind returns SchemeGroupVersion of an KnativeEventing
func (e *KnativeEventing) GroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(Kind)
}

// GetCondition returns the current condition of a given condition type
func (es *KnativeEventingStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return eventingCondSet.Manage(es).GetCondition(t)
}

// InitializeConditions initializes conditions of an KnativeEventingStatus
func (es *KnativeEventingStatus) InitializeConditions() {
	eventingCondSet.Manage(es).InitializeConditions()
}

// IsReady looks at the conditions and if the Status has a condition
// EventingConditionReady returns true if ConditionStatus is True
func (es *KnativeEventingStatus) IsReady() bool {
	return eventingCondSet.Manage(es).IsHappy()
}

// MarkInstallSucceeded marks the InstallationSucceeded status as true.
func (es *KnativeEventingStatus) MarkInstallSucceeded() {
	eventingCondSet.Manage(es).MarkTrue(InstallSucceeded)
	if es.GetCondition(DependenciesInstalled).IsUnknown() {
		// Assume deps are installed if we're not sure
		es.MarkDependenciesInstalled()
	}
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (es *KnativeEventingStatus) MarkInstallFailed(msg string) {
	eventingCondSet.Manage(es).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (es *KnativeEventingStatus) MarkDeploymentsAvailable() {
	eventingCondSet.Manage(es).MarkTrue(DeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (es *KnativeEventingStatus) MarkDeploymentsNotReady() {
	eventingCondSet.Manage(es).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}

// MarkDependenciesInstalled marks the DependenciesInstalled status as true.
func (es *KnativeEventingStatus) MarkDependenciesInstalled() {
	eventingCondSet.Manage(es).MarkTrue(DependenciesInstalled)
}

// MarkDependencyInstalling marks the DependenciesInstalled status as false with the
// given message.
func (es *KnativeEventingStatus) MarkDependencyInstalling(msg string) {
	eventingCondSet.Manage(es).MarkFalse(
		DependenciesInstalled,
		"Installing",
		"Dependency installing: %s", msg)
}

// MarkDependencyMissing marks the DependenciesInstalled status as false with the
// given message.
func (es *KnativeEventingStatus) MarkDependencyMissing(msg string) {
	eventingCondSet.Manage(es).MarkFalse(
		DependenciesInstalled,
		"Error",
		"Dependency missing: %s", msg)
}
