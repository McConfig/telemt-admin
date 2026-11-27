package configx

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// Marshal сериализует бандл в YAML с отступом 2.
func Marshal(b *Bundle) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode разбирает YAML строго: без алиасов, дублей и неизвестных полей.
func Decode(data []byte) (*Bundle, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("bad_yaml: %w", err)
	}
	if err := checkNode(&root); err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var b Bundle
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("bad_yaml: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("bad_yaml: несколько документов в файле")
	}
	if b.APIVersion != APIVersion {
		return nil, fmt.Errorf("unsupported_version: %q", b.APIVersion)
	}
	if b.Kind != Kind {
		return nil, fmt.Errorf("bad_yaml: неизвестный kind %q", b.Kind)
	}
	if b.Metadata.Name == "" {
		return nil, errors.New("bad_yaml: metadata.name обязателен")
	}
	if b.Metadata.Secrets != SecretsNone && b.Metadata.Secrets != SecretsIncluded {
		return nil, errors.New("bad_yaml: metadata.secrets должен быть none или included")
	}
	return &b, nil
}

const (
	maxYAMLDepth = 64
	maxYAMLNodes = 100000
)

func checkNode(n *yaml.Node) error {
	count := 0
	return checkNodeDepth(n, 0, &count)
}

func checkNodeDepth(n *yaml.Node, depth int, count *int) error {
	*count++
	if *count > maxYAMLNodes {
		return errors.New("bad_yaml: слишком много узлов")
	}
	if depth > maxYAMLDepth {
		return errors.New("bad_yaml: слишком глубокая вложенность")
	}
	if n.Kind == yaml.AliasNode {
		return errors.New("bad_yaml: алиасы запрещены")
	}
	if n.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if _, ok := seen[key]; ok {
				return fmt.Errorf("bad_yaml: дубль ключа %q", key)
			}
			seen[key] = struct{}{}
		}
	}
	for _, child := range n.Content {
		if err := checkNodeDepth(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}
