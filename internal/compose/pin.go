package compose

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PinImages sets the tag of every compose service whose image repository (the
// image without its tag) is a key of tags, and reports which services changed
// per repository. Services are matched by image, not by name, so any service
// reusing a built image moves with it; images not in tags are left alone.
//
// Only the image values are rewritten in place, so comments and formatting in
// the rest of the file survive.
func PinImages(content []byte, tags map[string]string) ([]byte, map[string][]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, nil, fmt.Errorf("failed to parse docker-compose.prod.yml: %w", err)
	}

	lines := strings.SplitAfter(string(content), "\n")
	updated := map[string][]string{}

	var root *yaml.Node
	if len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	services := mappingValue(root, "services")
	if services != nil && services.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(services.Content); i += 2 {
			name := services.Content[i].Value
			image := mappingValue(services.Content[i+1], "image")
			if image == nil || image.Kind != yaml.ScalarNode {
				continue
			}
			repository := imageRepository(image.Value)
			tag, ok := tags[repository]
			if !ok {
				continue
			}
			// Search from the node's own column, not the line start: in flow
			// style several services share one line.
			line := &lines[image.Line-1]
			from := byteOffset(*line, image.Column-1)
			rel := strings.Index((*line)[from:], image.Value)
			if rel < 0 {
				return nil, nil, fmt.Errorf(
					"cannot update the image of service %s on line %d of docker-compose.prod.yml; write it on one line",
					name,
					image.Line,
				)
			}
			start := from + rel
			*line = (*line)[:start] + repository + ":" + tag + (*line)[start+len(image.Value):]
			updated[repository] = append(updated[repository], name)
		}
	}

	for _, names := range updated {
		sort.Strings(names)
	}
	return []byte(strings.Join(lines, "")), updated, nil
}

// mappingValue returns the value node for key in a YAML mapping, or nil.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// byteOffset converts a 0-based character column, as yaml.v3 reports it, to a
// byte offset in line, clamped to the line's length.
func byteOffset(line string, column int) int {
	for i := range line {
		if column <= 0 {
			return i
		}
		column--
	}
	return len(line)
}

// imageRepository strips the tag and digest from an image reference. A colon
// before the last "/" belongs to a registry port, not a tag.
func imageRepository(image string) string {
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	lastSlash := strings.LastIndex(image, "/")
	if colon := strings.Index(image[lastSlash+1:], ":"); colon >= 0 {
		return image[:lastSlash+1+colon]
	}
	return image
}
