/*
Copyright 2022 The Tekton Authors

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

package config

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"

	cm "knative.dev/pkg/configmap"
)

// TrustedResources holds the collection of configurations that we attach to contexts.
// Configmap named with "config-trusted-resources" where cosign pub key path and
// KMS pub key path can be configured
// +k8s:deepcopy-gen=true
type TrustedResources struct {
	// CosignKey defines the name of the key in configmap data
	CosignKey string
	// KmsKey defines the name of the key in configmap data
	KMSKey string
}

const (
	// CosignPubKey is the name of the cosign key in configmap data
	CosignPubKey = "cosign-pubkey-path"
	// DefaultSecretPath is the default path of cosign public key
	DefaultSecretPath = "/etc/signing-secrets/cosign.pub"
	// KMSPubKey is the name of the KMS key in configmap data
	KMSPubKey = "kms-pubkey-path"
	// TrustedTaskConfig is the name of the trusted resources configmap
	TrustedTaskConfig = "config-trusted-resources"
)

func defaultConfig() *Config {
	return &Config{
		TrustedResources: &TrustedResources{
			CosignKey: DefaultSecretPath,
		},
	}
}

// NewTrustedResourcesConfigFromMap creates a Config from the supplied map
func NewTrustedResourcesConfigFromMap(data map[string]string) (*TrustedResources, error) {
	cfg := &TrustedResources{
		CosignKey: DefaultSecretPath,
	}
	if err := cm.Parse(data,
		cm.AsString(CosignPubKey, &cfg.CosignKey),
		cm.AsString(KMSPubKey, &cfg.KMSKey),
	); err != nil {
		return nil, fmt.Errorf("failed to parse data: %w", err)
	}
	return cfg, nil
}

// NewTrustedResourcesConfigFromConfigMap creates a Config from the supplied ConfigMap
func NewTrustedResourcesConfigFromConfigMap(configMap *corev1.ConfigMap) (*TrustedResources, error) {
	return NewTrustedResourcesConfigFromMap(configMap.Data)
}

// GetTrustedResourcesConfigName returns the name of TrustedResources ConfigMap
func GetTrustedResourcesConfigName() string {
	if e := os.Getenv("CONFIG_Trusted_RESOURCES_NAME"); e != "" {
		return e
	}
	return TrustedTaskConfig
}
