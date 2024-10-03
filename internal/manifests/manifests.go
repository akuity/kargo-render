package manifests

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
	libyaml "sigs.k8s.io/yaml"
)

func JSONStringsToYAMLBytes(jsonManifests []string) ([][]byte, error) {
	yamlManifests := make([][]byte, len(jsonManifests))

	for i, jsonManifest := range jsonManifests {
		var err error
		if yamlManifests[i], err =
			libyaml.JSONToYAML([]byte(jsonManifest)); err != nil {
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
	dec := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(manifest)))
	manifestsByResourceTypeAndName := map[string][]byte{}
	for {
		manifest, err := dec.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("error reading YAML document: %w", err)
		}

		resource := struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}{}
		if err := libyaml.Unmarshal(manifest, &resource); err != nil {
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
