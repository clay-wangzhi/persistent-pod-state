/*
Copyright 2022 The Koordinator Authors.

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

package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"

	utilfeature "github.com/clay-wangzhi/persistent-pod-state/pkg/util/feature"
)

const (
	// PodMutatingWebhook enables mutating webhook for Pods creations.
	PodMutatingWebhook featuregate.Feature = "PodMutatingWebhook"
)

var defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	PodMutatingWebhook: {Default: true, PreRelease: featuregate.Beta},
}

const (
	DisablePVCReservation featuregate.Feature = "DisablePVCReservation"
)

var defaultDeschedulerFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	DisablePVCReservation: {Default: false, PreRelease: featuregate.Beta},
}

func init() {
	runtime.Must(utilfeature.DefaultMutableFeatureGate.Add(defaultFeatureGates))
	runtime.Must(utilfeature.DefaultMutableFeatureGate.Add(defaultDeschedulerFeatureGates))
}

func SetDefaultFeatureGates() {

}
