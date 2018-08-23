/*
Copyright 2018 Brett Warminski.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CrosswalkSignalSpec defines the desired state of CrosswalkSignal
type CrosswalkSignalSpec struct {
	Stoplight                   string `json:"stoplight,omitempty"`
	FlashingHandDurationSec     int64  `json:"flashingHandDurationSec,omitempty"`
	GreenLightBufferDurationSec int64  `json:"greenLightBufferDurationSec,omitempty"`
}

// CrosswalkSignalStatus defines the observed state of CrosswalkSignal
type CrosswalkSignalStatus struct {
	Symbol         CrosswalkSymbol `json:"symbol,omitempty"`
	LastTransition int64           `json:"lastTransition,omitempty"`
}

type CrosswalkSymbol string

const (
	Walk       CrosswalkSymbol = "Walk"
	FlashyHand CrosswalkSymbol = "FlashyHand"
	DontWalk   CrosswalkSymbol = "DontWalk"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CrosswalkSignal is the Schema for the crosswalksignals API
// +k8s:openapi-gen=true
type CrosswalkSignal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CrosswalkSignalSpec   `json:"spec,omitempty"`
	Status CrosswalkSignalStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CrosswalkSignalList contains a list of CrosswalkSignal
type CrosswalkSignalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CrosswalkSignal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CrosswalkSignal{}, &CrosswalkSignalList{})
}
