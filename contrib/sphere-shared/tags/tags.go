package tags

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	rComment = regexp.MustCompile(`^//.*?@(?i:sphere?)=\s*(.*)$`)
	rTags    = regexp.MustCompile(`[\w_]+:"[^"]+"`)
)

func FromComment(comment string) string {
	match := rComment.FindStringSubmatch(strings.TrimSpace(comment))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

type Item struct {
	Key   string
	Value string
}

type Items []Item

func (t Items) Format() string {
	var tags []string
	for _, item := range t {
		tags = append(tags, fmt.Sprintf(`%s:%s`, item.Key, item.Value))
	}
	return strings.Join(tags, " ")
}

func (t Items) Override(items Items) Items {
	var override []Item
	for i := range t {
		dup := -1
		for j := range items {
			if t[i].Key == items[j].Key {
				dup = j
				break
			}
		}
		if dup == -1 {
			override = append(override, t[i])
		} else {
			override = append(override, items[dup])
			items = append(items[:dup], items[dup+1:]...)
		}
	}
	return append(override, items...)
}

func NewTagItems(tag string) Items {
	var items []Item
	split := rTags.FindAllString(tag, -1)
	for _, t := range split {
		sepPos := strings.Index(t, ":")
		items = append(items, Item{
			Key:   t[:sepPos],
			Value: t[sepPos+1:],
		})
	}
	return items
}

func NewSphereTagItems(raw, protoName string) Items {
	var items Items
	if raw == "" {
		return items
	}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			kvParts := strings.SplitN(part, "=", 2)
			if len(kvParts) != 2 {
				continue
			}
			items = append(items, Item{
				Key:   kvParts[0],
				Value: strings.TrimSpace(kvParts[1]),
			})
		} else if protoName != "" {
			items = append(items, Item{
				Key:   part,
				Value: fmt.Sprintf("\"%s\"", protoName),
			})
		}
	}
	return items
}

func GetProtoTagName(tags Items) string {
	for _, item := range tags {
		if item.Key != "protobuf" {
			continue
		}
		cmp := strings.Split(item.Value, ",")
		for _, c := range cmp {
			if strings.HasPrefix(c, "name=") {
				return strings.TrimPrefix(c, "name=")
			}
		}
	}
	return ""
}
