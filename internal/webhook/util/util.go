package util

import (
	"os"
	"strconv"

	"k8s.io/klog/v2"
)

func GetMutatingWebhookName() string {
	if name := os.Getenv("MUTATING_WEBHOOK_NAME"); len(name) > 0 {
		return name
	}
	return "persistentpodstate-mutating-webhook-configuration"
}

func GetValidatingWebhookName() string {
	if name := os.Getenv("VALIDATING_WEBHOOK_NAME"); len(name) > 0 {
		return name
	}
	return "persistentpodstate-validating-webhook-configuration"
}

func GetHost() string {
	return os.Getenv("WEBHOOK_HOST")
}

func GetNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); len(ns) > 0 {
		return ns
	}
	return "persistent-pod-state-system"
}

func GetSecretName() string {
	if name := os.Getenv("SECRET_NAME"); len(name) > 0 {
		return name
	}
	return "persistentpodstate-webhook-certs"
}

func GetServiceName() string {
	if name := os.Getenv("SERVICE_NAME"); len(name) > 0 {
		return name
	}
	return "persistentpodstate-webhook-service"
}

func GetPort() int {
	port := 9876
	if p := os.Getenv("WEBHOOK_PORT"); len(p) > 0 {
		if p, err := strconv.ParseInt(p, 10, 32); err == nil {
			port = int(p)
		} else {
			klog.Fatalf("failed to convert WEBHOOK_PORT=%v in env: %v", p, err)
		}
	}
	return port
}

func GetCertDir() string {
	if p := os.Getenv("WEBHOOK_CERT_DIR"); len(p) > 0 {
		return p
	}
	return "/tmp/persistentpodstate-webhook-certs"
}

func GetCertWriter() string {
	return os.Getenv("WEBHOOK_CERT_WRITER")
}
