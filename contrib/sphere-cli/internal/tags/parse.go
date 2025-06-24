package tags

import (
	"fmt"
	"strings"
)

func tagFromComment(comment string) (tag string) {
	match := rComment.FindStringSubmatch(strings.TrimSpace(comment))
	if len(match) == 2 {
		tag = match[1]
	}
	return
}

func injectTag(contents []byte, area textArea, removeTagComment bool) (injected []byte) {
	expr := make([]byte, area.End-area.Start)
	copy(expr, contents[area.Start-1:area.End-1])
	cti := newTagItems(area.CurrentTag)
	protoName := defaultProtoTagName(cti)
	iti := newSphereTagItems(area.InjectTag, protoName)
	ti := cti.override(iti)
	expr = rInject.ReplaceAll(expr, []byte(fmt.Sprintf("`%s`", ti.format())))
	if removeTagComment {
		strippedComment := make([]byte, area.CommentEnd-area.CommentStart)
		copy(strippedComment, contents[area.CommentStart-1:area.CommentEnd-1])
		strippedComment = rAll.ReplaceAll(expr, []byte(" "))
		if area.CommentStart < area.Start {
			injected = append(injected, contents[:area.CommentStart-1]...)
			injected = append(injected, strippedComment...)
			injected = append(injected, contents[area.CommentEnd-1:area.Start-1]...)
			injected = append(injected, expr...)
			injected = append(injected, contents[area.End-1:]...)
		} else {
			injected = append(injected, contents[:area.Start-1]...)
			injected = append(injected, expr...)
			injected = append(injected, contents[area.End-1:area.CommentStart-1]...)
			injected = append(injected, strippedComment...)
			injected = append(injected, contents[area.CommentEnd-1:]...)
		}
	} else {
		injected = append(injected, contents[:area.Start-1]...)
		injected = append(injected, expr...)
		injected = append(injected, contents[area.End-1:]...)
	}

	return
}
