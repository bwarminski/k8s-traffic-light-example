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

// StoplightSpec defines the desired state of Stoplight
type StoplightSpec struct {
	GreenLightDurationSec  int64 `json:"greenLightDurationSec,omitempty"`
	RedLightDurationSec    int64 `json:"redLightDurationSec,omitempty"`
	YellowLightDurationSec int64 `json:"yellowLightDurationSec,omitempty"`
}

// StoplightStatus defines the observed state of Stoplight
type StoplightStatus struct {
	Color          StoplightColor `json:"color,omitempty"`
	LastTransition int64          `json:"lastTransition,omitempty"`
	EmergencyMode  bool           `json:"emergencyMode,omitempty"`
}

type StoplightColor string

const (
	Red    StoplightColor = "Red"
	Yellow StoplightColor = "Yellow"
	Green  StoplightColor = "Green"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Stoplight is the Schema for the stoplights API
// +k8s:openapi-gen=true
type Stoplight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoplightSpec   `json:"spec,omitempty"`
	Status StoplightStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StoplightList contains a list of Stoplight
type StoplightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stoplight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Stoplight{}, &StoplightList{})
}
