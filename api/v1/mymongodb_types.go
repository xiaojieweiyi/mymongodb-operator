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

package v1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UnitRelationResourceSpec struct {
	Service *OwnService `json:"serviceInfo,omitempty"`
	PVC     *OwnPVC     `json:"pvcInfo,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MymongodbSpec defines the desired state of Mymongodb
type MymongodbSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Mymongodb. Edit mymongodb_types.go to remove/update
	Replicas *int32                `json:"replicas,omitempty"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Template describes the pods that will be created.

	Template         corev1.PodTemplateSpec   `json:"template"`
	RelationResource UnitRelationResourceSpec `json:"relationResource,omitempty"`
}

type UnitRelationResourceStatus struct {
	Service UnitRelationServiceStatus `json:"service,omitempty"`

	Endpoint []UnitRelationEndpointStatus       `json:"endpoint,omitempty"`
	PVC      corev1.PersistentVolumeClaimStatus `json:"pvc,omitempty"`
}

// MymongodbStatus defines the observed state of Mymongodb
type MymongodbStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Replicas               *int32                     `json:"replicas,omitempty"`
	Selector               string                     `json:"selector"`
	LastUpdateTime         metav1.Time                `json:"lastUpdateTime,omitempty"`
	BaseStatefulSet        appsv1.StatefulSetStatus   `json:"statefulSet,omitempty"`
	RelationResourceStatus UnitRelationResourceStatus `json:"relationResourceStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector

// Mymongodb is the Schema for the mymongodbs API
type Mymongodb struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MymongodbSpec   `json:"spec,omitempty"`
	Status MymongodbStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MymongodbList contains a list of Mymongodb
type MymongodbList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mymongodb `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mymongodb{}, &MymongodbList{})
}
