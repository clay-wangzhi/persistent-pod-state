package hack

import (
	// To generate clientset/informers/listers for apis
	_ "k8s.io/code-generator"
	// submodule test dependencies
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1670
	// _ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
)
