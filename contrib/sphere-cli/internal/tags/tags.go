package tags

import (
	"fmt"
	"strings"
)

type tagItem struct {
	key   string
	value string
}

type tagItems []tagItem

func (ti tagItems) format() string {
	var tags []string
	for _, item := range ti {
		tags = append(tags, fmt.Sprintf(`%s:%s`, item.key, item.value))
	}
	return strings.Join(tags, " ")
}

func (ti tagItems) override(nti tagItems) tagItems {
	var override []tagItem
	for i := range ti {
		dup := -1
		for j := range nti {
			if ti[i].key == nti[j].key {
				dup = j
				break
			}
		}
		if dup == -1 {
			override = append(override, ti[i])
		} else {
			override = append(override, nti[dup])
			nti = append(nti[:dup], nti[dup+1:]...)
		}
	}
	return append(override, nti...)
}

func newTagItems(tag string) tagItems {
	var items []tagItem
	split := rTags.FindAllString(tag, -1)
	for _, t := range split {
		sepPos := strings.Index(t, ":")
		items = append(items, tagItem{
			key:   t[:sepPos],
			value: t[sepPos+1:],
		})
	}
	return items
}

func newSphereTagItems(tag, protoName string) tagItems {
	var items tagItems
	if tag == "" {
		return items
	}
	parts := strings.Split(tag, ",")
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
			items = append(items, tagItem{
				key:   kvParts[0],
				value: strings.TrimSpace(kvParts[1]),
			})
		} else if protoName != "" {
			items = append(items, tagItem{
				key:   part,
				value: fmt.Sprintf("\"%s\"", protoName),
			})
		}
	}
	return items
}

func defaultProtoTagName(tags tagItems) string {
	for _, item := range tags {
		if item.key != "protobuf" {
			continue
		}
		cmp := strings.Split(item.value, ",")
		for _, c := range cmp {
			if strings.HasPrefix(c, "name=") {
				return strings.TrimPrefix(c, "name=")
			}
		}
	}
	return ""
}
