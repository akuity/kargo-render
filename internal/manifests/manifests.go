package manifests

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
)

func JSONStringsToYAMLBytes(jsonManifests []string) ([][]byte, error) {
	yamlManifests := make([][]byte, len(jsonManifests))
	for i, jsonManifest := range jsonManifests {
		var err error
		if yamlManifests[i], err =
			yaml.JSONToYAML([]byte(jsonManifest)); err != nil {
			return nil,
				fmt.Errorf("error converting JSON manifest to YAML: %w", err)
		}
	}
	return yamlManifests, nil
}

func CombineYAML(manifests [][]byte) []byte {
	return bytes.Join(manifests, []byte("---\n"))
}

func SplitYAML(manifest []byte) (map[string][]byte, error) {
	manifests := bytes.Split(manifest, []byte("---\n"))
	manifestsByResourceTypeAndName := map[string][]byte{}
	for _, manifest = range manifests {
		resource := struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}{}
		if err := yaml.Unmarshal(manifest, &resource); err != nil {
			return nil, fmt.Errorf("error unmarshaling resource: %w", err)
		}
		if resource.Kind == "" {
			return nil, errors.New("resource is missing kind field")
		}
		if resource.Metadata.Name == "" {
			return nil, errors.New("resource is missing metadata.name field")
		}
		resourceTypeAndName := fmt.Sprintf(
			"%s-%s",
			strings.ToLower(resource.Metadata.Name),
			strings.ToLower(resource.Kind),
		)
		manifestsByResourceTypeAndName[resourceTypeAndName] = manifest
	}
	return manifestsByResourceTypeAndName, nil
}
