package tags

import (
	"fmt"
	"regexp"

	"github.com/TBXark/sphere/internal/tags"
)

var (
	rInject = regexp.MustCompile("`.+`$")
	rAll    = regexp.MustCompile(".*")
)

func injectTag(contents []byte, area textArea, removeTagComment bool) []byte {
	expr := make([]byte, area.End-area.Start)
	copy(expr, contents[area.Start-1:area.End-1])
	cti := tags.NewTagItems(area.CurrentTag)
	protoName := tags.GetProtoTagName(cti)
	iti := tags.NewSphereTagItems(area.InjectTag, protoName)
	ti := cti.Override(iti)
	expr = rInject.ReplaceAll(expr, []byte(fmt.Sprintf("`%s`", ti.Format())))

	var injected []byte
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
	return injected
}
